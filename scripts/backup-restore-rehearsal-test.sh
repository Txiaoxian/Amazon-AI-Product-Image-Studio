#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/backup-restore-rehearsal.sh"
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

assert_line_order() {
  local file="$1"
  shift
  local previous=0
  local pattern line
  for pattern in "$@"; do
    line="$(grep -nF -- "$pattern" "$file" | head -n 1 | cut -d: -f1 || true)"
    if [[ -z "$line" || "$line" -le "$previous" ]]; then
      echo "[fail] expected ordered log entry: $pattern" >&2
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

  cat >"$bindir/docker" <<EOF
#!$REAL_BASH
EOF
  cat >>"$bindir/docker" <<'EOF'
set -euo pipefail
printf 'docker %s\n' "$*" >>"$FAKE_LOG"

if [[ "$*" == *'/backup/generated'* ]]; then
  previous=""
  for arg in "$@"; do
    if [[ "$previous" == "-v" && "$arg" == *":/backup" ]]; then
      backup_dir="${arg%:/backup}"
      for required_dir in originals generated thumbnails; do
        if [[ ! -d "$backup_dir/$required_dir" ]]; then
          printf 'missing empty backup directory\n' >&2
          exit 23
        fi
      done
    fi
    previous="$arg"
  done
fi

if [[ -n "${FAKE_SIGNAL_MATCH:-}" && "$*" == *"$FAKE_SIGNAL_MATCH"* ]]; then
  kill "-${FAKE_SIGNAL_KIND:-TERM}" "$PPID"
  exit 143
fi

if [[ -n "${FAKE_FAIL_MATCH:-}" && "$*" == *"$FAKE_FAIL_MATCH"* ]]; then
  printf 'MYSQL_ROOT_PASSWORD=fake-root-password\n' >&2
  printf 'MINIO_ACCESS_KEY=fake-minio-access-key\n' >&2
  printf 'bucket=product-originals object_key=tenants/fixture-object\n' >&2
  exit 17
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
    -u BACKUP_RESTORE_REHEARSAL_CONFIRM \
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

test_help_is_safe() {
  run_case "help is safe" --help
  assert_status 0 "$CASE_STATUS" "help"
  assert_contains "$CASE_OUTPUT" "Usage: bash scripts/backup-restore-rehearsal.sh"
  if [[ -s "$CASE_LOG" ]]; then
    echo "[fail] help must not execute Docker commands" >&2
    exit 1
  fi
}

test_default_mode_is_guardrail_only() {
  run_case "default mode is guardrail only"
  assert_status 0 "$CASE_STATUS" "default guardrail mode"
  assert_contains "$CASE_OUTPUT" "guardrail dry-run passed"
  if [[ -s "$CASE_LOG" ]]; then
    echo "[fail] default mode must not execute Docker commands" >&2
    exit 1
  fi
}

test_live_requires_confirmation_before_docker() {
  run_case "live requires confirmation" --live
  assert_nonzero_status "$CASE_STATUS" "missing live confirmation"
  assert_contains "$CASE_OUTPUT" "BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT"
  if [[ -s "$CASE_LOG" ]]; then
    echo "[fail] missing confirmation must fail before Docker commands" >&2
    exit 1
  fi
}

test_confirmed_live_runs_scoped_steps_and_cleanup() {
  run_case "confirmed live runs scoped steps" \
    BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
    --live
  assert_status 0 "$CASE_STATUS" "confirmed live"
  assert_line_order "$CASE_LOG" \
    "up -d mysql minio" \
    "exec -T mysql sh -c mysqladmin ping" \
    "up minio-bootstrap" \
    "CREATE TABLE IF NOT EXISTS" \
    "run --rm --no-deps" \
    "down -v --remove-orphans"
  assert_contains "$CASE_OUTPUT" "stage passed: restore matching backup pair"
  assert_contains "$CASE_OUTPUT" "stage passed: rollback restore matching backup pair"
  assert_contains "$CASE_OUTPUT" "stage passed: verify exact restore removed backup-external object"
  assert_contains "$CASE_OUTPUT" "stage passed: verify exact rollback removed backup-external object"
  assert_contains "$CASE_OUTPUT" "sanitized evidence summary: isolated rehearsal passed"
  assert_contains "$CASE_LOG" "compose -p amazon-ai-product-image-studio-backup-restore-rehearsal-"
  assert_contains "$CASE_LOG" "mc mirror --overwrite --remove /backup/originals local/"
  assert_contains "$CASE_LOG" "mc mirror --overwrite --remove /backup/generated local/"
  assert_contains "$CASE_LOG" "mc mirror --overwrite --remove /backup/thumbnails local/"
  assert_not_contains "$CASE_LOG" "system prune"
  assert_not_contains "$CASE_LOG" "volume prune"
  assert_not_contains "$CASE_LOG" "curl "
  assert_not_contains "$CASE_LOG" "api.openai.com"
  assert_not_contains "$CASE_LOG" "googleapis.com"
  assert_not_contains "$CASE_LOG" "127.0.0.1:3306"
  assert_not_contains "$CASE_LOG" "127.0.0.1:9000"
  assert_not_contains "$CASE_LOG" "dev-mysql"
  assert_not_contains "$CASE_LOG" "dev-redis"
  assert_not_contains "$CASE_LOG" "dev-minio"
}

test_failure_attempts_cleanup_and_redacts_output() {
  run_case "failure attempts cleanup" \
    BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
    FAKE_FAIL_MATCH="CREATE TABLE IF NOT EXISTS" \
    --live
  assert_nonzero_status "$CASE_STATUS" "failed live rehearsal"
  assert_contains "$CASE_LOG" "down -v --remove-orphans"
  assert_contains "$CASE_OUTPUT" "stage failed: create isolated fixture metadata"
  assert_not_contains "$CASE_OUTPUT" "fake-root-password"
  assert_not_contains "$CASE_OUTPUT" "fake-minio-access-key"
  assert_not_contains "$CASE_OUTPUT" "product-originals"
  assert_not_contains "$CASE_OUTPUT" "tenants/fixture-object"
}

test_cleanup_failure_is_sanitized_and_nonzero() {
  run_case "cleanup failure is sanitized" \
    BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
    FAKE_FAIL_MATCH="down -v --remove-orphans" \
    --live
  assert_nonzero_status "$CASE_STATUS" "cleanup failure"
  assert_contains "$CASE_OUTPUT" "scoped cleanup failed"
  assert_not_contains "$CASE_OUTPUT" "fake-root-password"
  assert_not_contains "$CASE_OUTPUT" "fake-minio-access-key"
  assert_not_contains "$CASE_OUTPUT" "product-originals"
  assert_not_contains "$CASE_OUTPUT" "tenants/fixture-object"
}

test_sigterm_attempts_cleanup() {
  run_case "SIGTERM attempts cleanup" \
    BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
    FAKE_SIGNAL_MATCH="CREATE TABLE IF NOT EXISTS" \
    --live
  assert_nonzero_status "$CASE_STATUS" "SIGTERM during live rehearsal"
  assert_contains "$CASE_LOG" "down -v --remove-orphans"
}

test_sigint_attempts_cleanup() {
  run_case "SIGINT attempts cleanup" \
    BACKUP_RESTORE_REHEARSAL_CONFIRM=I_UNDERSTAND_DATA_REPLACEMENT \
    FAKE_SIGNAL_KIND=INT \
    FAKE_SIGNAL_MATCH="CREATE TABLE IF NOT EXISTS" \
    --live
  assert_nonzero_status "$CASE_STATUS" "SIGINT during live rehearsal"
  assert_contains "$CASE_LOG" "down -v --remove-orphans"
}

test_help_is_safe
test_default_mode_is_guardrail_only
test_live_requires_confirmation_before_docker
test_confirmed_live_runs_scoped_steps_and_cleanup
test_failure_attempts_cleanup_and_redacts_output
test_cleanup_failure_is_sanitized_and_nonzero
test_sigterm_attempts_cleanup
test_sigint_attempts_cleanup

echo
echo "[ok] backup/restore rehearsal script tests passed"
