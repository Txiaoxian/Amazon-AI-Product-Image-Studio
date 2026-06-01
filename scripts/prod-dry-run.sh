#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
RUN_LIVE_COMPOSE=0
RUN_REAL_PROVIDER_SMOKE=0
PRODUCTION_ENV_FILE=""
TMP_FILES=()
ENV_KEYS=()
ENV_VALUES=()

usage() {
  cat <<'EOF'
Usage: bash scripts/prod-dry-run.sh [--live-compose] [--real-provider-smoke] [--production-env-file PATH] [--help]

Runs the sanitized P18 production dry-run summary for operator rehearsal.

Default safe checks:
  - deployment release validation without persistent services
  - focused security regression
  - real Provider smoke guardrail dry-run only
  - backup/restore rehearsal guardrail dry-run only
  - docker compose config validation

Options:
  --live-compose              Delegate live stack checks and scoped cleanup to
                              deploy-release-validation.sh --up --down.
  --real-provider-smoke       After safe checks, delegate the optional billable
                              smoke to real-provider-smoke.sh --run. Requires
                              REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS.
  --production-env-file PATH  Read PATH without sourcing it and fail closed
                              unless required production settings pass preflight.
  --help                      Show this help text without executing checks.

The default mode does not call a real AI Provider. Delegated command output is
summarized by stage so secrets, response bodies, object keys, and signed URLs
are not copied into review evidence.
EOF
}

cleanup() {
  local file
  for file in "${TMP_FILES[@]:-}"; do
    rm -f "$file"
  done
}

trap cleanup EXIT

trim_whitespace() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

fail() {
  printf '[prod-dry-run][fail] %s\n' "$*" >&2
  exit 1
}

log() {
  printf '[prod-dry-run] %s\n' "$*"
}

run_stage() {
  local name="$1"
  shift
  local output
  output="$(mktemp)"
  TMP_FILES+=("$output")

  log "stage: $name"
  set +e
  "$@" >"$output" 2>&1
  local status=$?
  set -e
  rm -f "$output"
  if [[ "$status" -ne 0 ]]; then
    fail "stage failed: $name (exit $status)"
  fi
  log "stage passed: $name"
}

