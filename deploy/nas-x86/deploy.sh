#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
ENV_FILE="${SCRIPT_DIR}/.env"
DATA_DIR_ARG=""
APP_BIND_HOST_ARG="0.0.0.0"
APP_PORT_ARG="8080"

usage() {
  cat <<'EOF'
用法：
  ./deploy.sh --data-dir /NAS外部硬盘/ai-image-data [--bind-host 0.0.0.0] [--port 8080]

说明：
  - 首次部署必须提供 --data-dir，且必须是 NAS 外部硬盘上的绝对目录。
  - 脚本会导入同目录的 amd64 镜像、生成随机密钥、校验 Compose 并启动容器。
  - 已存在 .env 时不会覆盖密钥；升级时可直接再次执行 ./deploy.sh。
EOF
}

die() {
  printf '部署失败：%s\n' "$*" >&2
  exit 1
}

random_hex() {
  local bytes="$1"
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "${bytes}"
  else
    od -An -N "${bytes}" -tx1 /dev/urandom | tr -d ' \n'
  fi
}

env_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '%s' "${value}"
}

read_env_value() {
  local key="$1"
  awk -v key="${key}" '
    index($0, key "=") == 1 {
      value = substr($0, length(key) + 2)
      if (value ~ /^".*"$/) {
        value = substr(value, 2, length(value) - 2)
      }
      print value
      exit
    }
  ' "${ENV_FILE}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --data-dir)
      [[ $# -ge 2 ]] || die '--data-dir 缺少参数'
      DATA_DIR_ARG="$2"
      shift 2
      ;;
    --bind-host)
      [[ $# -ge 2 ]] || die '--bind-host 缺少参数'
      APP_BIND_HOST_ARG="$2"
      shift 2
      ;;
    --port)
      [[ $# -ge 2 ]] || die '--port 缺少参数'
      APP_PORT_ARG="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "未知参数 $1"
      ;;
  esac
done

command -v docker >/dev/null 2>&1 || die 'NAS 上未找到 Docker'
docker info >/dev/null 2>&1 || die 'Docker daemon 不可用'
docker compose version >/dev/null 2>&1 || die 'NAS 上未安装 Docker Compose v2'
[[ -f "${COMPOSE_FILE}" ]] || die '同目录缺少 docker-compose.yml'

image_ref="${STUDIO_IMAGE:-}"
if [[ -z "${image_ref}" && -f "${SCRIPT_DIR}/image-ref.txt" ]]; then
  image_ref="$(tr -d '\r\n' < "${SCRIPT_DIR}/image-ref.txt")"
fi
[[ -n "${image_ref}" ]] || die '无法确定镜像名称，缺少 image-ref.txt 或 STUDIO_IMAGE'
[[ "${image_ref}" =~ ^[A-Za-z0-9][A-Za-z0-9._/:@-]*$ ]] || die '镜像名称格式不合法'

image_file=""
if [[ -f "${SCRIPT_DIR}/image-file.txt" ]]; then
  image_file="$(tr -d '\r\n' < "${SCRIPT_DIR}/image-file.txt")"
fi
if [[ -n "${image_file}" && -f "${SCRIPT_DIR}/${image_file}" ]]; then
  printf '导入 Docker 镜像 %s...\n' "${image_ref}"
  docker load --input "${SCRIPT_DIR}/${image_file}" >/dev/null
elif ! docker image inspect "${image_ref}" >/dev/null 2>&1; then
  die '镜像归档不存在，且 Docker 中没有目标镜像'
fi

image_platform="$(docker image inspect "${image_ref}" --format '{{.Os}}/{{.Architecture}}')"
[[ "${image_platform}" == "linux/amd64" ]] || die "目标镜像架构是 ${image_platform}，NAS 需要 linux/amd64"

if [[ ! -f "${ENV_FILE}" ]]; then
  [[ -n "${DATA_DIR_ARG}" ]] || { usage >&2; die '首次部署必须提供 --data-dir'; }
  [[ "${DATA_DIR_ARG}" == /* ]] || die '--data-dir 必须是绝对路径'
  [[ "${DATA_DIR_ARG}" != *$'\n'* && "${DATA_DIR_ARG}" != *$'\r'* ]] || die '--data-dir 包含非法换行'
  [[ "${APP_PORT_ARG}" =~ ^[0-9]+$ && "${APP_PORT_ARG}" -ge 1 && "${APP_PORT_ARG}" -le 65535 ]] || die '--port 必须在 1 到 65535 之间'
  [[ "${APP_BIND_HOST_ARG}" =~ ^[A-Za-z0-9:.%-]+$ ]] || die '--bind-host 格式不合法'

  mkdir -p "${DATA_DIR_ARG}"
  touch "${DATA_DIR_ARG}/.studio-write-test" || die '外部数据目录不可写'
  rm -f "${DATA_DIR_ARG}/.studio-write-test"

  umask 077
  cat > "${ENV_FILE}" <<EOF
STUDIO_IMAGE="$(env_escape "${image_ref}")"
APP_BIND_HOST="$(env_escape "${APP_BIND_HOST_ARG}")"
APP_PORT=${APP_PORT_ARG}
DATA_DIR="$(env_escape "${DATA_DIR_ARG}")"
TZ=Asia/Shanghai
APP_ENV=staging
MEMORY_LIMIT=12g
MYSQL_DATABASE=amazon_ai_image_studio
MYSQL_USER=studio_app
MYSQL_ROOT_PASSWORD=$(random_hex 32)
MYSQL_PASSWORD=$(random_hex 32)
REDIS_PASSWORD=$(random_hex 32)
MINIO_ROOT_USER=studio_minio
MINIO_ROOT_PASSWORD=$(random_hex 32)
JWT_SIGNING_SECRET=$(random_hex 32)
API_KEY_ENCRYPTION_KEY=$(random_hex 32)
API_KEY_ENCRYPTION_KEY_ID=nas-v1
COOKIE_SECURE=false
CORS_ALLOWED_ORIGINS=
AUTH_DEFAULT_TENANT_ID=
WORKER_CONCURRENCY=2
TASK_GLOBAL_CONCURRENCY=2
TASK_TENANT_CONCURRENCY=2
TASK_USER_CONCURRENCY=2
TASK_PROVIDER_CONCURRENCY=2
TASK_MODEL_CONCURRENCY=2
PROVIDER_TIMEOUT_SECONDS=600
PROVIDER_MAX_RETRIES=1
PROVIDER_MAX_RESPONSE_SIZE_MB=1024
PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB=512
EOF
  chmod 0600 "${ENV_FILE}"
  printf '已生成 %s，请与数据目录一起安全备份。\n' "${ENV_FILE}"
else
  env_tmp="$(mktemp "${ENV_FILE}.tmp.XXXXXX")"
  trap 'rm -f "${env_tmp}"' EXIT
  awk -v image_ref="${image_ref}" '
    BEGIN { replacement = "STUDIO_IMAGE=\"" image_ref "\"" }
    /^STUDIO_IMAGE=/ {
      if (!updated) {
        print replacement
        updated = 1
      }
      next
    }
    { print }
    END {
      if (!updated) {
        print replacement
      }
    }
  ' "${ENV_FILE}" > "${env_tmp}"
  chmod 0600 "${env_tmp}"
  mv "${env_tmp}" "${ENV_FILE}"
  trap - EXIT
  printf '保留现有 %s 的密钥和数据目录，并切换到镜像 %s。\n' "${ENV_FILE}" "${image_ref}"
fi

effective_port="$(read_env_value APP_PORT)"
effective_bind_host="$(read_env_value APP_BIND_HOST)"
[[ "${effective_port}" =~ ^[0-9]+$ ]] || effective_port="${APP_PORT_ARG}"
[[ -n "${effective_bind_host}" ]] || effective_bind_host="${APP_BIND_HOST_ARG}"

docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" config --quiet
docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d --remove-orphans

deadline=$((SECONDS + 420))
while (( SECONDS < deadline )); do
  health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' gpt-image-studio 2>/dev/null || true)"
  case "${health}" in
    healthy)
      printf '\n部署成功，容器已健康运行。\n'
      printf '访问地址：http://NAS_IP:%s\n' "${effective_port}"
      printf '首次使用请按 README.md 的“初始化管理员”步骤创建账号。\n'
      if [[ "${effective_bind_host}" == "0.0.0.0" ]]; then
        printf '安全提示：当前 Web 端口对 NAS 所有网卡开放，请勿在路由器上直接映射到公网。\n'
      fi
      exit 0
      ;;
    unhealthy|exited|dead)
      docker logs --tail 200 gpt-image-studio >&2 || true
      die "容器状态为 ${health}"
      ;;
  esac
  sleep 5
done

docker logs --tail 200 gpt-image-studio >&2 || true
die '等待容器健康状态超时'
