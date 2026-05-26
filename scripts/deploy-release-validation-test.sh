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
  if [[ "${1:-}" == "-f" ]]; then
    shift 2
  fi
  case "${1:-}" in
    config|build|logs)
      log "compose $*"
      exit 0
      ;;
    ps)
      log "compose $*"
      last_arg="${*: -1}"
      if [[ " $* " == *" -q "* && "$last_arg" != "ps" && "$last_arg" != "-q" && "$last_arg" != "-a" ]]; then
        printf '%s_id\n' "$last_arg"
      fi
      exit 0
      ;;
    up)
      log "compose $*"
      if [[ "${FAKE_COMPOSE_UP_FAIL:-0}" == "1" ]]; then
        echo "compose up failed" >&2
        exit 17
      fi
      exit 0
      ;;
    down)
      log "compose $*"
      if [[ "${FAKE_COMPOSE_DOWN_FAIL:-0}" == "1" ]]; then
        echo "MYSQL_PASSWORD: fixture-redaction-value" >&2
        echo "compose down failed" >&2
        exit 19
      fi
      exit 0
      ;;
    *)
      log "compose $*"
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
  printf 'security-regression\\n' >>"\$FAKE_LOG"
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
  touch "$log"

  set +e +u
  if [[ "${#env_args[@]}" -gt 0 ]]; then
    env \
      PATH="$tmpdir/bin:$PATH" \
      FAKE_LOG="$log" \
      DEPLOY_HEALTH_TIMEOUT_SECONDS=1 \
      "${env_args[@]}" \
      "$REAL_BASH" "$SCRIPT" "${script_args[@]}" >"$output" 2>&1
  else
    env \
      PATH="$tmpdir/bin:$PATH" \
      FAKE_LOG="$log" \
      DEPLOY_HEALTH_TIMEOUT_SECONDS=1 \
      "$REAL_BASH" "$SCRIPT" "${script_args[@]}" >"$output" 2>&1
  fi
  local status=$?
  set -e -u

  CASE_LOG="$log"
  CASE_OUTPUT="$output"
  CASE_STATUS="$status"
  echo "[ok] ran $name"
}

test_default_mode_does_not_start_or_cleanup() {
  run_case "default mode does not start or cleanup"
  assert_status 0 "$CASE_STATUS" "default mode"
  assert_contains "$CASE_LOG" "docker compose config"
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
test_missing_required_command_fails_before_cleanup docker
test_missing_required_command_fails_before_cleanup rg
test_missing_required_command_fails_before_cleanup curl
test_sigterm_during_live_validation_cleans_stack
test_sigint_during_live_validation_cleans_stack

echo
echo "[ok] deploy release validation script tests passed"
