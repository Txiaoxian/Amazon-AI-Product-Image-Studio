#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/deploy/docker-compose.yml}"
HEALTH_TIMEOUT_SECONDS="${DEPLOY_HEALTH_TIMEOUT_SECONDS:-240}"
RUN_UP=0
RUN_DOWN=0

usage() {
  cat <<'EOF'
Usage: bash scripts/deploy-release-validation.sh [--up] [--down] [--help]

Runs the P15 deployment release validation:
  - docker compose config validation
  - frontend nginx /api/ and SSE proxy safety checks
  - backend-api, backend-worker, and frontend image builds
  - focused security regression script
  - optional Compose up, container health checks, frontend /api/ proxy checks
  - optional Compose cleanup with --down

Options:
  --up    Start the Compose stack and run live health/proxy checks.
          The stack is left running for operator inspection.
  --down  Run docker compose down -v --remove-orphans after validation.
          When used without --up, only cleanup is performed.
  --help  Show this help text.

Environment:
  COMPOSE_FILE                    Override the Compose file path.
  DEPLOY_HEALTH_TIMEOUT_SECONDS   Health wait timeout for --up checks.

Run from any directory. The script does not print secret values from Compose
config output and does not contact external AI Providers.
EOF
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --up)
      RUN_UP=1
      ;;
    --down)
      RUN_DOWN=1
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
  shift
done

redact_file() {
  sed -E \
    -e 's/((PASSWORD|SECRET|API_KEY_ENCRYPTION_KEY|JWT_SIGNING_SECRET|AUTH_COOKIE_NAME|CSRF_COOKIE_NAME): )[[:graph:]]+/\1[redacted]/Ig' \
    -e 's/(MINIO_(ROOT_USER|ACCESS_KEY|SECRET_KEY): )[[:graph:]]+/\1[redacted]/g' \
    -e 's/(MYSQL_(USER|PASSWORD|ROOT_PASSWORD): )[[:graph:]]+/\1[redacted]/g' \
    -e 's/(REDIS_PASSWORD: )[[:graph:]]+/\1[redacted]/g' \
    "$1"
}

run() {
  echo
  echo "==> $*"
  "$@"
}

run_quiet() {
  echo
  echo "==> $*"
  local output
  output="$(mktemp)"
  set +e
  "$@" >"$output" 2>&1
  local status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    rm -f "$output"
    echo "[ok]"
    return 0
  fi
  redact_file "$output" >&2
  rm -f "$output"
  return "$status"
}

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[fail] required command not found: $1" >&2
    exit 1
  fi
}

check_frontend_proxy_config() {
  local nginx_conf="$ROOT_DIR/frontend/nginx.conf"
  echo
  echo "==> frontend nginx /api/ and SSE proxy checks"
  if [[ ! -f "$nginx_conf" ]]; then
    echo "[fail] missing frontend nginx config: $nginx_conf" >&2
    exit 1
  fi
  if ! rg -q 'location /api/' "$nginx_conf"; then
    echo "[fail] frontend nginx config has no /api/ location" >&2
    exit 1
  fi
  if ! rg -q 'proxy_pass http://backend-api:8080;' "$nginx_conf"; then
    echo "[fail] frontend nginx /api/ proxy is not mapped to backend-api:8080" >&2
    exit 1
  fi
  if ! rg -q 'location /api/v1/events/' "$nginx_conf"; then
    echo "[fail] frontend nginx config has no SSE /api/v1/events/ location" >&2
    exit 1
  fi
  if ! rg -q 'proxy_buffering off;' "$nginx_conf"; then
    echo "[fail] frontend nginx SSE proxy does not disable buffering" >&2
    exit 1
  fi
  if ! rg -q 'X-Accel-Buffering no' "$nginx_conf"; then
    echo "[fail] frontend nginx SSE proxy does not set X-Accel-Buffering no" >&2
    exit 1
  fi
  if rg -n -i '(api\.openai\.com|generativelanguage\.googleapis\.com|/v1/images/generations|/v1beta/models|proxy_pass\s+https?://[^;]*(openai|googleapis|gemini|relay))' \
    "$nginx_conf" "$COMPOSE_FILE"; then
    echo "[fail] deploy/frontend config contains forbidden AI Provider proxy target" >&2
    exit 1
  fi
  echo "[ok]"
}

