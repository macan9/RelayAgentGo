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

REMOTE_PORT="${REMOTE_PORT:-22}"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:-}"

if [[ -z "$REMOTE_HOST" ]]; then
  echo "[ERR] REMOTE_HOST is required"
  exit 1
fi

TARGET="$REMOTE_USER@$REMOTE_HOST"

ssh -p "$REMOTE_PORT" -o BatchMode=yes "$TARGET" "bash -s" <<'EOF'
set -euo pipefail
systemctl enable relay-agent >/dev/null 2>&1 || true
systemctl restart relay-agent
sleep 2
systemctl --no-pager --full status relay-agent
EOF

echo "[OK] remote service restarted"
