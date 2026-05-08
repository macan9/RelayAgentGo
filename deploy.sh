#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ENV="${DEPLOY_ENV:-$ROOT_DIR/deploy/deploy.env}"

usage() {
  cat <<'EOF'
One-click deploy for RelayAgentGo

Usage:
  ./deploy.sh
  ./deploy.sh --skip-build

Options:
  --skip-build   Skip local build (deploy/00_build_local.sh)
EOF
}

SKIP_BUILD=false

for arg in "$@"; do
  case "$arg" in
    --skip-build) SKIP_BUILD=true ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "[ERR] unknown arg: $arg"
      usage
      exit 1
      ;;
  esac
done

if [[ ! -f "$DEPLOY_ENV" ]]; then
  echo "[ERR] missing deploy env: $DEPLOY_ENV"
  echo "      copy deploy/deploy.env.example to deploy/deploy.env first"
  exit 1
fi

if ! $SKIP_BUILD; then
  echo "[STEP] 00_build_local.sh"
  bash "$ROOT_DIR/deploy/00_build_local.sh"
else
  echo "[STEP] skip local build"
fi

echo "[STEP] 01_upload_release.sh"
bash "$ROOT_DIR/deploy/01_upload_release.sh"

echo "[STEP] 02_install_remote.sh"
bash "$ROOT_DIR/deploy/02_install_remote.sh"

echo "[STEP] 03_start_remote.sh"
bash "$ROOT_DIR/deploy/03_start_remote.sh"

echo "[STEP] 04_verify_remote.sh"
bash "$ROOT_DIR/deploy/04_verify_remote.sh"

echo "[OK] one-click deploy complete"
