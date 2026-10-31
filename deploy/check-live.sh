#!/usr/bin/env bash
#
# Is the server actually running this checkout?
#
# A deploy that copies the binary but not web/dist (or copies both but never
# restarts) looks fine from the outside: nginx keeps serving the old page, and
# every symptom gets blamed on the code that was written since. That happened
# once already — the login page kept rendering a 122x44 cropped captcha for a
# day while the fix sat in the repository.
#
# This script answers the question directly, by comparing the asset filenames
# the server advertises with the ones in the local build. Vite hashes those
# names, so equal names mean equal content.
#
# Usage:  bash deploy/check-live.sh [base_url] [dist_dir]
#         bash deploy/check-live.sh http://47.115.62.125:8088
#
# Exit codes: 0 = in sync, 1 = drift, 2 = could not check.

set -uo pipefail

BASE="${1:-http://47.115.62.125:8088}"
DIST="${2:-web/dist}"
INDEX="$DIST/index.html"

if [ ! -f "$INDEX" ]; then
  echo "  no local build at $INDEX — run: (cd web && npm run build)" >&2
  exit 2
fi

# The machine this runs on may have a proxy configured for outbound traffic.
# The target is usually a LAN/loopback address in dev and a bare IP in prod;
# either way the proxy is not the way to reach it.
CURL=(curl -s --max-time 15 --noproxy '*' -H 'Cache-Control: no-cache')

# Fixed scratch names in the temp directory, overwritten on each run rather than
# deleted: this script is meant to be run from odd shells, and a cleanup trap on
# a freshly made file is one more thing that can fail for no benefit.
SCRATCH="${TMPDIR:-/tmp}"
STATUS_FILE="$SCRATCH/lopezpress-check-live.status"
HTML_FILE="$SCRATCH/lopezpress-check-live.html"
HEALTH_FILE="$SCRATCH/lopezpress-check-live.health"

assets_of() { grep -oE 'assets/[A-Za-z0-9._-]+\.(js|css)' "$1" | sort -u; }

fetch() {
  local url="$1" out="$2"
  if ! "${CURL[@]}" -o "$out" -w '%{http_code}' "$url" > "$STATUS_FILE" 2>/dev/null; then
    echo "  request failed: $url" >&2
    return 1
  fi
  [ "$(cat "$STATUS_FILE")" = "200" ] || {
    echo "  $url answered $(cat "$STATUS_FILE")" >&2
    return 1
  }
}

echo "== local build =="
assets_of "$INDEX" | sed 's/^/  /'

echo "== $BASE =="
if ! fetch "$BASE/login" "$HTML_FILE"; then
  echo "  cannot read the login page; nothing to compare" >&2
  exit 2
fi

REMOTE_ASSETS=$(assets_of "$HTML_FILE")
if [ -z "$REMOTE_ASSETS" ]; then
  echo "  the login page references no hashed assets — is this really the app?" >&2
  exit 2
fi
echo "$REMOTE_ASSETS" | sed 's/^/  /'

echo "== build identity =="
# /health no longer carries the version: it is an unauthenticated probe, and
# answering "which build is this" to anyone who scans the port is exactly the
# fingerprint a self-hosted box should not hand out. That makes this section
# unable to report a version over HTTP, which is fine — the asset hashes above
# are the stronger signal anyway (they change on every frontend build), and the
# binary's own version is one ssh command away:
#     ssh root@HOST '/opt/lopezpress/lopezpress -version'
if fetch "$BASE/health" "$HEALTH_FILE"; then
  tr -d '\n' < "$HEALTH_FILE" | sed 's/^/  /'
  echo
  case "$(tr -d '\n' < "$HEALTH_FILE")" in
    *'"version"'*) : ;;
    *) echo "  (no version field — expected: the probe is anonymous by design)" ;;
  esac
else
  echo "  no /health answer" >&2
fi

echo "== verdict =="
if [ "$(assets_of "$INDEX")" = "$REMOTE_ASSETS" ]; then
  echo "  in sync — the server is serving the build in this checkout"
  exit 0
fi

echo "  DRIFT — the server is not serving this checkout."
echo
echo "  Deploy both halves, then restart:"
echo "    scp bin/lopezpress root@HOST:/opt/lopezpress/lopezpress && ssh root@HOST 'chmod 755 /opt/lopezpress/lopezpress && systemctl restart lopezpress'"
echo "    rsync -a --delete $DIST/ root@HOST:/opt/lopezpress/web/"
exit 1
