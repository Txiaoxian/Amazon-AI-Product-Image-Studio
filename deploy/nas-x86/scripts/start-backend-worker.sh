#!/usr/bin/env bash

set -Eeuo pipefail

rm -f "${WORKER_HEALTHCHECK_FILE:-/run/studio/worker-ready}"
deadline=$((SECONDS + ${STARTUP_TIMEOUT_SECONDS:-180}))
until curl -fsS http://127.0.0.1:8081/healthz >/dev/null 2>&1; do
  (( SECONDS < deadline )) || { printf 'Worker 等待 API 就绪超时。\n' >&2; exit 1; }
  sleep 1
done

exec /usr/local/bin/backend-worker
