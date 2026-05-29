#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/real-provider-smoke.sh"
REAL_BASH="${BASH:-/usr/bin/env bash}"

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

setup_fake_bin() {
  local tmpdir="$1"
  local bindir="$tmpdir/bin"
  mkdir -p "$bindir"

  cat >"$bindir/curl" <<EOF
#!$REAL_BASH
EOF
  cat >>"$bindir/curl" <<'EOF'
set -euo pipefail
printf 'curl %s\n' "$*" >>"$FAKE_LOG"

out=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    -o)
      out="$2"
      shift 2
      ;;
    -w|-X|-H|-b|-c|--max-time|--data-binary)
      shift 2
      ;;
    -k|-s|-S|-N|-ksS)
      shift
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -n "$out" ]]; then
  printf '{"error":{"code":"INTERNAL_ERROR","message":"sk-fake-provider-secret Authorization Cookie JWT base64 tenants/object-key"}}' >"$out"
fi
printf '500'
EOF
  chmod +x "$bindir/curl"

  cat >"$bindir/python3" <<EOF
#!$REAL_BASH
exec /usr/bin/python3 "\$@"
EOF
  chmod +x "$bindir/python3"

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
  local runtime_tmp="$tmpdir/runtime"
  mkdir -p "$runtime_tmp"
  local log="$tmpdir/calls.log"
  local output="$tmpdir/output.log"
  touch "$log"

  set +e +u
  env \
    PATH="$tmpdir/bin:$PATH" \
    FAKE_LOG="$log" \
    TMPDIR="$runtime_tmp" \
    "${env_args[@]}" \
    "$REAL_BASH" "$SCRIPT" "${script_args[@]}" >"$output" 2>&1
  local status=$?
  set -e -u

  CASE_LOG="$log"
  CASE_OUTPUT="$output"
  CASE_RUNTIME_TMP="$runtime_tmp"
  CASE_STATUS="$status"
  echo "[ok] ran $name"
}

test_help_is_safe() {
  run_case "help is safe" --help
  assert_status 0 "$CASE_STATUS" "help"
  assert_contains "$CASE_OUTPUT" "Usage: bash scripts/real-provider-smoke.sh"
  assert_not_contains "$CASE_LOG" "curl "
}

test_no_args_is_help_and_safe() {
  run_case "no args is help and safe"
  assert_status 0 "$CASE_STATUS" "no args"
  assert_contains "$CASE_OUTPUT" "Usage: bash scripts/real-provider-smoke.sh"
  assert_not_contains "$CASE_LOG" "curl "
}

test_dry_run_does_not_call_api() {
  run_case "dry-run does not call API" --dry-run
  assert_status 0 "$CASE_STATUS" "dry-run"
  assert_contains "$CASE_OUTPUT" "dry-run only; no API calls"
  assert_not_contains "$CASE_LOG" "curl "
}

test_run_requires_confirmation_before_any_api_call() {
  run_case "run requires confirmation" \
    SMOKE_ADMIN_EMAIL=admin@example.com \
    SMOKE_ADMIN_PASSWORD=correct-horse-battery-staple \
    SMOKE_PROVIDER_API_KEY=sk-fake-provider-secret \
    SMOKE_MODEL_NAME=fake-image-model \
    --run
  assert_nonzero_status "$CASE_STATUS" "missing confirmation"
  assert_contains "$CASE_OUTPUT" "refusing --run without REAL_PROVIDER_SMOKE_CONFIRM"
  assert_not_contains "$CASE_OUTPUT" "sk-fake-provider-secret"
  assert_not_contains "$CASE_LOG" "curl "
}

test_run_reports_missing_env_without_secret_values() {
  run_case "run reports missing env" \
    REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS \
    SMOKE_PROVIDER_API_KEY=sk-fake-provider-secret \
    --run
  assert_nonzero_status "$CASE_STATUS" "missing env"
  assert_contains "$CASE_OUTPUT" "missing required env:"
  assert_contains "$CASE_OUTPUT" "SMOKE_ADMIN_EMAIL"
  assert_contains "$CASE_OUTPUT" "SMOKE_ADMIN_PASSWORD"
  assert_contains "$CASE_OUTPUT" "SMOKE_MODEL_NAME"
  assert_not_contains "$CASE_OUTPUT" "sk-fake-provider-secret"
  assert_not_contains "$CASE_LOG" "curl "
}

test_run_failure_is_sanitized() {
  run_case "run failure is sanitized" \
    REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS \
    SMOKE_ADMIN_EMAIL=admin@example.com \
    SMOKE_ADMIN_PASSWORD=correct-horse-battery-staple \
    SMOKE_PROVIDER_API_KEY=sk-fake-provider-secret \
    SMOKE_MODEL_NAME=fake-image-model \
    --run
  assert_nonzero_status "$CASE_STATUS" "fake API failure"
  assert_contains "$CASE_LOG" "curl "
  assert_contains "$CASE_OUTPUT" "API request failed during GET /healthz with HTTP 500"
  assert_not_contains "$CASE_OUTPUT" "sk-fake-provider-secret"
  assert_not_contains "$CASE_OUTPUT" "Authorization"
  assert_not_contains "$CASE_OUTPUT" "Cookie"
  assert_not_contains "$CASE_OUTPUT" "JWT"
  assert_not_contains "$CASE_OUTPUT" "base64"
  assert_not_contains "$CASE_OUTPUT" "tenants/object-key"
  if grep -R "sk-fake-provider-secret" "$CASE_RUNTIME_TMP" >/dev/null 2>&1; then
    echo "[fail] expected runtime temp files to be cleaned after failure" >&2
    exit 1
  fi
}

test_run_rejects_direct_provider_api_base_before_any_api_call() {
  run_case "run rejects direct Provider API base" \
    REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS \
    SMOKE_API_BASE_URL=https://api.openai.com/v1 \
    SMOKE_ADMIN_EMAIL=admin@example.com \
    SMOKE_ADMIN_PASSWORD=correct-horse-battery-staple \
    SMOKE_PROVIDER_API_KEY=sk-fake-provider-secret \
    SMOKE_MODEL_NAME=fake-image-model \
    --run
  assert_nonzero_status "$CASE_STATUS" "direct Provider API base"
  assert_contains "$CASE_OUTPUT" "SMOKE_API_BASE_URL must point to this platform backend"
  assert_not_contains "$CASE_OUTPUT" "sk-fake-provider-secret"
  assert_not_contains "$CASE_LOG" "curl "
}

test_help_is_safe
test_no_args_is_help_and_safe
test_dry_run_does_not_call_api
test_run_requires_confirmation_before_any_api_call
test_run_reports_missing_env_without_secret_values
test_run_failure_is_sanitized
test_run_rejects_direct_provider_api_base_before_any_api_call

echo
echo "[ok] real Provider smoke script tests passed"