wait_for_service_health() {
  local service="$1"
  local deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
  local container_id
  while (( SECONDS < deadline )); do
    if [[ "$service" == "minio-bootstrap" ]]; then
      container_id="$(compose ps -a -q "$service" 2>/dev/null || true)"
    else
      container_id="$(compose ps -q "$service" 2>/dev/null || true)"
    fi
    if [[ -n "$container_id" ]]; then
      local health
      health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id" 2>/dev/null || true)"
      case "$health" in
        healthy|running)
          echo "[ok] $service is $health"
          return 0
          ;;
        exited)
          if [[ "$service" == "minio-bootstrap" ]]; then
            echo "[ok] $service completed"
            return 0
          fi
          echo "[fail] $service exited before becoming healthy" >&2
          compose logs --no-color --tail=80 "$service" >&2 || true
          return 1
          ;;
        unhealthy)
          echo "[fail] $service is unhealthy" >&2
          compose logs --no-color --tail=80 "$service" >&2 || true
          return 1
          ;;
      esac
    fi
    sleep 3
  done
  echo "[fail] timed out waiting for $service health" >&2
  compose ps >&2 || true
  compose logs --no-color --tail=80 "$service" >&2 || true
  return 1
}

wait_for_http() {
  local description="$1"
  local url="$2"
  local expected_pattern="${3:-^2}"
  local deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    local status
    status="$(curl -ksS -o /dev/null -w '%{http_code}' --max-time 5 "$url" 2>/dev/null || true)"
    if [[ "$status" =~ $expected_pattern ]]; then
      echo "[ok] $description returned HTTP $status"
      return 0
    fi
    sleep 3
  done
  echo "[fail] $description did not return expected HTTP status: $url" >&2
  return 1
}

live_checks() {
  echo
  echo "==> docker compose up"
  compose up -d

  echo
  echo "==> container health checks"
  wait_for_service_health mysql
  wait_for_service_health redis
  wait_for_service_health minio
  wait_for_service_health minio-bootstrap
  wait_for_service_health backend-api
  wait_for_service_health backend-worker
  wait_for_service_health frontend

  local backend_port="${BACKEND_API_PORT:-8081}"
  local frontend_host="${FRONTEND_BIND_HOST:-127.0.0.1}"
  local frontend_port="${FRONTEND_PORT:-8080}"
  local backend_url="http://127.0.0.1:$backend_port"
  local frontend_url="http://$frontend_host:$frontend_port"

  echo
  echo "==> live HTTP health and proxy checks"
  wait_for_http "backend /healthz" "$backend_url/healthz"
  wait_for_http "backend /api/v1/healthz" "$backend_url/api/v1/healthz"
  wait_for_http "frontend root" "$frontend_url/"
  wait_for_http "frontend /api/ proxy health" "$frontend_url/api/v1/healthz"
  wait_for_http "frontend SSE proxy auth boundary" "$frontend_url/api/v1/events/tasks" '^(2|3|4)'
}

require_command docker
require_command rg
require_command curl

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "[fail] Compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi

if [[ "$RUN_DOWN" -eq 1 && "$RUN_UP" -ne 1 ]]; then
  run compose down -v --remove-orphans
  echo
  echo "[ok] deployment cleanup completed"
  exit 0
fi

run_quiet compose config
check_frontend_proxy_config
run compose build backend-api backend-worker frontend
run bash "$ROOT_DIR/scripts/security-regression.sh"

if [[ "$RUN_UP" -eq 1 ]]; then
  live_checks
fi

if [[ "$RUN_DOWN" -eq 1 ]]; then
  run compose down -v --remove-orphans
fi

echo
echo "[ok] deployment release validation completed"
