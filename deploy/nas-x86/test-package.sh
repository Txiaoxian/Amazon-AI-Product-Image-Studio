#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"

fail() {
  printf 'NAS 部署契约检查失败：%s\n' "$*" >&2
  exit 1
}

assert_file() {
  local path="$1"
  [[ -f "${path}" ]] || fail "缺少文件 ${path#"${REPO_ROOT}/"}"
}

assert_contains() {
  local path="$1"
  local pattern="$2"
  local message="$3"
  grep -Eq -- "${pattern}" "${path}" || fail "${message}"
}

assert_not_contains() {
  local path="$1"
  local pattern="$2"
  local message="$3"
  if grep -Eq -- "${pattern}" "${path}"; then
    fail "${message}"
  fi
}

for required in \
  "${SCRIPT_DIR}/Dockerfile" \
  "${SCRIPT_DIR}/docker-compose.yml" \
  "${SCRIPT_DIR}/nas.env.example" \
  "${SCRIPT_DIR}/build-package.sh" \
  "${SCRIPT_DIR}/deploy.sh" \
  "${SCRIPT_DIR}/scripts/entrypoint.sh" \
  "${SCRIPT_DIR}/scripts/healthcheck.sh" \
  "${SCRIPT_DIR}/config/nginx.conf" \
  "${SCRIPT_DIR}/config/mysql.cnf" \
  "${SCRIPT_DIR}/config/supervisord.conf"; do
  assert_file "${required}"
done

assert_contains "${SCRIPT_DIR}/build-package.sh" '--platform[ =]linux/amd64' \
  '构建脚本必须固定输出 linux/amd64 镜像'
assert_contains "${SCRIPT_DIR}/build-package.sh" 'type=docker,dest=' \
  '构建脚本必须生成可离线导入的 Docker 镜像归档'
assert_contains "${SCRIPT_DIR}/build-package.sh" 'DIRECT_IMAGE_ARCHIVE=' \
  '构建脚本必须在输出根目录生成可直接导入极空间的镜像包'
assert_contains "${SCRIPT_DIR}/build-package.sh" 'shasum -a 256 "\$\{IMAGE_ARCHIVE_NAME\}"' \
  'Docker 镜像校验文件必须使用可跨机器验证的相对文件名'
assert_contains "${SCRIPT_DIR}/docker-compose.yml" '^[[:space:]]+studio:' \
  'Compose 必须只声明 studio 应用服务'
assert_contains "${SCRIPT_DIR}/docker-compose.yml" '(target:[[:space:]]*/data|:/data([[:space:]]|$))' \
  'Compose 必须把外部数据目录映射到容器 /data'
assert_contains "${SCRIPT_DIR}/docker-compose.yml" 'APP_BIND_HOST:-127\.0\.0\.1.*APP_PORT:-8080.*:8080' \
  '默认只能把 Web 端口绑定到 NAS 回环地址'
assert_contains "${SCRIPT_DIR}/Dockerfile" 'backend-api' \
  '单镜像必须包含 Go API'
assert_contains "${SCRIPT_DIR}/Dockerfile" 'backend-worker' \
  '单镜像必须包含 Go Worker'
assert_contains "${SCRIPT_DIR}/Dockerfile" 'supervisor' \
  '单容器必须由进程监管器管理内部服务'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" 'MYSQL_PASSWORD' \
  '入口脚本必须校验数据库密钥'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" 'API_KEY_ENCRYPTION_KEY' \
  '入口脚本必须校验 Provider 密钥加密主密钥'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" 'RUNTIME_SECRET_FILE="\$\{RUNTIME_SECRET_FILE:-/data/config/runtime-secrets\.env\}"' \
  '镜像必须把自动生成的内部密钥保存到 /data/config'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" '^ensure_persistent_secrets\(\)' \
  '镜像必须支持在没有环境变量时自动生成并加载内部密钥'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" 'MYSQL_INITIALIZE_DIR="/data/\.mysql-initialize"' \
  'MySQL 必须在独立临时目录初始化，避免异常中断污染正式数据目录'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" '^show_mysql_initialization_error\(\)' \
  'MySQL 初始化失败时必须把真实错误输出到容器日志'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" '--datadir="\$\{MYSQL_INITIALIZE_DIR\}"' \
  'MySQL 初始化命令必须使用独立临时目录'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" '--log-error="\$\{MYSQL_INITIALIZE_DIR\}/error\.log"' \
  'MySQL 初始化错误必须先写入可回显的临时日志'
assert_contains "${SCRIPT_DIR}/scripts/healthcheck.sh" 'RUNTIME_SECRET_FILE="/data/config/runtime-secrets\.env"' \
  'Docker 健康检查必须读取镜像运行时自动生成的 Redis 密钥'
assert_contains "${REPO_ROOT}/docs/nas-x86-deployment.md" '极空间 Docker 界面' \
  '部署文档必须说明如何在极空间 Docker 界面直接导入镜像'
assert_not_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" 'mysql(admin)? --protocol=socket' \
  'MySQL 8 客户端在 amd64 模拟环境下不能强制使用会触发 ERROR 2047 的 protocol=socket 参数'
assert_contains "${SCRIPT_DIR}/config/supervisord.conf" 'redis-server /run/redis/studio\.conf' \
  'Redis 配置必须放在 redis 用户可遍历的独立运行目录'
assert_contains "${SCRIPT_DIR}/scripts/start-minio.sh" '^export MC_CONFIG_DIR=/run/studio/mc$' \
  'MinIO 客户端配置必须写入 studio 用户可写的运行目录'
assert_contains "${SCRIPT_DIR}/deploy.sh" 'replacement = "STUDIO_IMAGE=' \
  '升级部署必须自动把已有 .env 切换到新包的镜像版本'
assert_contains "${SCRIPT_DIR}/deploy.sh" 'effective_port="\$\(read_env_value APP_PORT\)"' \
  '重复部署后的访问提示必须读取已有 .env 的真实端口'
assert_contains "${SCRIPT_DIR}/config/mysql.cnf" '^log-error=/data/mysql/error\.log$' \
  'MySQL 错误日志必须写入可持久化且 mysql 用户可写的数据目录'
assert_contains "${SCRIPT_DIR}/config/mysql.cnf" '^tmpdir=/data/mysql-tmp$' \
  'MySQL 必须使用 mysql 用户专属临时目录，不能继承应用的 /data/tmp'
assert_contains "${SCRIPT_DIR}/scripts/entrypoint.sh" '/data/mysql-tmp' \
  '入口脚本必须创建并维护 MySQL 专属临时目录'
assert_not_contains "${SCRIPT_DIR}/config/mysql.cnf" '^log-error=/dev/stderr$' \
  'MySQL 初始化阶段不能把 /dev/stderr 当作错误日志文件'
assert_not_contains "${SCRIPT_DIR}/nas.env.example" 'sk-[A-Za-z0-9]{12,}' \
  '环境模板不得包含 Provider API Key'
assert_not_contains "${SCRIPT_DIR}/nas.env.example" '=change-me' \
  '环境模板不得使用可被误部署的 change-me 默认密钥'

service_count="$(awk '
  /^services:/ { in_services=1; next }
  in_services && /^[^[:space:]]/ { in_services=0 }
  in_services && /^  [A-Za-z0-9_.-]+:([[:space:]]*#.*)?$/ { count++ }
  END { print count+0 }
' "${SCRIPT_DIR}/docker-compose.yml")"
[[ "${service_count}" == "1" ]] || fail "Compose 声明了 ${service_count} 个服务，应当只有 1 个"

printf 'NAS 部署契约检查通过。\n'
