#!/usr/bin/env bash
# scripts/build_update.sh — Build the Hero and Telegram daemon binaries for the
# current target.
#
# Unlike build_dev.sh, this script does not cross-compile, remove dist/, install
# files, or update a plugin manifest. It creates the two host-targeted
# executables needed by the development updater.
#
# Usage:
#   ./scripts/build_update.sh
#   HERO_UPDATE_OUTPUT=/tmp/hero.next \
#   HERO_UPDATE_TELEGRAM_DAEMON_OUTPUT=/tmp/hero-telegram-daemon.next \
#   ./scripts/build_update.sh
#
# Environment:
#   GOOS / GOARCH            Optional target override; defaults to go env.
#   HERO_UPDATE_OUTPUT                 Optional exact Hero output path.
#   HERO_UPDATE_TELEGRAM_DAEMON_OUTPUT Optional exact daemon output path.

set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd -- "${ROOT_DIR}"

GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"

case "${GOOS}/${GOARCH}" in
  linux/amd64|linux/arm64|darwin/amd64|darwin/arm64)
    ;;
  *)
    echo "[ERROR] Unsupported Hero update target: ${GOOS}/${GOARCH}" >&2
    echo "        Supported targets: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64" >&2
    exit 1
    ;;
esac

LAST_TAG="$(git tag -l --sort=-creatordate | head -n1 || true)"
COMMIT="$(git rev-parse --short HEAD)"
if [[ -n "${LAST_TAG}" ]]; then
  VERSION="${LAST_TAG#v}_${COMMIT}"
else
  VERSION="dev_${COMMIT}"
fi

OUTPUT="${HERO_UPDATE_OUTPUT:-${ROOT_DIR}/dist/hero_update_${VERSION}_${GOOS}_${GOARCH}}"
DAEMON_OUTPUT="${HERO_UPDATE_TELEGRAM_DAEMON_OUTPUT:-${ROOT_DIR}/dist/hero-telegram-daemon_update_${VERSION}_${GOOS}_${GOARCH}}"
mkdir -p "$(dirname -- "${OUTPUT}")" "$(dirname -- "${DAEMON_OUTPUT}")"

echo "Building Hero update: ${VERSION} (${GOOS}/${GOARCH})"
GOOS="${GOOS}" GOARCH="${GOARCH}" go build \
  -trimpath \
  -ldflags "-X main.version=${VERSION}" \
  -o "${OUTPUT}" \
  ./cmd/hero
chmod +x "${OUTPUT}"

echo "Built ${OUTPUT}"

echo "Building Telegram daemon update: ${VERSION} (${GOOS}/${GOARCH})"
GOOS="${GOOS}" GOARCH="${GOARCH}" go build \
  -trimpath \
  -ldflags "-X main.version=${VERSION}" \
  -o "${DAEMON_OUTPUT}" \
  ./cmd/hero-telegram-daemon
chmod +x "${DAEMON_OUTPUT}"

echo "Built ${DAEMON_OUTPUT}"
