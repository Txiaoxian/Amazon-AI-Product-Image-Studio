#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_FILE="${TLS_REVERSE_PROXY_CONFIG:-$ROOT_DIR/deploy/nginx/amazon-ai-product-image-studio.conf.template}"

usage() {
  cat <<'EOF'
Usage: bash scripts/tls-reverse-proxy-check.sh [--config <path>] [--help]

Statically validates the host-level production TLS reverse proxy template.
The check does not read certificate files, start Nginx, or contact any service.

Options:
  --config <path>  Validate an alternate Nginx configuration file.
  --help           Show this help text.

Environment:
  TLS_REVERSE_PROXY_CONFIG  Override the Nginx configuration path.
EOF
}

fail() {
  echo "[fail] $*" >&2
  exit 1
}

require_pattern() {
  local pattern="$1"
  local message="$2"
  if ! grep -Eq -- "$pattern" "$CONFIG_FILE"; then
    fail "$message"
  fi
}

require_block_pattern() {
  local block="$1"
  local pattern="$2"
  local message="$3"
  if ! grep -Eq -- "$pattern" <<<"$block"; then
    fail "$message"
  fi
}

extract_location() {
  local target="$1"
  awk -v target="$target" '
    $0 ~ "^[[:space:]]*location[[:space:]]+" target "[[:space:]]*\\{" {
      found = 1
    }
    found {
      print
      line = $0
      opens = gsub(/\{/, "", line)
      line = $0
      closes = gsub(/\}/, "", line)
      depth += opens - closes
      if (depth == 0) {
        exit
      }
    }
    END {
      if (!found) {
        exit 1
      }
    }
  ' "$CONFIG_FILE"
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --config)
      if [[ "$#" -lt 2 ]]; then
        usage >&2
        exit 2
      fi
      CONFIG_FILE="$2"
      shift
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

if [[ ! -f "$CONFIG_FILE" ]]; then
  fail "TLS reverse proxy config not found: $CONFIG_FILE"
fi

echo "==> host TLS reverse proxy static checks"

if grep -Ein '^[[:space:]]*(proxy_pass|server)[[:space:]]+[^;]*(openai|gemini|googleapis|relay)' "$CONFIG_FILE"; then
  fail "forbidden AI Provider or relay proxy target"
fi

unexpected_upstreams="$(
  awk '
    /^[[:space:]]*server[[:space:]]+[^{}]+;[[:space:]]*$/ &&
      $0 !~ /^[[:space:]]*server[[:space:]]+127\.0\.0\.1:8080;[[:space:]]*$/ {
        print NR ":" $0
      }
  ' "$CONFIG_FILE"
)"
if [[ -n "$unexpected_upstreams" ]]; then
  printf '%s\n' "$unexpected_upstreams" >&2
  fail "unexpected upstream server target; host proxy must use loopback frontend 127.0.0.1:8080"
fi

unexpected_proxy_passes="$(
  awk '
    /^[[:space:]]*proxy_pass[[:space:]]+/ &&
      $0 !~ /^[[:space:]]*proxy_pass[[:space:]]+http:\/\/frontend_loopback;[[:space:]]*$/ {
        print NR ":" $0
      }
  ' "$CONFIG_FILE"
)"
if [[ -n "$unexpected_proxy_passes" ]]; then
  printf '%s\n' "$unexpected_proxy_passes" >&2
  fail "unexpected proxy_pass target; host proxy must route only through frontend_loopback"
fi

require_pattern '^[[:space:]]*upstream[[:space:]]+frontend_loopback[[:space:]]*\{' \
  "missing frontend_loopback upstream"
require_pattern '^[[:space:]]*server[[:space:]]+127\.0\.0\.1:8080;[[:space:]]*$' \
  "frontend upstream is not loopback 127.0.0.1:8080"
require_pattern '^[[:space:]]*listen[[:space:]]+80;[[:space:]]*$' \
  "missing HTTP listen for HTTPS redirect"
