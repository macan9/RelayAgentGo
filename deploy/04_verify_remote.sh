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
REMOTE_PORT="${REMOTE_PORT//$'\r'/}"
REMOTE_USER="${REMOTE_USER//$'\r'/}"
REMOTE_HOST="${REMOTE_HOST//$'\r'/}"
CONTROLLER_BASE_URL="${CONTROLLER_BASE_URL//$'\r'/}"
CONTROLLER_TOKEN="${CONTROLLER_TOKEN//$'\r'/}"
RELAY_NODE_ID="${RELAY_NODE_ID//$'\r'/}"

REMOTE_PORT="${REMOTE_PORT:-22}"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:-}"

if [[ -z "$REMOTE_HOST" ]]; then
  echo "[ERR] REMOTE_HOST is required"
  exit 1
fi

TARGET="$REMOTE_USER@$REMOTE_HOST"

echo "[INFO] checking remote service and logs"
ssh -p "$REMOTE_PORT" -o BatchMode=yes "$TARGET" "bash -s" <<'EOF'
set -euo pipefail
systemctl is-active relay-agent
journalctl -u relay-agent -n 80 --no-pager
EOF

if [[ "${CONTROLLER_TOKEN:-}" == "change-me" ]]; then
  echo "[WARN] skip controller api verification (CONTROLLER_TOKEN is placeholder: change-me)"
elif [[ -n "${CONTROLLER_BASE_URL:-}" && -n "${CONTROLLER_TOKEN:-}" && -n "${RELAY_NODE_ID:-}" ]]; then
  echo "[INFO] checking relay record from controller api"
  curl -fsS \
    -H "Authorization: Bearer $CONTROLLER_TOKEN" \
    "$CONTROLLER_BASE_URL/api/relays/$RELAY_NODE_ID" | sed 's/.*/[API] &/'
else
  echo "[WARN] skip controller api verification (CONTROLLER_BASE_URL/CONTROLLER_TOKEN/RELAY_NODE_ID not set)"
fi

echo "[OK] verify step complete"
