#!/bin/bash
# Post-deploy smoke test. Copied to each app server and run there by
# .github/scripts/deploy-host.sh right after /opt/levelog/deploy.sh's
# /health/ready check passes. /health/ready only proves the api container
# can reach the DB — the phase 17 rollback drill (docs/load-test-results.md
# section 3) showed a build whose login handler always returned 500 sails
# straight through it. This exercises the real request paths through
# Nginx -> api -> DB instead, without writing any data (no registration,
# no successful login), so it is safe to run against production.
#
# Usage: smoke-test.sh [BASE_HTTPS_URL] [BASE_HTTP_URL]
#   defaults: https://localhost  http://localhost
#
# -k is deliberate: this checks the app behind this one server's Nginx,
# reached as "localhost", which never matches the certificate's name.
# Certificate validity is checked separately (the certificate expiry
# check at the end, and the external simple_monitor).
set -euo pipefail

https_base="${1:-https://localhost}"
http_base="${2:-http://localhost}"

failures=0
body_file="$(mktemp)"
trap 'rm -f "$body_file"' EXIT

# check NAME EXPECTED_STATUS CURL_ARGS...
# Prints the result; on mismatch also prints the start of the body.
check() {
  local name="$1" expected="$2"
  shift 2
  local status
  # On a connection failure curl still prints "000" via -w and exits
  # non-zero; `|| true` keeps that single "000" instead of appending another.
  status="$(curl -ksS -o "$body_file" -w '%{http_code}' --max-time 10 "$@" || true)"
  if [ "$status" = "$expected" ]; then
    echo "ok   $name ($status)"
    return 0
  fi
  echo "FAIL $name: expected $expected, got $status" >&2
  head -c 300 "$body_file" >&2 || true
  echo >&2
  failures=$((failures + 1))
  return 1
}

# body_contains NAME NEEDLE — asserts on the body of the preceding check.
body_contains() {
  if grep -qF -- "$2" "$body_file"; then
    echo "ok   $1"
  else
    echo "FAIL $1: response body does not contain '$2'" >&2
    failures=$((failures + 1))
  fi
}

# header_present NAME HEADER URL
header_present() {
  if curl -ksS -o /dev/null -D - --max-time 10 "$3" | grep -qi "^$2:"; then
    echo "ok   $1"
  else
    echo "FAIL $1: header $2 missing" >&2
    failures=$((failures + 1))
  fi
}

echo "==> Smoke testing $https_base"

# Plain HTTP must redirect to HTTPS (except /health/, for the LB check).
check "HTTP -> HTTPS redirect" 301 "$http_base/" || true
check "HTTP /health/live stays unredirected" 200 "$http_base/health/live" || true

check "health ready" 200 "$https_base/health/ready" || true

# The SPA shell itself: Nginx serves the built index.html.
if check "index.html" 200 "$https_base/"; then
  body_contains "index.html has the app root" 'id="root"'
fi
header_present "CSP header on /" Content-Security-Policy "$https_base/"
header_present "HSTS header on /" Strict-Transport-Security "$https_base/"

# Unauthenticated API access must be a clean 401 (auth middleware +
# session lookup against the DB), not a 5xx.
check "GET /api/me without a session" 401 "$https_base/api/me" || true

# Login with credentials that cannot exist: goes through the rate limiter,
# JSON decoding, the users table lookup and error mapping — exactly the
# path the phase 17 drill broke. The address uses the reserved .invalid
# TLD (RFC 2606), so it can never match a real account.
if check "POST /api/auth/login with unknown user" 401 \
  -X POST -H 'Content-Type: application/json' \
  --data '{"email":"smoke-test@levelog.invalid","password":"not-a-real-password"}' \
  "$https_base/api/auth/login"; then
  body_contains "login error is the API's JSON error shape" '"UNAUTHORIZED"'
fi

# Unknown API route: 404 from the Go mux, not the SPA fallback.
check "unknown API route" 404 "$https_base/api/does-not-exist" || true

# Certificate expiry: warn well before it bites. Renewal is automated
# (certbot.timer + the deploy hook, see docs/tls-design.md), so a cert this
# close to expiry means renewal is silently failing. 14 days, not 0: this
# must fail while there is still time to fix it by hand.
host_port="${https_base#https://}"
host_port="${host_port%%/*}"
case "$host_port" in *:*) ;; *) host_port="$host_port:443" ;; esac
if cert="$(echo | openssl s_client -connect "$host_port" -servername "${host_port%%:*}" 2>/dev/null | openssl x509 2>/dev/null)"; then
  if echo "$cert" | openssl x509 -noout -checkend $((14 * 24 * 3600)) >/dev/null; then
    echo "ok   certificate valid for at least 14 more days ($(echo "$cert" | openssl x509 -noout -enddate))"
  else
    echo "FAIL certificate expires within 14 days ($(echo "$cert" | openssl x509 -noout -enddate))" >&2
    failures=$((failures + 1))
  fi
else
  echo "FAIL could not read the served certificate from $host_port" >&2
  failures=$((failures + 1))
fi

if [ "$failures" -gt 0 ]; then
  echo "==> Smoke test FAILED ($failures check(s))" >&2
  exit 1
fi
echo "==> Smoke test passed"
