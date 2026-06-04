#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/deploy-release-validation.sh"
REAL_BASH="${BASH:-/usr/bin/env bash}"

assert_contains() {
  local file="$1"
  local pattern="$2"
  if ! grep -Fq -- "$pattern" "$file"; then
    echo "[fail] expected log to contain: $pattern" >&2
    echo "[debug] log contents:" >&2
    sed 's/^/[debug] /' "$file" >&2
    exit 1
  fi
}

assert_not_contains() {
  local file="$1"
  local pattern="$2"
  if grep -Fq -- "$pattern" "$file"; then
    echo "[fail] expected log not to contain: $pattern" >&2
    echo "[debug] log contents:" >&2
    sed 's/^/[debug] /' "$file" >&2
    exit 1
  fi
}

assert_status() {
  local expected="$1"
  local actual="$2"
  local name="$3"
  if [[ "$actual" -ne "$expected" ]]; then
    echo "[fail] $name: expected status $expected, got $actual" >&2
    exit 1
  fi
}

assert_nonzero_status() {
  local actual="$1"
  local name="$2"
  if [[ "$actual" -eq 0 ]]; then
    echo "[fail] $name: expected nonzero status" >&2
    exit 1
  fi
}

setup_fake_bin() {
  local tmpdir="$1"
  local bindir="$tmpdir/bin"
  mkdir -p "$bindir"

  cat >"$bindir/docker" <<EOF
#!$REAL_BASH
EOF
  cat >>"$bindir/docker" <<'EOF'
set -euo pipefail

log() {
  printf 'docker %s\n' "$*" >>"$FAKE_LOG"
}

if [[ "${1:-}" == "compose" ]]; then
  shift
  compose_log_args=(compose)
  if [[ "${1:-}" == "-f" ]]; then
    shift 2
  fi
  if [[ "${1:-}" == "--env-file" ]]; then
    compose_log_args+=(--env-file "$2")
    shift 2
  fi
  case "${1:-}" in
    config|build|logs)
      log "${compose_log_args[*]} $*"
      if [[ "${1:-}" == "logs" && "${FAKE_LOGS_PRINT_SECRETS:-0}" == "1" ]]; then
        echo "API_KEY: fixture-api-key-secret"
        echo "Authorization: Bearer fixture-authorization-secret"
        echo "Cookie: session=fixture-cookie-secret"
        echo "password=fixture-password-secret"
        echo '{"api_key":"fixture-json-api-key-secret","Authorization":"Bearer fixture-json-authorization-secret","Cookie":"session=fixture-json-cookie-secret","password":"fixture-json-password-secret"}'
      fi
      exit 0
      ;;
    ps)
      log "${compose_log_args[*]} $*"
      last_arg="${*: -1}"
      if [[ " $* " == *" -q "* && "$last_arg" != "${FAKE_MISSING_SERVICE:-}" && "$last_arg" != "ps" && "$last_arg" != "-q" && "$last_arg" != "-a" ]]; then
        printf '%s_id\n' "$last_arg"
      fi
      exit 0
      ;;
    up)
      log "${compose_log_args[*]} $*"
      if [[ "${FAKE_COMPOSE_UP_FAIL:-0}" == "1" ]]; then
        echo "compose up failed" >&2
        exit 17
      fi
      exit 0
      ;;
    down)
      log "${compose_log_args[*]} $*"
      if [[ "${FAKE_COMPOSE_DOWN_FAIL:-0}" == "1" ]]; then
        echo "MYSQL_PASSWORD: fixture-redaction-value" >&2
        echo "compose down failed" >&2
        exit 19
      fi
      exit 0
      ;;
    *)
      log "${compose_log_args[*]} $*"
      exit 0
      ;;
  esac
fi

