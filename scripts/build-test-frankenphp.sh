#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd "$SCRIPT_DIR/.." && pwd)
OUTPUT="$REPO_ROOT/module/frankenphp"
TMP_BINARY=$(mktemp "${TMPDIR:-/tmp}/queue-frankenphp.XXXXXX")

cleanup() {
    rm -f "$TMP_BINARY"
}
trap cleanup EXIT HUP INT TERM

cd "$REPO_ROOT"

export XCADDY_GO_BUILD_FLAGS="-buildvcs=false -ldflags='-w -s' -tags=nobadger,nomysql,nopgx,nowatcher"

xcaddy build \
    --output "$TMP_BINARY" \
    --with github.com/dunglas/frankenphp@v1.12.3 \
    --with github.com/dunglas/frankenphp/caddy@v1.12.3 \
    --with github.com/dunglas/caddy-cbrotli@v1.0.1 \
    --with github.com/y-l-g/queue/module=./module

cp "$TMP_BINARY" "$OUTPUT"
chmod 755 "$OUTPUT"
