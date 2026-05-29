#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="help"
API_BASE="${SMOKE_API_BASE_URL:-http://127.0.0.1:8081/api/v1}"
CONFIRM_VALUE="I_UNDERSTAND_COSTS"
MAX_OUTPUT_COUNT=2
MAX_TIMEOUT_SECONDS=600
COOKIE_JAR=""
TMP_FILES=()

usage() {
  cat <<'EOF'
Usage: bash scripts/real-provider-smoke.sh [--help] [--dry-run] [--run]

Manual, opt-in smoke test for a real AI Provider through the platform backend.

Default behavior is safe:
  - no arguments show this help and do not call any API
  - --dry-run validates local script guardrails and prints required --run env names
  - --run is the only mode that can create Provider/model/project/task data

Required for --run:
  REAL_PROVIDER_SMOKE_CONFIRM=I_UNDERSTAND_COSTS
  SMOKE_ADMIN_EMAIL
  SMOKE_ADMIN_PASSWORD
  SMOKE_PROVIDER_API_KEY
  SMOKE_MODEL_NAME

Common optional env:
  SMOKE_API_BASE_URL              Default http://127.0.0.1:8081/api/v1
  SMOKE_PROVIDER_TYPE             Default OPENAI_COMPATIBLE
  SMOKE_PROVIDER_BASE_URL         Default https://api.openai.com/v1
  SMOKE_PROVIDER_NAME_PREFIX      Default codex-smoke-provider
  SMOKE_PROJECT_NAME_PREFIX       Default codex-smoke-project
  SMOKE_PROVIDER_MODEL_NAME       Deprecated alias for SMOKE_MODEL_NAME
  SMOKE_SIZE                      Default 1024x1024
  SMOKE_QUALITY                   Default standard
  SMOKE_OUTPUT_FORMAT             Default png
  SMOKE_OUTPUT_COUNT              Default 1, max 2
  SMOKE_TIMEOUT_SECONDS           Default 180, max 600
  SMOKE_ALLOW_INIT_ADMIN          Set to 1 to initialize admin when login fails
  SMOKE_TENANT_NAME               Required only when SMOKE_ALLOW_INIT_ADMIN=1
  SMOKE_ADMIN_DISPLAY_NAME        Default Codex Smoke Admin

The script never calls OpenAI, Gemini, or relay URLs directly. It only calls
the configured platform /api/v1 backend. Do not put real secrets in shell
history, committed files, logs, or screenshots.
EOF
}

log() {
  printf '[real-provider-smoke] %s\n' "$*"
}

fail() {
  printf '[real-provider-smoke][fail] %s\n' "$*" >&2
  exit 1
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "required command not found: $1"
  fi
}

cleanup() {
  local file
  for file in "${TMP_FILES[@]:-}"; do
    rm -f "$file"
  done
  if [[ -n "${COOKIE_JAR:-}" ]]; then
    rm -f "$COOKIE_JAR"
  fi
}

trap cleanup EXIT

tracked_mktemp() {
  local file
  file="$(mktemp)"
  TMP_FILES+=("$file")
  printf '%s\n' "$file"
}

trim_api_base() {
  API_BASE="${API_BASE%/}"
}

validate_api_base() {
  python3 - "$API_BASE" <<'PY'
from urllib.parse import urlparse
import sys

raw = sys.argv[1]
parsed = urlparse(raw)
host = (parsed.hostname or "").lower()
path = parsed.path.rstrip("/")
blocked_hosts = {
    "api.openai.com",
    "generativelanguage.googleapis.com",
    "gemini.googleapis.com",
    "ai.google.dev",
}
if parsed.scheme not in {"http", "https"} or not host:
    raise SystemExit("SMOKE_API_BASE_URL must be an http(s) URL")
if host in blocked_hosts or host.endswith(".openai.com") or host.endswith(".googleapis.com"):
    raise SystemExit("SMOKE_API_BASE_URL must point to this platform backend, not an AI Provider")
if not path.endswith("/api/v1"):
    raise SystemExit("SMOKE_API_BASE_URL must include the platform /api/v1 prefix")
PY
}

json_get() {
  local file="$1"
  local path="$2"
  python3 - "$file" "$path" <<'PY'
import json
import sys

path = sys.argv[2].split(".")
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    value = json.load(fh)
for part in path:
    if isinstance(value, dict) and part in value:
        value = value[part]
    else:
        sys.exit(1)
if value is None:
    sys.exit(1)
if isinstance(value, (dict, list)):
    print(json.dumps(value, separators=(",", ":")))
else:
    print(value)
PY
}