if [[ "${1:-}" == "inspect" ]]; then
  shift
  while [[ "$#" -gt 0 && "${1:-}" == --* ]]; do
    shift
    if [[ "${1:-}" == "{{"* ]]; then
      shift
    fi
  done
  local_id="${1:-}"
  log "inspect $local_id"
  if [[ -n "${FAKE_SIGNAL_ON_INSPECT:-}" ]]; then
    kill "-$FAKE_SIGNAL_ON_INSPECT" "$PPID"
    exit 143
  fi
  if [[ -n "${FAKE_UNHEALTHY_SERVICE:-}" && "$local_id" == "${FAKE_UNHEALTHY_SERVICE}_id" ]]; then
    echo "unhealthy"
    exit 0
  fi
  if [[ -n "${FAKE_EXITED_SERVICE:-}" && "$local_id" == "${FAKE_EXITED_SERVICE}_id" ]]; then
    echo "exited"
    exit 0
  fi
  if [[ "$local_id" == "minio-bootstrap_id" ]]; then
    echo "exited"
  else
    echo "healthy"
  fi
  exit 0
fi

log "$*"
exit 0
EOF
  chmod +x "$bindir/docker"

  cat >"$bindir/rg" <<EOF
#!$REAL_BASH
EOF
  cat >>"$bindir/rg" <<'EOF'
set -euo pipefail
printf 'rg %s\n' "$*" >>"$FAKE_LOG"

quiet=0
if [[ "${1:-}" == "-q" ]]; then
  quiet=1
  shift
fi

pattern="${1:-}"
if [[ "$quiet" -eq 1 ]]; then
  case "$pattern" in
    *"location /api/"*|*"proxy_pass http://backend-api:8080;"*|*"location /api/v1/events/"*|*"proxy_buffering off;"*|*"X-Accel-Buffering no"*)
      exit 0
      ;;
    *)
      exit 1
      ;;
  esac
fi

case "$pattern" in
  *"api\\.openai\\.com"*|*"generativelanguage\\.googleapis\\.com"*|*"/v1/images/generations"*|*"/v1beta/models"*)
    exit 1
    ;;
esac

exit 1
EOF
  chmod +x "$bindir/rg"

  cat >"$bindir/curl" <<EOF
#!$REAL_BASH
EOF
  cat >>"$bindir/curl" <<'EOF'
set -euo pipefail
printf 'curl %s\n' "$*" >>"$FAKE_LOG"
if [[ "${FAKE_CURL_FAIL:-0}" == "1" ]]; then
  printf '500'
else
  printf '200'
fi
EOF
  chmod +x "$bindir/curl"

  cat >"$bindir/bash" <<EOF
#!$REAL_BASH
set -euo pipefail
if [[ "\${1:-}" == "$ROOT_DIR/scripts/security-regression.sh" ]]; then
  printf 'security-regression COMPOSE_ENV_FILES=%s\\n' "\${COMPOSE_ENV_FILES:-}" >>"\$FAKE_LOG"
  exit 0
fi
if [[ "\${1:-}" == "$ROOT_DIR/scripts/tls-reverse-proxy-check.sh" ]]; then
  printf 'tls-reverse-proxy-check\\n' >>"\$FAKE_LOG"
  exit 0
fi
exec "$REAL_BASH" "\$@"
EOF
  chmod +x "$bindir/bash"

  cat >"$bindir/dirname" <<EOF
#!$REAL_BASH
exec /usr/bin/dirname "\$@"
EOF
  chmod +x "$bindir/dirname"
}

