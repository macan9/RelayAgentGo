#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_ENV="${DEPLOY_ENV:-$ROOT_DIR/deploy/deploy.env}"

if [[ ! -f "$DEPLOY_ENV" ]]; then
  echo "[ERR] missing deploy env: $DEPLOY_ENV"
  echo "      copy deploy/deploy.env.example to deploy/deploy.env first"
  exit 1
fi

# shellcheck disable=SC1090
source "$DEPLOY_ENV"

APP_NAME="${APP_NAME:-relay-agent}"
VERSION="${VERSION:-dev}"
BUILD_DIR="${BUILD_DIR:-./.deploy-dist}"
BUILD_DIR_ABS="$ROOT_DIR/${BUILD_DIR#./}"
RELEASE_DIR="$BUILD_DIR_ABS/$VERSION"
ROOT_DIR_WIN=""
RELEASE_DIR_WIN=""
GO_CMD_WIN=""

echo "[INFO] root: $ROOT_DIR"
echo "[INFO] build dir: $BUILD_DIR_ABS"
echo "[INFO] version: $VERSION"

GO_CMD="go"
if ! command -v "$GO_CMD" >/dev/null 2>&1; then
  if [[ -x "/mnt/d/GoENV/bin/go.exe" ]]; then
    GO_CMD="/mnt/d/GoENV/bin/go.exe"
  elif [[ -x "/mnt/c/Go/bin/go.exe" ]]; then
    GO_CMD="/mnt/c/Go/bin/go.exe"
  else
    echo "[ERR] go not found"
    exit 1
  fi
fi

if [[ "$GO_CMD" == *.exe ]] && command -v wslpath >/dev/null 2>&1; then
  ROOT_DIR_WIN="$(wslpath -w "$ROOT_DIR")"
  RELEASE_DIR_WIN="$(wslpath -w "$RELEASE_DIR")"
  GO_CMD_WIN="$(wslpath -w "$GO_CMD")"
fi

run_go() {
  local subcmd="$1"
  shift
  if [[ "$GO_CMD" == *.exe ]] && command -v powershell.exe >/dev/null 2>&1; then
    local ps_args=()
    for arg in "$@"; do
      arg="${arg//\'/''}"
      ps_args+=("'$arg'")
    done
    local args_joined
    args_joined="$(IFS=,; echo "${ps_args[*]}")"
    powershell.exe -NoProfile -Command "\$env:CGO_ENABLED='0'; \$env:GOOS='linux'; \$env:GOARCH='amd64'; Set-Location -LiteralPath '$ROOT_DIR_WIN'; & '$GO_CMD_WIN' '$subcmd' @($args_joined)"
  else
    "$GO_CMD" "$subcmd" "$@"
  fi
}

rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

echo "[INFO] running tests"
if [[ "$GO_CMD" == *.exe ]] && command -v powershell.exe >/dev/null 2>&1; then
  powershell.exe -NoProfile -Command "Set-Location -LiteralPath '$ROOT_DIR_WIN'; & '$GO_CMD_WIN' test ./..."
else
  "$GO_CMD" test ./...
fi

echo "[INFO] building linux binary"
(
  cd "$ROOT_DIR"
  if [[ "$GO_CMD" == *.exe ]]; then
    run_go build -trimpath -ldflags "-s -w" -o "$RELEASE_DIR_WIN\\$APP_NAME" ./cmd/relay-agent
  else
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$GO_CMD" build -trimpath -ldflags "-s -w" -o "$RELEASE_DIR/$APP_NAME" ./cmd/relay-agent
  fi
)

cp "$ROOT_DIR/deploy/relay-agent.service" "$RELEASE_DIR/relay-agent.service"
cp "$ROOT_DIR/deploy/relay-agent.env.example" "$RELEASE_DIR/relay-agent.env.example"

echo "$VERSION" > "$RELEASE_DIR/VERSION"

if [[ ! -f "$RELEASE_DIR/$APP_NAME" ]]; then
  echo "[ERR] build finished but binary is missing: $RELEASE_DIR/$APP_NAME"
  exit 1
fi

echo "[OK] local build complete: $RELEASE_DIR"
