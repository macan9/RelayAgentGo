#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_ENV="${DEPLOY_ENV:-$ROOT_DIR/deploy/deploy.env}"

if [[ ! -f "$DEPLOY_ENV" ]]; then
  echo "[ERR] missing deploy env: $DEPLOY_ENV"
  exit 1
fi

# shellcheck disable=SC1090
source "$DEPLOY_ENV"
APP_NAME="${APP_NAME//$'\r'/}"
VERSION="${VERSION//$'\r'/}"
BUILD_DIR="${BUILD_DIR//$'\r'/}"
REMOTE_PORT="${REMOTE_PORT//$'\r'/}"
REMOTE_USER="${REMOTE_USER//$'\r'/}"
REMOTE_HOST="${REMOTE_HOST//$'\r'/}"
REMOTE_RELEASES_DIR="${REMOTE_RELEASES_DIR//$'\r'/}"

APP_NAME="${APP_NAME:-relay-agent}"
VERSION="${VERSION:-dev}"
BUILD_DIR="${BUILD_DIR:-./.deploy-dist}"
REMOTE_PORT="${REMOTE_PORT:-22}"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:-}"
REMOTE_RELEASES_DIR="${REMOTE_RELEASES_DIR:-/opt/relay-agent-go/releases}"

if [[ -z "$REMOTE_HOST" ]]; then
  echo "[ERR] REMOTE_HOST is required"
  exit 1
fi

RELEASE_DIR="$ROOT_DIR/${BUILD_DIR#./}/$VERSION"
if [[ ! -f "$RELEASE_DIR/$APP_NAME" ]]; then
  echo "[ERR] missing binary: $RELEASE_DIR/$APP_NAME"
  echo "      run deploy/00_build_local.sh first"
  exit 1
fi

command -v rsync >/dev/null 2>&1 || { echo "[ERR] rsync not found"; exit 1; }
command -v ssh >/dev/null 2>&1 || { echo "[ERR] ssh not found"; exit 1; }

TARGET="$REMOTE_USER@$REMOTE_HOST"

echo "[INFO] creating remote release dir"
ssh -p "$REMOTE_PORT" -o BatchMode=yes "$TARGET" "mkdir -p '$REMOTE_RELEASES_DIR/$VERSION'"

echo "[INFO] uploading release by rsync"
rsync -avz --delete -e "ssh -p $REMOTE_PORT" \
  "$RELEASE_DIR/" "$TARGET:$REMOTE_RELEASES_DIR/$VERSION/"

echo "[OK] upload complete: $TARGET:$REMOTE_RELEASES_DIR/$VERSION"