run_case() {
  local name="$1"
  shift
  local -a env_args=()
  local -a script_args=()
  while [[ "$#" -gt 0 ]]; do
    case "$1" in
      *=*)
        env_args+=("$1")
        ;;
      *)
        script_args+=("$1")
        ;;
    esac
    shift
  done
  local tmpdir
  tmpdir="$(mktemp -d)"
  setup_fake_bin "$tmpdir"
  local log="$tmpdir/calls.log"
  local output="$tmpdir/output.log"
  local script_tmpdir="$tmpdir/tmp"
  mkdir -p "$script_tmpdir"
  touch "$log"

  set +e +u
  if [[ "${#env_args[@]}" -gt 0 ]]; then
    env \
      PATH="$tmpdir/bin:$PATH" \
      FAKE_LOG="$log" \
      TMPDIR="$script_tmpdir" \
      DEPLOY_HEALTH_TIMEOUT_SECONDS=1 \
      "${env_args[@]}" \
      "$REAL_BASH" "$SCRIPT" "${script_args[@]}" >"$output" 2>&1
  else
    env \
      PATH="$tmpdir/bin:$PATH" \
      FAKE_LOG="$log" \
      TMPDIR="$script_tmpdir" \
      DEPLOY_HEALTH_TIMEOUT_SECONDS=1 \
      "$REAL_BASH" "$SCRIPT" "${script_args[@]}" >"$output" 2>&1
  fi
  local status=$?
  set -e -u

  CASE_LOG="$log"
  CASE_OUTPUT="$output"
  CASE_STATUS="$status"
  CASE_TMPDIR="$script_tmpdir"
  echo "[ok] ran $name"
}

assert_tmpdir_empty() {
  local tmpdir="$1"
  if find "$tmpdir" -type f | grep -q .; then
    echo "[fail] expected temporary files to be removed from: $tmpdir" >&2
    find "$tmpdir" -type f -print >&2
    exit 1
  fi
}

test_default_mode_does_not_start_or_cleanup() {
  run_case "default mode does not start or cleanup"
  assert_status 0 "$CASE_STATUS" "default mode"
  assert_contains "$CASE_LOG" "docker compose config"
  assert_contains "$CASE_LOG" "tls-reverse-proxy-check"
  assert_contains "$CASE_LOG" "docker compose build backend-api backend-worker frontend"
  assert_contains "$CASE_LOG" "security-regression"
  assert_not_contains "$CASE_LOG" "docker compose up -d"
  assert_not_contains "$CASE_LOG" "docker compose down -v --remove-orphans"
}

test_down_only_runs_cleanup_only() {
  run_case "down only runs cleanup only" --down
  assert_status 0 "$CASE_STATUS" "--down"
  assert_contains "$CASE_LOG" "docker compose down -v --remove-orphans"
  assert_not_contains "$CASE_LOG" "docker compose config"
  assert_not_contains "$CASE_LOG" "tls-reverse-proxy-check"
  assert_not_contains "$CASE_LOG" "docker compose build backend-api backend-worker frontend"
  assert_not_contains "$CASE_LOG" "security-regression"
  assert_not_contains "$CASE_LOG" "docker compose up -d"
}

test_up_failure_without_down_keeps_stack() {
  run_case "up failure without down keeps stack" FAKE_UNHEALTHY_SERVICE=mysql --up
  assert_status 1 "$CASE_STATUS" "--up failed health"
  assert_contains "$CASE_LOG" "docker compose up -d"
  assert_contains "$CASE_LOG" "docker inspect mysql_id"
  assert_not_contains "$CASE_LOG" "docker compose down -v --remove-orphans"
}

test_up_down_failure_cleans_stack() {
  run_case "up down failure cleans stack" FAKE_UNHEALTHY_SERVICE=mysql --up --down
  assert_status 1 "$CASE_STATUS" "--up --down failed health"
  assert_contains "$CASE_LOG" "docker compose up -d"
  assert_contains "$CASE_LOG" "docker inspect mysql_id"
  assert_contains "$CASE_LOG" "docker compose down -v --remove-orphans"
}

test_up_down_success_cleans_stack() {
  run_case "up down success cleans stack" --up --down
  assert_status 0 "$CASE_STATUS" "--up --down success"
  assert_contains "$CASE_LOG" "docker compose up -d"
  assert_contains "$CASE_LOG" "curl -ksS -o /dev/null -w %{http_code} --max-time 5 http://127.0.0.1:8080/api/v1/events/tasks"
  assert_contains "$CASE_LOG" "docker compose down -v --remove-orphans"
}