set_env_value() {
  local key="$1"
  local value="$2"
  local index
  for ((index = 0; index < ${#ENV_KEYS[@]}; index++)); do
    if [[ "${ENV_KEYS[$index]}" == "$key" ]]; then
      ENV_VALUES[$index]="$value"
      return 0
    fi
  done
  ENV_KEYS+=("$key")
  ENV_VALUES+=("$value")
}

get_env_value() {
  local key="$1"
  local index
  for ((index = 0; index < ${#ENV_KEYS[@]}; index++)); do
    if [[ "${ENV_KEYS[$index]}" == "$key" ]]; then
      printf '%s' "${ENV_VALUES[$index]}"
      return 0
    fi
  done
  return 1
}

validate_production_env_file() {
  local env_file="$1"
  if [[ ! -f "$env_file" ]]; then
    fail "production env file not found"
  fi

  ENV_KEYS=()
  ENV_VALUES=()
  local line line_number=0 key value
  while IFS= read -r line || [[ -n "$line" ]]; do
    line_number=$((line_number + 1))
    line="$(trim_whitespace "$line")"
    if [[ -z "$line" || "${line:0:1}" == "#" ]]; then
      continue
    fi
    if [[ "$line" == export[[:space:]]* ]]; then
      line="$(trim_whitespace "${line#export}")"
    fi
    if [[ "$line" != *=* ]]; then
      fail "invalid production env syntax at line $line_number"
    fi
    key="$(trim_whitespace "${line%%=*}")"
    value="$(trim_whitespace "${line#*=}")"
    if [[ ! "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      fail "invalid production env key at line $line_number"
    fi
    if [[ ${#value} -ge 2 && "${value:0:1}" == '"' && "${value: -1}" == '"' ]]; then
      value="${value:1:${#value}-2}"
    elif [[ ${#value} -ge 2 && "${value:0:1}" == "'" && "${value: -1}" == "'" ]]; then
      value="${value:1:${#value}-2}"
    fi
    set_env_value "$key" "$value"
  done <"$env_file"

  local required=(
    APP_ENV
    COOKIE_SECURE
    CORS_ALLOWED_ORIGINS
    MYSQL_ROOT_PASSWORD
    MYSQL_PASSWORD
    REDIS_PASSWORD
    MINIO_ROOT_USER
    MINIO_ROOT_PASSWORD
    MINIO_ACCESS_KEY
    MINIO_SECRET_KEY
    JWT_SIGNING_SECRET
    API_KEY_ENCRYPTION_KEY
  )
  for key in "${required[@]}"; do
    value="$(get_env_value "$key" || true)"
    if [[ -z "$value" ]]; then
      fail "missing required production env: $key"
    fi
  done

  if [[ "$(get_env_value APP_ENV)" != "production" ]]; then
    fail "APP_ENV must be production"
  fi
  if [[ "$(get_env_value COOKIE_SECURE)" != "true" ]]; then
    fail "COOKIE_SECURE must be true"
  fi
  if value="$(get_env_value CSRF_HEADER_NAME)"; then
    if [[ "$value" != "X-CSRF-Token" ]]; then
      fail "CSRF_HEADER_NAME must be X-CSRF-Token when set"
    fi
  fi

  local secret_keys=(
    MYSQL_ROOT_PASSWORD
    MYSQL_PASSWORD
    REDIS_PASSWORD
    MINIO_ROOT_USER
    MINIO_ROOT_PASSWORD
    MINIO_ACCESS_KEY
    MINIO_SECRET_KEY
    JWT_SIGNING_SECRET
    API_KEY_ENCRYPTION_KEY
  )
  local normalized
  for key in "${secret_keys[@]}"; do
    normalized="$(get_env_value "$key" | tr '[:upper:]' '[:lower:]')"
    if [[ "$normalized" == *change-me* ]]; then
      fail "production env contains forbidden placeholder: $key"
    fi
  done

  local -a origins=()
  local origin authority host normalized_host
  local first_octet second_octet third_octet fourth_octet
  IFS=',' read -r -a origins <<<"$(get_env_value CORS_ALLOWED_ORIGINS)"
  if [[ "${#origins[@]}" -eq 0 ]]; then
    fail "CORS_ALLOWED_ORIGINS must contain restricted non-localhost HTTPS origins"
  fi
  for origin in "${origins[@]}"; do
    origin="$(trim_whitespace "$origin")"
    if [[ -z "$origin" ||
      ! "$origin" =~ ^https://([A-Za-z0-9-]+\.)*[A-Za-z0-9-]+(:[0-9]+)?$ ||
      "$origin" == *"*"* ]]; then
      fail "CORS_ALLOWED_ORIGINS must contain restricted non-localhost HTTPS origins"
    fi

    authority="${origin#https://}"
    host="${authority%%:*}"
    normalized_host="$(printf '%s' "$host" | tr '[:upper:]' '[:lower:]')"
    if [[ "$normalized_host" == "localhost" || "$normalized_host" == *".localhost" ]]; then
      fail "CORS_ALLOWED_ORIGINS must contain restricted non-localhost HTTPS origins"
    fi
    if [[ "$normalized_host" =~ ^([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})$ ]]; then
      first_octet="${BASH_REMATCH[1]}"
      second_octet="${BASH_REMATCH[2]}"
      third_octet="${BASH_REMATCH[3]}"
      fourth_octet="${BASH_REMATCH[4]}"
      if (( first_octet > 255 || second_octet > 255 || third_octet > 255 || fourth_octet > 255 ||
        first_octet == 0 || first_octet == 10 || first_octet == 127 ||
        (first_octet == 169 && second_octet == 254) ||
        (first_octet == 172 && second_octet >= 16 && second_octet <= 31) ||
        (first_octet == 192 && second_octet == 168) )); then
        fail "CORS_ALLOWED_ORIGINS must contain restricted non-localhost HTTPS origins"
      fi
    fi
  done

  log "production env preflight passed"
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --live-compose)
      RUN_LIVE_COMPOSE=1
      ;;
    --real-provider-smoke)
      RUN_REAL_PROVIDER_SMOKE=1
      ;;
    --production-env-file)
      shift
      if [[ "$#" -eq 0 || -z "${1:-}" ]]; then
        usage >&2
        exit 2
      fi
      PRODUCTION_ENV_FILE="$1"
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

if [[ "$RUN_REAL_PROVIDER_SMOKE" -eq 1 && "${REAL_PROVIDER_SMOKE_CONFIRM:-}" != "I_UNDERSTAND_COSTS" ]]; then
  fail "optional real Provider smoke is billable; set REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS to continue"
fi

if [[ -n "$PRODUCTION_ENV_FILE" ]]; then
  log "stage: production env preflight"
  validate_production_env_file "$PRODUCTION_ENV_FILE"
fi

log "starting sanitized production dry-run"
run_stage "deployment release validation" bash "$ROOT_DIR/scripts/deploy-release-validation.sh"
run_stage "security regression" bash "$ROOT_DIR/scripts/security-regression.sh"
run_stage "real Provider smoke guardrail dry-run" bash "$ROOT_DIR/scripts/real-provider-smoke.sh" --dry-run
run_stage "backup/restore rehearsal guardrail dry-run" bash "$ROOT_DIR/scripts/backup-restore-rehearsal.sh"
run_stage "Compose config validation" docker compose -f "$COMPOSE_FILE" config

if [[ "$RUN_LIVE_COMPOSE" -eq 1 ]]; then
  run_stage "live Compose validation with scoped cleanup" \
    bash "$ROOT_DIR/scripts/deploy-release-validation.sh" --up --down
fi

if [[ "$RUN_REAL_PROVIDER_SMOKE" -eq 1 ]]; then
  log "optional real Provider smoke explicitly enabled; billable backend-mediated call may run"
  run_stage "optional real Provider smoke" bash "$ROOT_DIR/scripts/real-provider-smoke.sh" --run
fi

log "sanitized evidence summary: all requested stages passed"
log "backup/restore rehearsal: isolated live mode remains explicit; do not attach dumps, secrets, object keys, signed URLs, Provider responses, or image outputs"