require_pattern '^[[:space:]]*return[[:space:]]+301[[:space:]]+https://\$host\$request_uri;[[:space:]]*$' \
  "missing HTTP to HTTPS redirect"
require_pattern '^[[:space:]]*listen[[:space:]]+443[[:space:]]+ssl([[:space:]]|;)' \
  "missing HTTPS TLS listen"
require_pattern '^[[:space:]]*ssl_certificate[[:space:]]+[^;]+;' \
  "missing TLS certificate placeholder path"
require_pattern '^[[:space:]]*ssl_certificate_key[[:space:]]+[^;]+;' \
  "missing TLS private-key placeholder path"
require_pattern '^[[:space:]]*add_header[[:space:]]+Strict-Transport-Security[[:space:]]+' \
  "missing HSTS header"
require_pattern '^[[:space:]]*add_header[[:space:]]+X-Content-Type-Options[[:space:]]+"nosniff"[[:space:]]+always;' \
  "missing X-Content-Type-Options header"
require_pattern '^[[:space:]]*add_header[[:space:]]+X-Frame-Options[[:space:]]+"DENY"[[:space:]]+always;' \
  "missing X-Frame-Options header"
require_pattern '^[[:space:]]*add_header[[:space:]]+Referrer-Policy[[:space:]]+"strict-origin-when-cross-origin"[[:space:]]+always;' \
  "missing Referrer-Policy header"

if ! sse_location="$(extract_location '/api/v1/events/')"; then
  fail "missing SSE /api/v1/events/ location"
fi
if ! api_location="$(extract_location '/api/')"; then
  fail "missing /api/ location"
fi
if ! root_location="$(extract_location '/')"; then
  fail "missing / location"
fi

for location_name in "SSE /api/v1/events/" "/api/" "/"; do
  case "$location_name" in
    "SSE /api/v1/events/")
      location_block="$sse_location"
      ;;
    "/api/")
      location_block="$api_location"
      ;;
    "/")
      location_block="$root_location"
      ;;
  esac
  require_block_pattern "$location_block" 'proxy_set_header[[:space:]]+Host[[:space:]]+\$host;' \
    "$location_name location does not forward Host"
  require_block_pattern "$location_block" 'proxy_set_header[[:space:]]+X-Real-IP[[:space:]]+\$remote_addr;' \
    "$location_name location does not forward X-Real-IP"
  require_block_pattern "$location_block" 'proxy_set_header[[:space:]]+X-Forwarded-For[[:space:]]+\$proxy_add_x_forwarded_for;' \
    "$location_name location does not forward X-Forwarded-For"
  require_block_pattern "$location_block" 'proxy_set_header[[:space:]]+X-Forwarded-Proto[[:space:]]+https;' \
    "$location_name location does not force X-Forwarded-Proto=https"
  require_block_pattern "$location_block" 'proxy_pass[[:space:]]+http://frontend_loopback;' \
    "$location_name location does not proxy frontend_loopback"
done

require_block_pattern "$sse_location" 'proxy_http_version[[:space:]]+1\.1;' \
  "SSE location does not use HTTP/1.1"
require_block_pattern "$sse_location" 'proxy_set_header[[:space:]]+Connection[[:space:]]+"";' \
  'SSE location does not clear Connection'
require_block_pattern "$sse_location" 'proxy_buffering[[:space:]]+off;' \
  "SSE location does not disable proxy buffering"
require_block_pattern "$sse_location" 'proxy_cache[[:space:]]+off;' \
  "SSE location does not disable proxy cache"
require_block_pattern "$sse_location" 'proxy_read_timeout[[:space:]]+1h;' \
  "SSE location does not set a long read timeout"
require_block_pattern "$sse_location" 'proxy_send_timeout[[:space:]]+1h;' \
  "SSE location does not set a long send timeout"
require_block_pattern "$sse_location" 'add_header[[:space:]]+X-Accel-Buffering[[:space:]]+"?no"?[[:space:]]+always;' \
  "SSE location does not set X-Accel-Buffering no"

echo "[ok] host TLS reverse proxy config is safe for static validation"
