#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'EOF'
Usage: bash scripts/security-regression.sh [--help]

Runs the focused P15 final security regression:
  - backend focused security tests
  - frontend focused security tests
  - frontend npm dependency bootstrap when node_modules is absent
  - frontend production-code forbidden-pattern scans
  - backend production-code sensitive logging scans
  - deploy frontend proxy safety scan
  - docker compose config validation
  - git whitespace check against main...HEAD

The script only runs tests and static checks. It does not start MySQL, Redis,
MinIO, AI Providers, or any external relay, and it does not print secret values.
Test-only marker strings are reported separately from production-code scans.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

if [[ "$#" -gt 0 ]]; then
  usage >&2
  exit 2
fi

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
  sed -E \
    -e 's/((PASSWORD|SECRET|API_KEY_ENCRYPTION_KEY|JWT_SIGNING_SECRET|AUTH_COOKIE_NAME|CSRF_COOKIE_NAME): )[[:graph:]]+/\1[redacted]/Ig' \
    -e 's/(MINIO_BUCKET_[A-Z_]+: )[[:graph:]]+/\1[redacted]/g' \
    "$output" >&2
  rm -f "$output"
  return "$status"
}

ensure_frontend_dependencies() {
  if [[ -x "$ROOT_DIR/frontend/node_modules/.bin/vitest" ]]; then
    return 0
  fi
  run bash -lc "cd '$ROOT_DIR/frontend' && npm ci"
}

scan_no_hits() {
  local description="$1"
  shift
  local output
  output="$(mktemp)"
  set +e
  "$@" >"$output"
  local status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    echo
    echo "==> $description"
    sed 's/^/[forbidden] /' "$output"
    rm -f "$output"
    echo "[fail] forbidden production-code pattern found: $description" >&2
    exit 1
  fi
  if [[ "$status" -eq 1 ]]; then
    echo
    echo "==> $description"
    echo "[ok] no production-code hits"
    rm -f "$output"
    return 0
  fi
  cat "$output" >&2
  rm -f "$output"
  return "$status"
}

show_allowed_marker_files() {
  echo
  echo "==> allowed marker inventory"
  echo "[info] Test assertion files with sensitive-marker strings are excluded from production scans:"
  rg -l -i --glob '**/*_test.go' --glob 'src/test/**' \
    'authorization|cookie|jwt|api[_ -]?key|apikey|base64|b64_json|object[_ -]?key|objectKey|bearer|sk-' \
    "$ROOT_DIR/backend" "$ROOT_DIR/frontend/src/test" 2>/dev/null | sed 's/^/[allowed-test] /' || true
  echo "[info] Production helper files with marker names are allowed when they implement auth, Provider calls, encryption, or redaction:"
  rg -l -i --glob '!**/*_test.go' \
    'authorization|cookie|jwt|api[_ -]?key|apikey|base64|b64_json|object[_ -]?key|objectKey|bearer' \
    "$ROOT_DIR/backend/internal/auth" \
    "$ROOT_DIR/backend/internal/config" \
    "$ROOT_DIR/backend/internal/logger" \
    "$ROOT_DIR/backend/internal/provider" \
    "$ROOT_DIR/backend/internal/provideradapter" \
    "$ROOT_DIR/backend/internal/redaction" \
    "$ROOT_DIR/backend/internal/task" 2>/dev/null | sed 's/^/[allowed-helper] /' || true
  echo "[info] Frontend Provider labels/API-key form fields are allowed only as backend-admin UI state, not browser storage or direct Provider calls:"
  rg -l -i --glob '*.ts' --glob '*.tsx' --glob '!src/test/**' --glob '!**/*.test.*' --glob '!**/*.spec.*' \
    'openai|gemini|relay|apiKey' "$ROOT_DIR/frontend/src" 2>/dev/null | sed 's/^/[allowed-frontend-marker] /' || true
  echo "[info] Residual IndexedDB compatibility files are allowed only if legacyRetirement keeps them out of the production import graph:"
  rg -l -i 'Dexie|db\.(historyItems|images)' "$ROOT_DIR/frontend/src/db" 2>/dev/null | sed 's/^/[allowed-frontend-legacy-db] /' || true
}