json_array_len() {
  local file="$1"
  local path="$2"
  python3 - "$file" "$path" <<'PY'
import json
import sys

path = sys.argv[2].split(".")
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    value = json.load(fh)
for part in path:
    if isinstance(value, dict) and part in value:
        value = value[part]
    else:
        print(0)
        sys.exit(0)
print(len(value) if isinstance(value, list) else 0)
PY
}

write_json() {
  local target="$1"
  local template="$2"
  python3 - "$target" "$template" <<'PY'
import json
import os
import sys

target, template = sys.argv[1], sys.argv[2]
env = os.environ

if template == "login":
    payload = {"email": env["SMOKE_ADMIN_EMAIL"], "password": env["SMOKE_ADMIN_PASSWORD"]}
elif template == "init_admin":
    payload = {
        "tenantName": env["SMOKE_TENANT_NAME"],
        "email": env["SMOKE_ADMIN_EMAIL"],
        "password": env["SMOKE_ADMIN_PASSWORD"],
        "displayName": env.get("SMOKE_ADMIN_DISPLAY_NAME", "Codex Smoke Admin"),
    }
elif template == "provider":
    payload = {
        "type": env.get("SMOKE_PROVIDER_TYPE", "OPENAI_COMPATIBLE"),
        "name": env["SMOKE_PROVIDER_NAME"],
        "baseUrl": env.get("SMOKE_PROVIDER_BASE_URL", "https://api.openai.com/v1"),
        "apiKey": env["SMOKE_PROVIDER_API_KEY"],
        "status": "ENABLED",
        "timeoutSeconds": int(env.get("SMOKE_PROVIDER_TIMEOUT_SECONDS", "60")),
        "concurrencyLimit": 1,
    }
elif template == "model":
    payload = {
        "providerId": env["SMOKE_PROVIDER_ID"],
        "modelName": env["SMOKE_MODEL_NAME_EFFECTIVE"],
        "displayName": env.get("SMOKE_MODEL_DISPLAY_NAME", "Codex Smoke Image Model"),
        "supportsGenerate": True,
        "supportsEdit": True,
        "supportsMultiReference": True,
        "supportsN": True,
        "maxOutputCount": int(env.get("SMOKE_OUTPUT_COUNT", "1")),
        "supportedSizes": [env.get("SMOKE_SIZE", "1024x1024")],
        "supportedQualities": [env.get("SMOKE_QUALITY", "standard")],
        "supportedOutputFormats": [env.get("SMOKE_OUTPUT_FORMAT", "png")],
        "pricing": {"currency": env.get("SMOKE_PRICING_CURRENCY", "USD"), "unitPrices": {}},
        "status": "ENABLED",
    }
elif template == "project":
    payload = {"name": env["SMOKE_PROJECT_NAME"], "brand": env.get("SMOKE_PROJECT_BRAND", "codex-smoke")}
elif template == "task":
    payload = {
        "type": "IMAGE_GENERATION",
        "prompt": env.get("SMOKE_PROMPT", "Codex smoke test: generate one simple product image for platform verification."),
        "providerId": env["SMOKE_PROVIDER_ID"],
        "modelId": env["SMOKE_MODEL_ID"],
        "imageType": env.get("SMOKE_IMAGE_TYPE", "MAIN"),
        "parameters": {
            "size": env.get("SMOKE_SIZE", "1024x1024"),
            "quality": env.get("SMOKE_QUALITY", "standard"),
            "outputFormat": env.get("SMOKE_OUTPUT_FORMAT", "png"),
            "outputCount": int(env.get("SMOKE_OUTPUT_COUNT", "1")),
        },
    }
else:
    raise SystemExit(f"unknown template {template}")

with open(target, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, separators=(",", ":"))
PY
}

csrf_token() {
  local name="${SMOKE_CSRF_COOKIE_NAME:-studio_csrf}"
  awk -v name="$name" '$6 == name { value = $7 } END { if (value != "") print value }' "$COOKIE_JAR"
}

api_request() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"
  local output="$4"
  local csrf="${5:-0}"
  local status
  local url="$API_BASE$path"
  local -a args=(-ksS -X "$method" -b "$COOKIE_JAR" -c "$COOKIE_JAR" -H "Accept: application/json" -o "$output" -w "%{http_code}" --max-time 30)
  if [[ "$csrf" == "1" ]]; then
    local token
    token="$(csrf_token)"
    [[ -n "$token" ]] || fail "missing CSRF token after authentication"
    args+=(-H "X-CSRF-Token: $token")
  fi
  if [[ -n "$payload" ]]; then
    args+=(-H "Content-Type: application/json" --data-binary "@$payload")
  fi
  args+=("$url")

  set +e
  status="$(curl "${args[@]}")"
  local curl_status=$?
  set -e
  if [[ "$curl_status" -ne 0 ]]; then
    fail "curl failed during $method $path"
  fi
  if [[ ! "$status" =~ ^2 ]]; then
    fail "API request failed during $method $path with HTTP $status"
  fi
}

