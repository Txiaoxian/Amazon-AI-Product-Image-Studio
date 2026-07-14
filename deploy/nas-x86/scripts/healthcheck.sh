#!/usr/bin/env bash

set -Eeuo pipefail

RUNTIME_SECRET_FILE="/data/config/runtime-secrets.env"

load_redis_password() {
  local line
  local value

  [[ -f "${RUNTIME_SECRET_FILE}" && ! -L "${RUNTIME_SECRET_FILE}" ]] || return 1
  [[ "$(grep -Ec '^REDIS_PASSWORD=' "${RUNTIME_SECRET_FILE}")" == "1" ]] || return 1
  line="$(grep -E '^REDIS_PASSWORD=' "${RUNTIME_SECRET_FILE}")"
  value="${line#REDIS_PASSWORD=}"
  [[ ${#value} -ge 32 && ${#value} -le 128 ]] || return 1
  [[ "${value}" =~ ^[A-Za-z0-9._~-]+$ ]] || return 1
  export REDIS_PASSWORD="${value}"
}

[[ -n "${REDIS_PASSWORD:-}" ]] || load_redis_password

curl -fsS http://127.0.0.1:8080/ >/dev/null
curl -fsS http://127.0.0.1:8081/healthz >/dev/null
mysqladmin --defaults-extra-file=/run/studio/mysql-app.cnf ping --silent >/dev/null
REDISCLI_AUTH="${REDIS_PASSWORD}" redis-cli -h 127.0.0.1 ping 2>/dev/null | grep -q '^PONG$'
curl -fsS http://127.0.0.1:9000/minio/health/ready >/dev/null
test -f "${WORKER_HEALTHCHECK_FILE:-/run/studio/worker-ready}"
test -f /run/studio/minio-ready

while read -r name status _; do
  [[ "${status}" == "RUNNING" ]] || {
    printf '内部进程 %s 状态为 %s\n' "${name}" "${status}" >&2
    exit 1
  }
done < <(/usr/bin/supervisorctl -c /etc/supervisor/supervisord.conf status)