test_cleanup_failure_is_redacted_and_nonzero() {
  run_case "cleanup failure is redacted and nonzero" FAKE_COMPOSE_DOWN_FAIL=1 --up --down
  assert_nonzero_status "$CASE_STATUS" "cleanup failure"
  assert_contains "$CASE_LOG" "docker compose down -v --remove-orphans"
  assert_contains "$CASE_OUTPUT" "MYSQL_PASSWORD: [redacted]"
  assert_not_contains "$CASE_OUTPUT" "fixture-redaction-value"
}

test_explicit_env_file_is_used_by_all_compose_commands() {
  local env_file
  env_file="$(mktemp)"
  run_case "explicit env file is used by Compose commands" --env-file "$env_file" --up --down
  assert_status 0 "$CASE_STATUS" "explicit --env-file"
  assert_contains "$CASE_LOG" "docker compose --env-file $env_file config"
  assert_contains "$CASE_LOG" "docker compose --env-file $env_file build backend-api backend-worker frontend"
  assert_contains "$CASE_LOG" "docker compose --env-file $env_file up -d"
  assert_contains "$CASE_LOG" "docker compose --env-file $env_file ps -q mysql"
  assert_contains "$CASE_LOG" "docker compose --env-file $env_file down -v --remove-orphans"
  assert_contains "$CASE_LOG" "security-regression COMPOSE_ENV_FILES=$env_file"
  rm -f "$env_file"
}

test_missing_explicit_env_file_fails_before_compose() {
  local env_file="/tmp/deploy-release-validation-missing-env-file-$$"
  rm -f "$env_file"
  run_case "missing explicit env file fails closed" --env-file "$env_file"
  assert_nonzero_status "$CASE_STATUS" "missing explicit --env-file"
  assert_contains "$CASE_OUTPUT" "[fail] Compose env file not found"
  if [[ -s "$CASE_LOG" ]]; then
    echo "[fail] missing explicit env file must fail before Compose commands" >&2
    exit 1
  fi
}

assert_health_failure_logs_are_redacted() {
  local name="$1"
  assert_status 1 "$CASE_STATUS" "$name"
  assert_contains "$CASE_LOG" "logs --no-color --tail=80"
  assert_contains "$CASE_OUTPUT" "API_KEY: [redacted]"
  assert_contains "$CASE_OUTPUT" "Authorization: [redacted]"
  assert_contains "$CASE_OUTPUT" "Cookie: [redacted]"
  assert_contains "$CASE_OUTPUT" "password=[redacted]"
  assert_not_contains "$CASE_OUTPUT" "fixture-api-key-secret"
  assert_not_contains "$CASE_OUTPUT" "fixture-authorization-secret"
  assert_not_contains "$CASE_OUTPUT" "fixture-cookie-secret"
  assert_not_contains "$CASE_OUTPUT" "fixture-password-secret"
  assert_not_contains "$CASE_OUTPUT" "fixture-json-api-key-secret"
  assert_not_contains "$CASE_OUTPUT" "fixture-json-authorization-secret"
  assert_not_contains "$CASE_OUTPUT" "fixture-json-cookie-secret"
  assert_not_contains "$CASE_OUTPUT" "fixture-json-password-secret"
  assert_tmpdir_empty "$CASE_TMPDIR"
}

test_exited_service_logs_are_redacted() {
  local env_file
  env_file="$(mktemp)"
  run_case "exited service logs are redacted" \
    FAKE_EXITED_SERVICE=backend-api \
    FAKE_LOGS_PRINT_SECRETS=1 \
    --env-file "$env_file" \
    --up
  assert_health_failure_logs_are_redacted "exited service"
  assert_contains "$CASE_LOG" "docker compose --env-file $env_file logs --no-color --tail=80 backend-api"
  rm -f "$env_file"
}

test_unhealthy_service_logs_are_redacted() {
  run_case "unhealthy service logs are redacted" \
    FAKE_UNHEALTHY_SERVICE=mysql \
    FAKE_LOGS_PRINT_SECRETS=1 \
    --up
  assert_health_failure_logs_are_redacted "unhealthy service"
}