health_check() {
  local output
  output="$(tracked_mktemp)"
  api_request GET "/healthz" "" "$output" 0
  rm -f "$output"
}

login_or_init() {
  local payload output status
  payload="$(tracked_mktemp)"
  output="$(tracked_mktemp)"
  write_json "$payload" login
  set +e
  status="$(curl -ksS -X POST -b "$COOKIE_JAR" -c "$COOKIE_JAR" -H "Accept: application/json" -H "Content-Type: application/json" --data-binary "@$payload" -o "$output" -w "%{http_code}" --max-time 30 "$API_BASE/auth/login")"
  local curl_status=$?
  set -e
  rm -f "$payload"
  if [[ "$curl_status" -ne 0 ]]; then
    rm -f "$output"
    fail "curl failed during auth login"
  fi
  if [[ "$status" =~ ^2 ]]; then
    rm -f "$output"
    log "authenticated with existing admin credentials"
    return 0
  fi
  rm -f "$output"

  if [[ "${SMOKE_ALLOW_INIT_ADMIN:-0}" != "1" ]]; then
    fail "auth login failed with HTTP $status; set SMOKE_ALLOW_INIT_ADMIN=1 only for first-run environments"
  fi
  [[ -n "${SMOKE_TENANT_NAME:-}" ]] || fail "missing required env for init-admin: SMOKE_TENANT_NAME"

  payload="$(tracked_mktemp)"
  output="$(tracked_mktemp)"
  write_json "$payload" init_admin
  api_request POST "/auth/init-admin" "$payload" "$output" 0
  rm -f "$payload" "$output"
  log "initialized admin in first-run environment"
}

create_provider_model_project_task() {
  local payload output

  export SMOKE_PROVIDER_NAME="${SMOKE_PROVIDER_NAME_PREFIX:-codex-smoke-provider}-$(date +%Y%m%d%H%M%S)"
  export SMOKE_PROJECT_NAME="${SMOKE_PROJECT_NAME_PREFIX:-codex-smoke-project}-$(date +%Y%m%d%H%M%S)"
  export SMOKE_MODEL_NAME_EFFECTIVE="${SMOKE_MODEL_NAME:-${SMOKE_PROVIDER_MODEL_NAME:-}}"

  payload="$(tracked_mktemp)"
  output="$(tracked_mktemp)"
  write_json "$payload" provider
  api_request POST "/providers" "$payload" "$output" 1
  export SMOKE_PROVIDER_ID
  SMOKE_PROVIDER_ID="$(json_get "$output" data.id)"
  rm -f "$payload" "$output"
  log "created smoke Provider id=$SMOKE_PROVIDER_ID key=[REDACTED]"

  payload="$(tracked_mktemp)"
  output="$(tracked_mktemp)"
  write_json "$payload" model
  api_request POST "/models" "$payload" "$output" 1
  export SMOKE_MODEL_ID
  SMOKE_MODEL_ID="$(json_get "$output" data.id)"
  rm -f "$payload" "$output"
  log "created smoke model id=$SMOKE_MODEL_ID"

  payload="$(tracked_mktemp)"
  output="$(tracked_mktemp)"
  write_json "$payload" project
  api_request POST "/projects" "$payload" "$output" 1
  export SMOKE_PROJECT_ID
  SMOKE_PROJECT_ID="$(json_get "$output" data.id)"
  rm -f "$payload" "$output"
  log "created smoke project id=$SMOKE_PROJECT_ID"

  payload="$(tracked_mktemp)"
  output="$(tracked_mktemp)"
  write_json "$payload" task
  api_request POST "/projects/$SMOKE_PROJECT_ID/tasks" "$payload" "$output" 1
  export SMOKE_TASK_ID
  SMOKE_TASK_ID="$(json_get "$output" data.id)"
  rm -f "$payload" "$output"
  log "submitted smoke task id=$SMOKE_TASK_ID outputCount=${SMOKE_OUTPUT_COUNT:-1}"
}

