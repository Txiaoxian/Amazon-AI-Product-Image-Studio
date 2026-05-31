#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/prod-dry-run.sh"
REAL_BASH="${BASH:-/bin/bash}"

assert_contains() {
  local file="$1"
  local pattern="$2"
  if ! grep -Fq -- "$pattern" "$file"; then
    echo "[fail] expected output to contain: $pattern" >&2
    sed 's/^/[debug] /' "$file" >&2
    exit 1
  fi
}

assert_not_contains() {
  local file="$1"
  local pattern="$2"
  if grep -Fq -- "$pattern" "$file"; then
    echo "[fail] expected output not to contain: $pattern" >&2
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

assert_line_order() {
  local file="$1"
  shift
  local previous=0
  local pattern line
  for pattern in "$@"; do
    line="$(grep -nF -- "$pattern" "$file" | head -n 1 | cut -d: -f1 || true)"
    if [[ -z "$line" ]]; then
      echo "[fail] expected ordered log entry: $pattern" >&2
      sed 's/^/[debug] /' "$file" >&2
      exit 1
    fi
    if [[ "$line" -le "$previous" ]]; then
      echo "[fail] log entry out of order: $pattern" >&2
      sed 's/^/[debug] /' "$file" >&2
      exit 1
    fi
    previous="$line"
  done
}

setup_fake_bin() {
  local tmpdir="$1"
  local bindir="$tmpdir/bin"
  mkdir -p "$bindir"

  cat >"$bindir/bash" <<EOF
#!$REAL_BASH
EOF
  cat >>"$bindir/bash" <<'EOF'
set -euo pipefail
printf 'bash %s\n' "$*" >>"$FAKE_LOG"
if [[ "${FAKE_CHILD_PRINT_SECRET:-0}" == "1" ]]; then
  printf 'SMOKE_PROVIDER_API_KEY: %s\n' "${SMOKE_PROVIDER_API_KEY:-sk-fake-child-secret}" >&2
fi
if [[ -n "${FAKE_FAIL_SCRIPT:-}" && "${1:-}" == *"/$FAKE_FAIL_SCRIPT" ]]; then
  exit 17
fi
exit 0
EOF
  chmod +x "$bindir/bash"

  cat >"$bindir/docker" <<EOF
#!$REAL_BASH
EOF
  cat >>"$bindir/docker" <<'EOF'
set -euo pipefail
printf 'docker %s\n' "$*" >>"$FAKE_LOG"
if [[ "${FAKE_DOCKER_FAIL:-0}" == "1" ]]; then
  printf 'MYSQL_PASSWORD: fake-database-secret\n' >&2
  exit 19
fi
exit 0
EOF
  chmod +x "$bindir/docker"

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
  env \
    -u REAL_PROVIDER_SMOKE_CONFIRM \
    -u SMOKE_ADMIN_PASSWORD \
    -u SMOKE_PROVIDER_API_KEY \
    PATH="$tmpdir/bin:$PATH" \
    FAKE_LOG="$log" \
    "${env_args[@]}" \
    "$REAL_BASH" "$SCRIPT" "${script_args[@]}" >"$output" 2>&1
  local status=$?
  set -e -u

  CASE_LOG="$log"
  CASE_OUTPUT="$output"
  CASE_STATUS="$status"
  echo "[ok] ran $name"
}

write_valid_production_env() {
  local target="$1"
  cat >"$target" <<'EOF'
APP_ENV=production
COOKIE_SECURE=true
CORS_ALLOWED_ORIGINS=https://studio.example.com
MYSQL_ROOT_PASSWORD=fake-mysql-root-secret
MYSQL_PASSWORD=fake-mysql-secret
REDIS_PASSWORD=fake-redis-secret
MINIO_ROOT_USER=fake-minio-root-user
MINIO_ROOT_PASSWORD=fake-minio-root-secret
MINIO_ACCESS_KEY=fake-minio-access-key
MINIO_SECRET_KEY=fake-minio-secret-key
JWT_SIGNING_SECRET=fake-jwt-signing-secret
API_KEY_ENCRYPTION_KEY=fake-api-key-encryption-secret
EOF
}

test_help_is_safe() {
  run_case "help is safe" --help
  assert_status 0 "$CASE_STATUS" "help"
  assert_contains "$CASE_OUTPUT" "Usage: bash scripts/prod-dry-run.sh"
  if [[ -s "$CASE_LOG" ]]; then
    echo "[fail] help must not execute delegated commands" >&2
    exit 1
  fi
}

test_default_mode_runs_safe_checks_in_order() {
  run_case "default mode runs safe checks"
  assert_status 0 "$CASE_STATUS" "default mode"
  assert_line_order "$CASE_LOG" \
    "bash $ROOT_DIR/scripts/deploy-release-validation.sh" \
    "bash $ROOT_DIR/scripts/security-regression.sh" \
    "bash $ROOT_DIR/scripts/real-provider-smoke.sh --dry-run" \
    "docker compose -f $ROOT_DIR/deploy/docker-compose.yml config"
  assert_not_contains "$CASE_LOG" "real-provider-smoke.sh --run"
}

test_real_provider_smoke_requires_confirmation_before_checks() {
  run_case "real Provider smoke requires confirmation" \
    SMOKE_PROVIDER_API_KEY=sk-fake-provider-secret \
    --real-provider-smoke
  assert_nonzero_status "$CASE_STATUS" "missing real Provider confirmation"
  assert_contains "$CASE_OUTPUT" "REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS"
  assert_contains "$CASE_OUTPUT" "billable"
  assert_not_contains "$CASE_OUTPUT" "sk-fake-provider-secret"
  if [[ -s "$CASE_LOG" ]]; then
    echo "[fail] missing confirmation must fail before delegated commands" >&2
    exit 1
  fi
}

test_real_provider_smoke_delegates_run_after_safe_checks() {
  run_case "real Provider smoke delegates run" \
    REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS \
    SMOKE_PROVIDER_API_KEY=sk-fake-provider-secret \
    --real-provider-smoke
  assert_status 0 "$CASE_STATUS" "confirmed real Provider smoke"
  assert_line_order "$CASE_LOG" \
    "bash $ROOT_DIR/scripts/real-provider-smoke.sh --dry-run" \
    "docker compose -f $ROOT_DIR/deploy/docker-compose.yml config" \
    "bash $ROOT_DIR/scripts/real-provider-smoke.sh --run"
  assert_not_contains "$CASE_OUTPUT" "sk-fake-provider-secret"
}

test_live_compose_delegates_scoped_cleanup() {
  run_case "live Compose delegates scoped cleanup" --live-compose
  assert_status 0 "$CASE_STATUS" "live Compose"
  assert_contains "$CASE_LOG" "bash $ROOT_DIR/scripts/deploy-release-validation.sh --up --down"
  assert_not_contains "$CASE_LOG" "system prune"
  assert_not_contains "$CASE_LOG" "volume prune"
}

test_child_failure_is_sanitized_and_nonzero() {
  run_case "child failure is sanitized" \
    FAKE_CHILD_PRINT_SECRET=1 \
    FAKE_FAIL_SCRIPT=security-regression.sh
  assert_nonzero_status "$CASE_STATUS" "child failure"
  assert_contains "$CASE_OUTPUT" "stage failed: security regression"
  assert_not_contains "$CASE_OUTPUT" "sk-fake-child-secret"
  assert_contains "$CASE_LOG" "bash $ROOT_DIR/scripts/security-regression.sh"
  assert_not_contains "$CASE_LOG" "real-provider-smoke.sh --dry-run"
}

test_docker_failure_is_sanitized_and_nonzero() {
  run_case "docker failure is sanitized" FAKE_DOCKER_FAIL=1
  assert_nonzero_status "$CASE_STATUS" "docker failure"
  assert_contains "$CASE_OUTPUT" "stage failed: Compose config validation"
  assert_not_contains "$CASE_OUTPUT" "fake-database-secret"
}

test_production_env_preflight_missing_file_fails_closed() {
  local missing_env_file="/tmp/prod-dry-run-missing-env-file-$$"
  rm -f "$missing_env_file"
  run_case "production env preflight missing file" \
    --production-env-file "$missing_env_file"
  assert_nonzero_status "$CASE_STATUS" "missing production env file"
  assert_contains "$CASE_OUTPUT" "production env file not found"
  if [[ -s "$CASE_LOG" ]]; then
    echo "[fail] missing production env file must fail before delegated commands" >&2
    exit 1
  fi
}

test_production_env_preflight_missing_secret_fails_closed() {
  local env_file filtered_env_file
  env_file="$(mktemp)"
  filtered_env_file="$(mktemp)"
  write_valid_production_env "$env_file"
  grep -v '^REDIS_PASSWORD=' "$env_file" >"$filtered_env_file"
  run_case "production env preflight missing secret" --production-env-file "$filtered_env_file"
  rm -f "$env_file" "$filtered_env_file"
  assert_nonzero_status "$CASE_STATUS" "missing production secret"
  assert_contains "$CASE_OUTPUT" "missing required production env: REDIS_PASSWORD"
  assert_not_contains "$CASE_OUTPUT" "fake-mysql-secret"
  assert_not_contains "$CASE_OUTPUT" "fake-minio-secret-key"
}

test_production_env_preflight_rejects_placeholder_without_leak() {
  local env_file
  env_file="$(mktemp)"
  write_valid_production_env "$env_file"
  printf 'MYSQL_PASSWORD=change-me-fake-secret\n' >>"$env_file"
  run_case "production env preflight rejects placeholder" --production-env-file "$env_file"
  rm -f "$env_file"
  assert_nonzero_status "$CASE_STATUS" "placeholder production secret"
  assert_contains "$CASE_OUTPUT" "production env contains forbidden placeholder: MYSQL_PASSWORD"
  assert_not_contains "$CASE_OUTPUT" "change-me-fake-secret"
}

test_production_env_preflight_rejects_invalid_cookie() {
  local env_file
  env_file="$(mktemp)"
  write_valid_production_env "$env_file"
  printf 'COOKIE_SECURE=false\n' >>"$env_file"
  run_case "production env preflight rejects insecure cookie" --production-env-file "$env_file"
  rm -f "$env_file"
  assert_nonzero_status "$CASE_STATUS" "insecure production cookie"
  assert_contains "$CASE_OUTPUT" "COOKIE_SECURE must be true"
  assert_not_contains "$CASE_OUTPUT" "fake-jwt-signing-secret"
}

test_production_env_preflight_rejects_nonproduction_app_env() {
  local env_file
  env_file="$(mktemp)"
  write_valid_production_env "$env_file"
  printf 'APP_ENV=development\n' >>"$env_file"
  run_case "production env preflight rejects nonproduction app env" --production-env-file "$env_file"
  rm -f "$env_file"
  assert_nonzero_status "$CASE_STATUS" "nonproduction APP_ENV"
  assert_contains "$CASE_OUTPUT" "APP_ENV must be production"
  assert_not_contains "$CASE_OUTPUT" "fake-minio-root-secret"
}

test_production_env_preflight_rejects_localhost_cors() {
  local env_file
  env_file="$(mktemp)"
  write_valid_production_env "$env_file"
  printf 'CORS_ALLOWED_ORIGINS=http://localhost:8080\n' >>"$env_file"
  run_case "production env preflight rejects localhost CORS" --production-env-file "$env_file"
  rm -f "$env_file"
  assert_nonzero_status "$CASE_STATUS" "localhost production CORS"
  assert_contains "$CASE_OUTPUT" "CORS_ALLOWED_ORIGINS must contain restricted non-localhost HTTPS origins"
  assert_not_contains "$CASE_OUTPUT" "fake-api-key-encryption-secret"
}

test_production_env_preflight_rejects_private_and_link_local_cors() {
  local origin env_file
  for origin in \
    "https://10.0.0.1" \
    "https://172.16.0.1" \
    "https://192.168.1.1" \
    "https://169.254.1.1" \
    "https://[::1]" \
    "https://[fd00::1]" \
    "https://[fe80::1]"; do
    env_file="$(mktemp)"
    write_valid_production_env "$env_file"
    printf 'CORS_ALLOWED_ORIGINS=%s\n' "$origin" >>"$env_file"
    run_case "production env preflight rejects restricted CORS origin $origin" --production-env-file "$env_file"
    rm -f "$env_file"
    assert_nonzero_status "$CASE_STATUS" "restricted production CORS origin $origin"
    assert_contains "$CASE_OUTPUT" "CORS_ALLOWED_ORIGINS must contain restricted non-localhost HTTPS origins"
    assert_not_contains "$CASE_OUTPUT" "fake-api-key-encryption-secret"
  done
}

test_production_env_preflight_accepts_valid_template_without_leak() {
  local env_file
  env_file="$(mktemp)"
  write_valid_production_env "$env_file"
  run_case "production env preflight accepts valid template" --production-env-file "$env_file"
  rm -f "$env_file"
  assert_status 0 "$CASE_STATUS" "valid production env"
  assert_contains "$CASE_OUTPUT" "production env preflight passed"
  assert_not_contains "$CASE_OUTPUT" "fake-mysql-root-secret"
  assert_not_contains "$CASE_OUTPUT" "fake-minio-secret-key"
  assert_not_contains "$CASE_OUTPUT" "fake-jwt-signing-secret"
  assert_not_contains "$CASE_OUTPUT" "fake-api-key-encryption-secret"
}

test_help_is_safe
test_default_mode_runs_safe_checks_in_order
test_real_provider_smoke_requires_confirmation_before_checks
test_real_provider_smoke_delegates_run_after_safe_checks
test_live_compose_delegates_scoped_cleanup
test_child_failure_is_sanitized_and_nonzero
test_docker_failure_is_sanitized_and_nonzero
test_production_env_preflight_missing_file_fails_closed
test_production_env_preflight_missing_secret_fails_closed
test_production_env_preflight_rejects_placeholder_without_leak
test_production_env_preflight_rejects_invalid_cookie
test_production_env_preflight_rejects_nonproduction_app_env
test_production_env_preflight_rejects_localhost_cors
test_production_env_preflight_rejects_private_and_link_local_cors
test_production_env_preflight_accepts_valid_template_without_leak

echo
echo "[ok] production dry-run script tests passed"
