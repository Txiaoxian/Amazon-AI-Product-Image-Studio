#!/usr/bin/env bash

set -Eeuo pipefail

RUNTIME_SECRET_FILE="${RUNTIME_SECRET_FILE:-/data/config/runtime-secrets.env}"
MYSQL_INITIALIZE_DIR="/data/.mysql-initialize"
RUNTIME_SECRET_NAMES=(
  MYSQL_ROOT_PASSWORD
  MYSQL_PASSWORD
  REDIS_PASSWORD
  MINIO_ROOT_PASSWORD
  JWT_SIGNING_SECRET
  API_KEY_ENCRYPTION_KEY
)

log() {
  printf '[nas-entrypoint] %s\n' "$*"
}

die() {
  printf '[nas-entrypoint] 启动失败：%s\n' "$*" >&2
  exit 1
}

require_value() {
  local name="$1"
  local value="${!name:-}"
  [[ -n "${value}" ]] || die "缺少环境变量 ${name}"
  [[ "${value}" != *$'\n'* && "${value}" != *$'\r'* ]] || die "${name} 包含非法换行"
  if [[ "${value,,}" == *change-me* || "${value}" == __*__ ]]; then
    die "${name} 仍是占位值"
  fi
}

require_safe_secret() {
  local name="$1"
  require_value "${name}"
  local value="${!name}"
  [[ ${#value} -ge 16 && ${#value} -le 128 ]] || die "${name} 长度必须为 16 到 128 个字符"
  [[ "${value}" =~ ^[A-Za-z0-9._~-]+$ ]] || die "${name} 只能包含字母、数字和 . _ ~ -"
}

require_identifier() {
  local name="$1"
  require_value "${name}"
  local value="${!name}"
  [[ "${value}" =~ ^[A-Za-z_][A-Za-z0-9_]{0,31}$ ]] || die "${name} 不是安全的数据库标识符"
}

require_bucket() {
  local name="$1"
  require_value "${name}"
  local value="${!name}"
  [[ "${value}" =~ ^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$ ]] || die "${name} 不是有效的 MinIO bucket 名称"
}

apply_nas_defaults() {
  export APP_ENV="${APP_ENV:-staging}"
  export LOG_LEVEL="${LOG_LEVEL:-info}"
  export GIN_MODE="${GIN_MODE:-release}"
  export API_WRITE_TIMEOUT="${API_WRITE_TIMEOUT:-5m}"
  export MYSQL_DATABASE="${MYSQL_DATABASE:-amazon_ai_image_studio}"
  export MYSQL_USER="${MYSQL_USER:-studio_app}"
  export MINIO_ROOT_USER="${MINIO_ROOT_USER:-studio_minio}"
  export MINIO_BUCKET_ORIGINALS="${MINIO_BUCKET_ORIGINALS:-product-originals}"
  export MINIO_BUCKET_GENERATED="${MINIO_BUCKET_GENERATED:-product-generated}"
  export MINIO_BUCKET_THUMBNAILS="${MINIO_BUCKET_THUMBNAILS:-product-thumbnails}"
  export MINIO_BROWSER="${MINIO_BROWSER:-off}"
  export API_KEY_ENCRYPTION_KEY_ID="${API_KEY_ENCRYPTION_KEY_ID:-nas-v1}"
  export COOKIE_SECURE="${COOKIE_SECURE:-false}"
  export WORKER_CONCURRENCY="${WORKER_CONCURRENCY:-8}"
  export TASK_GLOBAL_CONCURRENCY="${TASK_GLOBAL_CONCURRENCY:-8}"
  export TASK_POLICY_MAX_CONCURRENCY="${TASK_POLICY_MAX_CONCURRENCY:-8}"
  export TASK_TENANT_CONCURRENCY="${TASK_TENANT_CONCURRENCY:-2}"
  export TASK_USER_CONCURRENCY="${TASK_USER_CONCURRENCY:-2}"
  export TASK_PROVIDER_CONCURRENCY="${TASK_PROVIDER_CONCURRENCY:-2}"
  export TASK_MODEL_CONCURRENCY="${TASK_MODEL_CONCURRENCY:-2}"
  export TASK_VISIBILITY_TIMEOUT="${TASK_VISIBILITY_TIMEOUT:-15m}"
  export TASK_CONCURRENCY_LEASE_TTL="${TASK_CONCURRENCY_LEASE_TTL:-30m}"
  export PROVIDER_TIMEOUT_SECONDS="${PROVIDER_TIMEOUT_SECONDS:-600}"
  export PROVIDER_MAX_RETRIES="${PROVIDER_MAX_RETRIES:-1}"
  export PROVIDER_MAX_RESPONSE_SIZE_MB="${PROVIDER_MAX_RESPONSE_SIZE_MB:-1024}"
  export PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB="${PROVIDER_MAX_OUTPUT_IMAGE_SIZE_MB:-512}"
}

is_runtime_secret_name() {
  local candidate="$1"
  local expected
  for expected in "${RUNTIME_SECRET_NAMES[@]}"; do
    [[ "${candidate}" == "${expected}" ]] && return 0
  done
  return 1
}

validate_runtime_secret_value() {
  local name="$1"
  local value="$2"
  [[ ${#value} -ge 32 && ${#value} -le 128 ]] || \
    die "${name} 必须是 32 到 128 个字符的安全密钥"
  [[ "${value}" =~ ^[A-Za-z0-9._~-]+$ ]] || \
    die "${name} 只能包含字母、数字和 . _ ~ -"
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    od -An -N 32 -tx1 /dev/urandom | tr -d ' \n'
  fi
}

create_persistent_secrets() {
  local config_dir
  local name
  local value
  local temp_file

  config_dir="$(dirname -- "${RUNTIME_SECRET_FILE}")"
  mkdir -p "${config_dir}"
  chown root:root "${config_dir}"
  chmod 0700 "${config_dir}"

  if [[ -d /data/mysql/mysql ]]; then
    for name in "${RUNTIME_SECRET_NAMES[@]}"; do
      [[ -n "${!name:-}" ]] || die \
        "检测到已有 MySQL 数据但缺少持久化密钥。请先使用原部署环境变量启动一次以完成迁移：${name}"
    done
  fi

  umask 077
  temp_file="$(mktemp "${RUNTIME_SECRET_FILE}.tmp.XXXXXX")"
  trap 'rm -f -- "${temp_file:-}"' EXIT
  : > "${temp_file}"
  for name in "${RUNTIME_SECRET_NAMES[@]}"; do
    value="${!name:-}"
    [[ -n "${value}" ]] || value="$(random_secret)"
    validate_runtime_secret_value "${name}" "${value}"
    printf '%s=%s\n' "${name}" "${value}" >> "${temp_file}"
  done
  chown root:root "${temp_file}"
  chmod 0600 "${temp_file}"
  mv "${temp_file}" "${RUNTIME_SECRET_FILE}"
  trap - EXIT
  log '首次启动密钥已生成并保存到 /data/config/runtime-secrets.env'
}

load_persistent_secrets() {
  local configured
  local line
  local name
  local value
  local expected
  local -A seen=()

  [[ -f "${RUNTIME_SECRET_FILE}" && ! -L "${RUNTIME_SECRET_FILE}" ]] || \
    die '持久化密钥文件不是普通文件或是符号链接'
  chown root:root "${RUNTIME_SECRET_FILE}"
  chmod 0600 "${RUNTIME_SECRET_FILE}"

  while IFS= read -r line || [[ -n "${line}" ]]; do
    [[ "${line}" =~ ^([A-Z0-9_]+)=(.*)$ ]] || die '持久化密钥文件格式不合法'
    name="${BASH_REMATCH[1]}"
    value="${BASH_REMATCH[2]}"
    is_runtime_secret_name "${name}" || die "持久化密钥文件包含未知字段 ${name}"
    [[ -z "${seen[${name}]:-}" ]] || die "持久化密钥文件重复定义 ${name}"
    validate_runtime_secret_value "${name}" "${value}"
    configured="${!name:-}"
    if [[ -n "${configured}" && "${configured}" != "${value}" ]]; then
      die "环境变量 ${name} 与 /data/config 中的持久化值不一致"
    fi
    printf -v "${name}" '%s' "${value}"
    export "${name}"
    seen["${name}"]=1
  done < "${RUNTIME_SECRET_FILE}"

  for expected in "${RUNTIME_SECRET_NAMES[@]}"; do
    [[ -n "${seen[${expected}]:-}" ]] || die "持久化密钥文件缺少 ${expected}"
  done
}

ensure_persistent_secrets() {
  [[ -e "${RUNTIME_SECRET_FILE}" ]] || create_persistent_secrets
  load_persistent_secrets
}

write_runtime_configs() {
  umask 077
  cat > /run/studio/mysql-root.cnf <<EOF
[client]
user=root
password=${MYSQL_ROOT_PASSWORD}
socket=/run/mysqld/mysqld.sock
EOF
  cat > /run/studio/mysql-app.cnf <<EOF
[client]
host=127.0.0.1
port=3306
user=${MYSQL_USER}
password=${MYSQL_PASSWORD}
EOF
  cat > /run/redis/studio.conf <<EOF
bind 127.0.0.1
protected-mode yes
port 6379
dir /data/redis
appendonly yes
appendfsync everysec
save 900 1
save 300 10
save 60 10000
requirepass ${REDIS_PASSWORD}
maxmemory-policy noeviction
EOF
  chown root:mysql /run/studio/mysql-root.cnf
  chown studio:studio /run/studio/mysql-app.cnf
  chown root:redis /run/redis/studio.conf
  chmod 0640 /run/studio/mysql-root.cnf /run/studio/mysql-app.cnf /run/redis/studio.conf
}

wait_for_bootstrap_mysql() {
  local initialized="$1"
  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    if [[ "${initialized}" == "1" ]]; then
      if mysqladmin --host=localhost --socket=/run/mysqld/mysqld.sock --user=root ping --silent >/dev/null 2>&1; then
        return 0
      fi
    elif mysqladmin --defaults-extra-file=/run/studio/mysql-root.cnf --host=localhost \
      --socket=/run/mysqld/mysqld.sock ping --silent >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

show_mysql_initialization_error() {
  local error_log="${1:-${MYSQL_INITIALIZE_DIR}/error.log}"
  if [[ -f "${error_log}" ]]; then
    printf '[nas-entrypoint] MySQL 错误日志（末尾 80 行）：\n' >&2
    tail -n 80 -- "${error_log}" >&2 || true
  else
    printf '[nas-entrypoint] MySQL 未生成错误日志，请检查 /data 所在文件系统和目录权限。\n' >&2
  fi
}

initialize_fresh_mysql_data_dir() {
  if find /data/mysql -mindepth 1 -print -quit 2>/dev/null | grep -q .; then
    log '检测到上次未完成的 MySQL 初始化数据，将安全重建未完成的数据目录'
  fi

  rm -rf -- "${MYSQL_INITIALIZE_DIR}"
  mkdir -p "${MYSQL_INITIALIZE_DIR}"
  chown mysql:mysql "${MYSQL_INITIALIZE_DIR}"
  chmod 0750 "${MYSQL_INITIALIZE_DIR}"

  if ! mysqld \
    --defaults-file=/etc/mysql/my.cnf \
    --datadir="${MYSQL_INITIALIZE_DIR}" \
    --log-error="${MYSQL_INITIALIZE_DIR}/error.log" \
    --initialize-insecure \
    --user=mysql; then
    show_mysql_initialization_error
    die 'MySQL 数据目录初始化失败；请根据上方错误检查 NAS 数据目录的文件系统和权限'
  fi

  rm -rf -- /data/mysql
  mv -- "${MYSQL_INITIALIZE_DIR}" /data/mysql
  chown mysql:mysql /data/mysql
  chmod 0750 /data/mysql
}

initialize_mysql() {
  local initialized=0
  if [[ ! -d /data/mysql/mysql ]]; then
    log '首次启动，正在初始化 MySQL 数据目录'
    initialize_fresh_mysql_data_dir
    initialized=1
  fi

  rm -f /run/mysqld/mysqld.sock /run/mysqld/bootstrap.pid
  mysqld \
    --defaults-file=/etc/mysql/my.cnf \
    --user=mysql \
    --skip-networking \
    --socket=/run/mysqld/mysqld.sock \
    --pid-file=/run/mysqld/bootstrap.pid &
  local mysql_pid=$!

  if ! wait_for_bootstrap_mysql "${initialized}"; then
    kill "${mysql_pid}" 2>/dev/null || true
    wait "${mysql_pid}" 2>/dev/null || true
    show_mysql_initialization_error /data/mysql/error.log
    die 'MySQL 初始化实例未在限定时间内就绪'
  fi

  if [[ "${initialized}" == "1" ]]; then
    mysql --host=localhost --socket=/run/mysqld/mysqld.sock --user=root <<SQL
CREATE DATABASE IF NOT EXISTS \`${MYSQL_DATABASE}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '${MYSQL_USER}'@'127.0.0.1' IDENTIFIED BY '${MYSQL_PASSWORD}';
ALTER USER '${MYSQL_USER}'@'127.0.0.1' IDENTIFIED BY '${MYSQL_PASSWORD}';
GRANT ALL PRIVILEGES ON \`${MYSQL_DATABASE}\`.* TO '${MYSQL_USER}'@'127.0.0.1';
ALTER USER 'root'@'localhost' IDENTIFIED BY '${MYSQL_ROOT_PASSWORD}';
SQL
  else
    mysql --defaults-extra-file=/run/studio/mysql-root.cnf --host=localhost \
      --socket=/run/mysqld/mysqld.sock <<SQL
CREATE DATABASE IF NOT EXISTS \`${MYSQL_DATABASE}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '${MYSQL_USER}'@'127.0.0.1' IDENTIFIED BY '${MYSQL_PASSWORD}';
ALTER USER '${MYSQL_USER}'@'127.0.0.1' IDENTIFIED BY '${MYSQL_PASSWORD}';
GRANT ALL PRIVILEGES ON \`${MYSQL_DATABASE}\`.* TO '${MYSQL_USER}'@'127.0.0.1';
SQL
  fi

  mysqladmin --defaults-extra-file=/run/studio/mysql-root.cnf --host=localhost \
    --socket=/run/mysqld/mysqld.sock shutdown --silent
  wait "${mysql_pid}"
  rm -f /run/mysqld/mysqld.sock /run/mysqld/bootstrap.pid
  log 'MySQL 数据目录和应用账号已就绪'
}

main() {
  [[ "$(id -u)" == "0" ]] || die '入口脚本必须以 root 启动，以便初始化挂载目录权限'

  STARTUP_TIMEOUT_SECONDS="${STARTUP_TIMEOUT_SECONDS:-180}"
  [[ "${STARTUP_TIMEOUT_SECONDS}" =~ ^[0-9]+$ && "${STARTUP_TIMEOUT_SECONDS}" -ge 30 ]] || \
    die 'STARTUP_TIMEOUT_SECONDS 必须是至少 30 的整数'

  apply_nas_defaults

  mkdir -p /data/config
  touch /data/.write-test || die '/data 不可写，请检查 NAS 外部硬盘目录及挂载权限'
  rm -f /data/.write-test
  chown root:root /data/config
  chmod 0700 /data/config

  ensure_persistent_secrets

  require_identifier MYSQL_DATABASE
  require_identifier MYSQL_USER
  require_safe_secret MYSQL_ROOT_PASSWORD
  require_safe_secret MYSQL_PASSWORD
  require_safe_secret REDIS_PASSWORD
  require_value MINIO_ROOT_USER
  [[ "${MINIO_ROOT_USER}" =~ ^[A-Za-z0-9._~-]{3,64}$ ]] || die 'MINIO_ROOT_USER 格式不合法'
  require_safe_secret MINIO_ROOT_PASSWORD
  require_value JWT_SIGNING_SECRET
  [[ ${#JWT_SIGNING_SECRET} -ge 32 ]] || die 'JWT_SIGNING_SECRET 至少需要 32 个字符'
  require_value API_KEY_ENCRYPTION_KEY
  [[ ${#API_KEY_ENCRYPTION_KEY} -ge 32 ]] || die 'API_KEY_ENCRYPTION_KEY 至少需要 32 个字符'
  require_bucket MINIO_BUCKET_ORIGINALS
  require_bucket MINIO_BUCKET_GENERATED
  require_bucket MINIO_BUCKET_THUMBNAILS

  export MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-${MINIO_ROOT_USER}}"
  export MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-${MINIO_ROOT_PASSWORD}}"

  mkdir -p /data/mysql /data/mysql-tmp /data/redis /data/minio /data/tmp /run/mysqld /run/redis /run/studio
  rm -f /run/studio/minio-ready /run/studio/worker-ready

  chown mysql:mysql /data/mysql /data/mysql-tmp /run/mysqld
  chown redis:redis /data/redis /run/redis
  chown studio:studio /data/minio /data/tmp /run/studio
  chmod 0750 /data/mysql /data/mysql-tmp /data/redis /data/minio /data/tmp /run/mysqld /run/redis /run/studio
  find /data/mysql-tmp -mindepth 1 -delete 2>/dev/null || true
  find /data/tmp -mindepth 1 -mtime +1 -delete 2>/dev/null || true

  write_runtime_configs
  initialize_mysql

  log '启动 Nginx、API、Worker、MySQL、Redis 和 MinIO'
  exec /usr/bin/supervisord -c /etc/supervisor/supervisord.conf
}

main "$@"