watch_task_sse() {
  local sse_output
  sse_output="$(tracked_mktemp)"
  set +e
  curl -ksS -N -b "$COOKIE_JAR" -H "Accept: text/event-stream" --max-time "${SMOKE_TIMEOUT_SECONDS:-180}" "$API_BASE/events/tasks?taskId=$SMOKE_TASK_ID" >"$sse_output" 2>/dev/null
  local curl_status=$?
  set -e
  if [[ "$curl_status" -ne 0 && "$curl_status" -ne 28 ]]; then
    rm -f "$sse_output"
    fail "SSE task stream failed"
  fi
  if grep -Eq 'event: (task\.completed|completed|task\.failed|failed|task\.cancelled|cancelled|task\.timed_out|timed_out)' "$sse_output"; then
    log "observed terminal task event via SSE"
  else
    log "SSE watch reached timeout; checking task detail once"
  fi
  rm -f "$sse_output"
}

verify_task_output() {
  local output status asset_count
  output="$(tracked_mktemp)"
  api_request GET "/tasks/$SMOKE_TASK_ID" "" "$output" 0
  status="$(json_get "$output" data.status || true)"
  if [[ "$status" != "SUCCEEDED" ]]; then
    rm -f "$output"
    fail "smoke task did not succeed; final status=${status:-unknown}"
  fi
  asset_count="$(json_array_len "$output" data.outputAssetIds)"
  rm -f "$output"
  if [[ "$asset_count" -lt 1 ]]; then
    fail "smoke task succeeded without output assets"
  fi
  log "smoke task succeeded with outputAssetCount=$asset_count"
}

validate_run_env() {
  local missing=()
  [[ -n "${SMOKE_ADMIN_EMAIL:-}" ]] || missing+=("SMOKE_ADMIN_EMAIL")
  [[ -n "${SMOKE_ADMIN_PASSWORD:-}" ]] || missing+=("SMOKE_ADMIN_PASSWORD")
  [[ -n "${SMOKE_PROVIDER_API_KEY:-}" ]] || missing+=("SMOKE_PROVIDER_API_KEY")
  [[ -n "${SMOKE_MODEL_NAME:-${SMOKE_PROVIDER_MODEL_NAME:-}}" ]] || missing+=("SMOKE_MODEL_NAME")
  if [[ "${#missing[@]}" -gt 0 ]]; then
    fail "missing required env: ${missing[*]}"
  fi

  local count="${SMOKE_OUTPUT_COUNT:-1}"
  if [[ ! "$count" =~ ^[0-9]+$ || "$count" -lt 1 || "$count" -gt "$MAX_OUTPUT_COUNT" ]]; then
    fail "SMOKE_OUTPUT_COUNT must be between 1 and $MAX_OUTPUT_COUNT"
  fi
  local timeout="${SMOKE_TIMEOUT_SECONDS:-180}"
  if [[ ! "$timeout" =~ ^[0-9]+$ || "$timeout" -lt 30 || "$timeout" -gt "$MAX_TIMEOUT_SECONDS" ]]; then
    fail "SMOKE_TIMEOUT_SECONDS must be between 30 and $MAX_TIMEOUT_SECONDS"
  fi
}

dry_run() {
  trim_api_base
  require_command python3
  validate_api_base
  log "dry-run only; no API calls and no Provider calls will be made"
  log "target API base: $API_BASE"
  log "required for --run: REAL_PROVIDER_SMOKE_CONFIRM, SMOKE_ADMIN_EMAIL, SMOKE_ADMIN_PASSWORD, SMOKE_PROVIDER_API_KEY, SMOKE_MODEL_NAME"
  log "cost guardrails: default outputCount=1, max outputCount=$MAX_OUTPUT_COUNT, max timeout=${MAX_TIMEOUT_SECONDS}s"
}

run_smoke() {
  if [[ "${REAL_PROVIDER_SMOKE_CONFIRM:-}" != "$CONFIRM_VALUE" ]]; then
    fail "refusing --run without REAL_PROVIDER_SMOKE_CONFIRM=$CONFIRM_VALUE"
  fi
  validate_run_env
  trim_api_base
  require_command curl
  require_command python3
  validate_api_base
  COOKIE_JAR="$(tracked_mktemp)"

  log "starting real Provider smoke through platform backend only"
  log "target API base: $API_BASE"
  health_check
  login_or_init
  create_provider_model_project_task
  watch_task_sse
  verify_task_output
  log "cleanup note: review and remove codex-smoke Provider/model/project/task data if this was a one-off validation"
  log "real Provider smoke completed"
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --help|-h)
      MODE="help"
      ;;
    --dry-run)
      MODE="dry-run"
      ;;
    --run)
      MODE="run"
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
  shift
done

case "$MODE" in
  help)
    usage
    ;;
  dry-run)
    dry_run
    ;;
  run)
    run_smoke
    ;;
esac