run bash -lc "cd '$ROOT_DIR/backend' && go test ./internal/api ./internal/provider ./internal/provideradapter ./internal/sse ./internal/task ./internal/audit -count=1"
ensure_frontend_dependencies
run bash -lc "cd '$ROOT_DIR/frontend' && npm run test -- src/test/taskWorkbench.test.tsx src/test/historyAssetSource.test.tsx src/test/adminObservabilitySettingsPanel.test.tsx src/test/authFlow.test.tsx src/test/adminProviderModelPanel.test.tsx src/test/legacyRetirement.test.tsx"

scan_no_hits "frontend production code must not call AI Providers or relay endpoints directly" \
  rg -n -i --glob '*.ts' --glob '*.tsx' --glob '!src/test/**' --glob '!**/*.test.*' --glob '!**/*.spec.*' \
  '(api\.openai\.com|generativelanguage\.googleapis\.com|/v1/images/generations|/v1beta/models|(fetch|XMLHttpRequest|EventSource)[^[:cntrl:]]{0,160}(openai|gemini|relay))' \
  "$ROOT_DIR/frontend/src"

scan_no_hits "frontend production code must not create Provider Authorization headers" \
  rg -n -i --glob '*.ts' --glob '*.tsx' --glob '!src/test/**' --glob '!**/*.test.*' --glob '!**/*.spec.*' \
  'authorization[^[:cntrl:]]{0,120}(bearer|apiKey|provider)|bearer[^[:cntrl:]]{0,120}(apiKey|provider)' \
  "$ROOT_DIR/frontend/src"

scan_no_hits "frontend production code must not persist sensitive data in browser storage" \
  rg -n -i --glob '*.ts' --glob '*.tsx' --glob '!src/test/**' --glob '!**/*.test.*' --glob '!**/*.spec.*' \
  '((localStorage|sessionStorage|indexedDB)[^[:cntrl:]]{0,160}(api[_ -]?key|provider[^[:cntrl:]]{0,20}key|token|jwt|auth|authorization|cookie|base64|b64_json|object[_ -]?key|objectKey|history|image)|((api[_ -]?key|provider[^[:cntrl:]]{0,20}key|token|jwt|auth|authorization|cookie|base64|b64_json|object[_ -]?key|objectKey|history|image)[^[:cntrl:]]{0,160}(localStorage|sessionStorage|indexedDB)))' \
  "$ROOT_DIR/frontend/src"

scan_no_hits "frontend production code must not poll task status" \
  rg -n -i --glob '*.ts' --glob '*.tsx' --glob '!src/test/**' --glob '!**/*.test.*' --glob '!**/*.spec.*' \
  '(setInterval\s*\(|setTimeout\s*\([^[:cntrl:]]{0,200}(fetch|/api/v1/tasks|events/tasks|status)|while\s*\([^)]*\)\s*\{[^}]*fetch\s*\()' \
  "$ROOT_DIR/frontend/src"

scan_no_hits "backend production logging must not emit sensitive markers directly" \
  rg -n -i --glob '!**/*_test.go' \
  '(slog\.(String|Any|Group)\([^)]*(authorization|cookie|jwt|api[_ -]?key|apikey|base64|b64_json|object[_ -]?key|objectKey|bearer)|log\.(Print|Printf|Println|Fatal|Fatalf)[^[:cntrl:]]{0,200}(authorization|cookie|jwt|api[_ -]?key|apikey|base64|b64_json|object[_ -]?key|objectKey|bearer))' \
  "$ROOT_DIR/backend/internal" "$ROOT_DIR/backend/cmd"

scan_no_hits "deploy config must not define frontend AI Provider proxy or relay targets" \
  rg -n -i \
  '(api\.openai\.com|generativelanguage\.googleapis\.com|/v1/images/generations|/v1beta/models|proxy_pass\s+https?://[^;]*(openai|googleapis|gemini|relay))' \
  "$ROOT_DIR/frontend/nginx.conf" "$ROOT_DIR/deploy/docker-compose.yml"

if ! rg -q 'location /api/' "$ROOT_DIR/frontend/nginx.conf"; then
  echo "[fail] frontend nginx config has no /api/ location" >&2
  exit 1
fi
if ! rg -q 'proxy_pass http://backend-api:8080;' "$ROOT_DIR/frontend/nginx.conf"; then
  echo "[fail] frontend nginx /api/ proxy is not mapped to backend-api:8080" >&2
  exit 1
fi

show_allowed_marker_files

run_quiet docker compose -f "$ROOT_DIR/deploy/docker-compose.yml" config
run_quiet git -C "$ROOT_DIR" diff --check main...HEAD

echo
echo "[ok] P15 focused security regression completed"
