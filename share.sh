#!/usr/bin/env bash
# Cross-compile yzr for every OS/arch in the matrix and share each build with kyoyu.
# Usage: ./share-all.sh <version>
set -euo pipefail

if [ $# -lt 1 ] || [ -z "${1:-}" ]; then
    echo "Usage: $0 <version>" >&2
    exit 1
fi

VERSION="$1"

# OS/ARCH matrix -- edit to add or remove targets.
MATRIX=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
)

OUT_DIR="dist"
mkdir -p "$OUT_DIR"

FAILED=()

for target in "${MATRIX[@]}"; do
    GOOS="${target%/*}"
    GOARCH="${target#*/}"

    BIN="${OUT_DIR}/yzr"
    if [ "$GOOS" = "windows" ]; then
        BIN="${BIN}.exe"
    fi

    echo "==> ${GOOS}/${GOARCH}: building ${BIN}"
    if ! GOOS="$GOOS" GOARCH="$GOARCH" go build -o "$BIN"; then
        echo "!! build failed for ${GOOS}/${GOARCH}" >&2
        FAILED+=("$target")
        continue
    fi

    echo "==> ${GOOS}/${GOARCH}: sharing yozora ${VERSION} with kyoyu"
    if ! kyoyu -bin "$BIN" -name yozora -verbose -version "$VERSION" -y; then
        echo "!! share failed for ${GOOS}/${GOARCH}" >&2
        FAILED+=("$target")
    fi
done

echo
if [ ${#FAILED[@]} -gt 0 ]; then
    echo "Failed targets: ${FAILED[*]}" >&2
    exit 1
fi
echo "All ${#MATRIX[@]} targets built and shared."












































