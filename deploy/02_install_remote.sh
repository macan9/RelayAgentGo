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
REMOTE_PORT="${REMOTE_PORT//$'\r'/}"
REMOTE_USER="${REMOTE_USER//$'\r'/}"
REMOTE_HOST="${REMOTE_HOST//$'\r'/}"
REMOTE_RELEASES_DIR="${REMOTE_RELEASES_DIR//$'\r'/}"
REMOTE_CURRENT_LINK="${REMOTE_CURRENT_LINK//$'\r'/}"
REMOTE_CONFIG_DIR="${REMOTE_CONFIG_DIR//$'\r'/}"
REMOTE_ENV_FILE="${REMOTE_ENV_FILE//$'\r'/}"
REMOTE_STATE_DIR="${REMOTE_STATE_DIR//$'\r'/}"
REMOTE_LOG_DIR="${REMOTE_LOG_DIR//$'\r'/}"

APP_NAME="${APP_NAME:-relay-agent}"
VERSION="${VERSION:-dev}"
REMOTE_PORT="${REMOTE_PORT:-22}"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:-}"
REMOTE_RELEASES_DIR="${REMOTE_RELEASES_DIR:-/opt/relay-agent-go/releases}"
REMOTE_CURRENT_LINK="${REMOTE_CURRENT_LINK:-/opt/relay-agent-go/current}"
REMOTE_CONFIG_DIR="${REMOTE_CONFIG_DIR:-/etc/relay-agent}"
REMOTE_ENV_FILE="${REMOTE_ENV_FILE:-/etc/relay-agent/relay-agent.env}"
REMOTE_STATE_DIR="${REMOTE_STATE_DIR:-/var/lib/relay-agent}"
REMOTE_LOG_DIR="${REMOTE_LOG_DIR:-/var/log/relay-agent}"

if [[ -z "$REMOTE_HOST" ]]; then
  echo "[ERR] REMOTE_HOST is required"
  exit 1
fi

TARGET="$REMOTE_USER@$REMOTE_HOST"
REMOTE_RELEASE="$REMOTE_RELEASES_DIR/$VERSION"

echo "[INFO] remote install on $TARGET"
ssh -p "$REMOTE_PORT" -o BatchMode=yes "$TARGET" "bash -s" <<EOF
set -euo pipefail

APP_NAME="$APP_NAME"
REMOTE_RELEASE="$REMOTE_RELEASE"
REMOTE_CURRENT_LINK="$REMOTE_CURRENT_LINK"
REMOTE_CONFIG_DIR="$REMOTE_CONFIG_DIR"
REMOTE_ENV_FILE="$REMOTE_ENV_FILE"
REMOTE_STATE_DIR="$REMOTE_STATE_DIR"
REMOTE_LOG_DIR="$REMOTE_LOG_DIR"

if [[ ! -f "\$REMOTE_RELEASE/\$APP_NAME" ]]; then
  echo "[ERR] release binary missing: \$REMOTE_RELEASE/\$APP_NAME"
  exit 1
fi

if ! command -v zerotier-cli >/dev/null 2>&1; then
  echo "[WARN] zerotier-one seems missing (zerotier-cli not found)"
fi

install_if_missing() {
  local cmd="\$1"
  local pkg="\$2"
  if command -v "\$cmd" >/dev/null 2>&1; then
    echo "[OK] \$cmd exists"
  else
    echo "[INFO] installing \$pkg"
    apt-get update -y
    DEBIAN_FRONTEND=noninteractive apt-get install -y "\$pkg"
  fi
}

install_if_missing ip iproute2
install_if_missing nft nftables

mkdir -p "\$REMOTE_CONFIG_DIR" "\$REMOTE_STATE_DIR" "\$REMOTE_LOG_DIR"

if [[ ! -f "\$REMOTE_ENV_FILE" ]]; then
  cp "\$REMOTE_RELEASE/relay-agent.env.example" "\$REMOTE_ENV_FILE"
  chmod 600 "\$REMOTE_ENV_FILE"
  echo "[WARN] created default env: \$REMOTE_ENV_FILE (please edit required values)"
fi

for key in CONTROLLER_BASE_URL CONTROLLER_TOKEN RELAY_NAME; do
  if ! grep -qE "^\\s*\$key=" "\$REMOTE_ENV_FILE"; then
    echo "[ERR] missing required key in env: \$key"
    exit 1
  fi
done

ln -sfn "\$REMOTE_RELEASE" "\$REMOTE_CURRENT_LINK"
chmod 755 "\$REMOTE_CURRENT_LINK/\$APP_NAME"

cat >/etc/systemd/system/relay-agent.service <<UNIT
[Unit]
Description=RelayAgentGo
After=network-online.target zerotier-one.service
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
EnvironmentFile=-\$REMOTE_ENV_FILE
ExecStart=\$REMOTE_CURRENT_LINK/\$APP_NAME
Restart=always
RestartSec=5s
StateDirectory=relay-agent
LogsDirectory=relay-agent

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
echo "[OK] remote install complete"
EOF
