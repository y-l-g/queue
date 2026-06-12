#!/usr/bin/env sh
set -eu

if [ "$#" -eq 0 ]; then
    echo "Usage: scripts/release-checksums.sh <artifact>..." >&2
    exit 2
fi

sha256sum "$@" > SHA256SUMS
echo "Wrote SHA256SUMS"
