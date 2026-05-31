#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/tls-reverse-proxy-check.sh"
TEMPLATE="$ROOT_DIR/deploy/nginx/amazon-ai-product-image-studio.conf.template"
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

run_fixture() {
  local name="$1"
  local fixture="$2"
  local output
  output="$(mktemp)"

  set +e
  TLS_REVERSE_PROXY_CONFIG="$fixture" "$REAL_BASH" "$SCRIPT" >"$output" 2>&1
  local status=$?
  set -e

  CASE_OUTPUT="$output"
  CASE_STATUS="$status"
  echo "[ok] ran $name"
}

copy_template() {
  local tmpdir
  tmpdir="$(mktemp -d)"
  local fixture="$tmpdir/nginx.conf"
  cp "$TEMPLATE" "$fixture"
  printf '%s\n' "$fixture"
}

test_production_template_passes() {
  run_fixture "production template passes" "$TEMPLATE"
  assert_status 0 "$CASE_STATUS" "production template"
}

test_missing_tls_listen_fails() {
  local fixture
  fixture="$(copy_template)"
  sed -i.bak '/listen 443 ssl/d' "$fixture"

  run_fixture "missing TLS listen fails" "$fixture"
  assert_nonzero_status "$CASE_STATUS" "missing TLS listen"
  assert_contains "$CASE_OUTPUT" "missing HTTPS TLS listen"
}

test_missing_hsts_fails() {
  local fixture
  fixture="$(copy_template)"
  sed -i.bak '/Strict-Transport-Security/d' "$fixture"

  run_fixture "missing HSTS fails" "$fixture"
  assert_nonzero_status "$CASE_STATUS" "missing HSTS"
  assert_contains "$CASE_OUTPUT" "missing HSTS header"
}

test_sse_buffering_not_disabled_fails() {
  local fixture
  fixture="$(copy_template)"
  sed -i.bak '/proxy_buffering off;/d' "$fixture"

  run_fixture "SSE buffering not disabled fails" "$fixture"
  assert_nonzero_status "$CASE_STATUS" "SSE buffering enabled"
  assert_contains "$CASE_OUTPUT" "SSE location does not disable proxy buffering"
}

test_non_loopback_frontend_upstream_fails() {
  local fixture
  fixture="$(copy_template)"
  sed -i.bak 's/server 127\.0\.0\.1:8080;/server frontend:80;/' "$fixture"

  run_fixture "non-loopback frontend upstream fails" "$fixture"
  assert_nonzero_status "$CASE_STATUS" "non-loopback frontend upstream"
  assert_contains "$CASE_OUTPUT" "unexpected upstream server target"
}

test_direct_backend_api_proxy_fails() {
  local fixture
  fixture="$(copy_template)"
  sed -i.bak 's#proxy_pass http://frontend_loopback;#proxy_pass http://127.0.0.1:8081;#g' "$fixture"

  run_fixture "direct backend-api proxy fails" "$fixture"
  assert_nonzero_status "$CASE_STATUS" "direct backend-api proxy"
  assert_contains "$CASE_OUTPUT" "unexpected proxy_pass target"
}

test_openai_proxy_fails() {
  local fixture
  fixture="$(copy_template)"
  sed -i.bak 's#proxy_pass http://frontend_loopback;#proxy_pass https://api.openai.com;#g' "$fixture"

  run_fixture "OpenAI proxy fails" "$fixture"
  assert_nonzero_status "$CASE_STATUS" "OpenAI proxy"
  assert_contains "$CASE_OUTPUT" "forbidden AI Provider or relay proxy target"
}

test_relay_proxy_fails() {
  local fixture
  fixture="$(copy_template)"
  sed -i.bak 's#proxy_pass http://frontend_loopback;#proxy_pass http://relay:3000;#g' "$fixture"

  run_fixture "relay proxy fails" "$fixture"
  assert_nonzero_status "$CASE_STATUS" "relay proxy"
  assert_contains "$CASE_OUTPUT" "forbidden AI Provider or relay proxy target"
}

test_production_template_passes
test_missing_tls_listen_fails
test_missing_hsts_fails
test_sse_buffering_not_disabled_fails
test_non_loopback_frontend_upstream_fails
test_direct_backend_api_proxy_fails
test_openai_proxy_fails
test_relay_proxy_fails

echo
echo "[ok] TLS reverse proxy static check tests passed"