test_timeout_service_logs_are_redacted() {
  run_case "timeout service logs are redacted" \
    FAKE_MISSING_SERVICE=mysql \
    FAKE_LOGS_PRINT_SECRETS=1 \
    --up
  assert_health_failure_logs_are_redacted "timeout service"
}

test_compose_logging_rotation_is_configured_for_long_running_services() {
  local compose_file="$ROOT_DIR/deploy/docker-compose.yml"
  local env_example="$ROOT_DIR/deploy/.env.example"
  local logging_count
  assert_contains "$compose_file" "x-json-file-logging: &json-file-logging"
  assert_contains "$compose_file" 'max-size: "${COMPOSE_LOG_MAX_SIZE:-10m}"'
  assert_contains "$compose_file" 'max-file: "${COMPOSE_LOG_MAX_FILE:-3}"'
  logging_count="$(grep -Fc 'logging: *json-file-logging' "$compose_file")"
  assert_status 6 "$logging_count" "long-running service logging config count"
  if sed -n '/^  minio-bootstrap:/,/^volumes:/p' "$compose_file" | grep -Fq 'logging:'; then
    echo "[fail] minio-bootstrap must remain a one-shot service without logging rotation" >&2
    exit 1
  fi
  assert_contains "$env_example" "COMPOSE_LOG_MAX_SIZE=10m"
  assert_contains "$env_example" "COMPOSE_LOG_MAX_FILE=3"
}

test_missing_required_command_fails_before_cleanup() {
  local command_name="$1"
  local tmpdir
  tmpdir="$(mktemp -d)"
  setup_fake_bin "$tmpdir"
  rm -f "$tmpdir/bin/$command_name"
  local log="$tmpdir/calls.log"
  local output="$tmpdir/output.log"
  touch "$log"

  set +e
  env PATH="$tmpdir/bin" FAKE_LOG="$log" "$REAL_BASH" "$SCRIPT" --up --down >"$output" 2>&1
  local status=$?
  set -e

  if [[ "$status" -eq 0 ]]; then
    echo "[fail] missing $command_name should fail" >&2
    exit 1
  fi
  assert_contains "$output" "[fail] required command not found: $command_name"
  assert_not_contains "$log" "docker compose down -v --remove-orphans"
}

test_sigterm_during_live_validation_cleans_stack() {
  run_case "sigterm during live validation cleans stack" FAKE_SIGNAL_ON_INSPECT=TERM --up --down
  assert_nonzero_status "$CASE_STATUS" "SIGTERM during live validation"
  assert_contains "$CASE_LOG" "docker compose up -d"
  assert_contains "$CASE_LOG" "docker inspect mysql_id"
  assert_contains "$CASE_LOG" "docker compose down -v --remove-orphans"
}

test_sigint_during_live_validation_cleans_stack() {
  run_case "sigint during live validation cleans stack" FAKE_SIGNAL_ON_INSPECT=INT --up --down
  assert_nonzero_status "$CASE_STATUS" "SIGINT during live validation"
  assert_contains "$CASE_LOG" "docker compose up -d"
  assert_contains "$CASE_LOG" "docker inspect mysql_id"
  assert_contains "$CASE_LOG" "docker compose down -v --remove-orphans"
}

test_default_mode_does_not_start_or_cleanup
test_down_only_runs_cleanup_only
test_up_failure_without_down_keeps_stack
test_up_down_failure_cleans_stack
test_up_down_success_cleans_stack
test_cleanup_failure_is_redacted_and_nonzero
test_explicit_env_file_is_used_by_all_compose_commands
test_missing_explicit_env_file_fails_before_compose
test_exited_service_logs_are_redacted
test_unhealthy_service_logs_are_redacted
test_timeout_service_logs_are_redacted
test_compose_logging_rotation_is_configured_for_long_running_services
test_missing_required_command_fails_before_cleanup docker
test_missing_required_command_fails_before_cleanup rg
test_missing_required_command_fails_before_cleanup curl
test_sigterm_during_live_validation_cleans_stack
test_sigint_during_live_validation_cleans_stack

echo
echo "[ok] deploy release validation script tests passed"
