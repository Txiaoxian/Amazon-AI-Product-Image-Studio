#!/usr/bin/env bash

set -Eeuo pipefail

deadline=$((SECONDS + ${STARTUP_TIMEOUT_SECONDS:-180}))
while true; do
  mysql_ready=0
  redis_ready=0
  minio_ready=0

  mysqladmin --defaults-extra-file=/run/studio/mysql-app.cnf ping --silent >/dev/null 2>&1 && mysql_ready=1
  REDISCLI_AUTH="${REDIS_PASSWORD}" redis-cli -h 127.0.0.1 ping 2>/dev/null | grep -q '^PONG$' && redis_ready=1
  [[ -f /run/studio/minio-ready ]] && curl -fsS http://127.0.0.1:9000/minio/health/ready >/dev/null 2>&1 && minio_ready=1

  if [[ "${mysql_ready}${redis_ready}${minio_ready}" == "111" ]]; then
    break
  fi
  (( SECONDS < deadline )) || { printf 'API 等待内部依赖超时。\n' >&2; exit 1; }
  sleep 1
done

exec /usr/local/bin/backend-api
