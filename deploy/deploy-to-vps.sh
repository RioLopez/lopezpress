#!/usr/bin/env bash
# deploy-to-vps.sh — 本地一键部署到 VPS
# 在开发机（Windows 上在 Git Bash 或 PowerShell 的 bash 里跑；Linux 直接 bash）执行。
# 用法：
#   VPS_USER=root VPS_HOST=1.2.3.4 bash deploy/deploy-to-vps.sh
# 可选：VPS_SSH_PORT=22、APP_DIR=/opt/lopezpress
# 首次部署会在 VPS 上自动创建 systemd 服务并启用（见 install-systemd.sh）。

set -euo pipefail

VPS_USER="${VPS_USER:-root}"
VPS_HOST="${VPS_HOST:?必须设置 VPS_HOST（VPS IP 或域名）}"
VPS_SSH_PORT="${VPS_SSH_PORT:-22}"
APP_DIR="${APP_DIR:-/opt/lopezpress}"
SSH_TARGET="${VPS_USER}@${VPS_HOST}"
SSH_ARGS=(-p "$VPS_SSH_PORT")

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "==> 交叉编译 Linux amd64 静态二进制（CGO_ENABLED=0）..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w" -o bin/lopezpress ./cmd/server
echo "    产物: bin/lopezpress ($(du -h bin/lopezpress | cut -f1))"

echo "==> 构建前端 (web/)..."
( cd web
  if [ ! -d node_modules ]; then
    echo "    安装前端依赖 (npm install)..."
    npm install
  fi
  npm run build
)
echo "    前端产物: web/dist/index.html"

echo "==> 创建/生成部署用 .env（VPS 侧 ${APP_DIR}/.env）..."
# 仅当本地没有 VPS 专用 .env 时用 deploy/lopezpress.env 作模板
ENV_LOCAL="$SCRIPT_DIR/lopezpress.env"
cp -f "$ENV_LOCAL" "$SCRIPT_DIR/.env.vps"

echo "==> 上传到 ${SSH_TARGET}:${APP_DIR} ..."
ssh "${SSH_ARGS[@]}" "$SSH_TARGET" "mkdir -p ${APP_DIR}/web"
scp "${SSH_ARGS[@]}" \
  "$PROJECT_ROOT/bin/lopezpress" \
  "$PROJECT_ROOT/deploy/install-systemd.sh" \
  "$PROJECT_ROOT/deploy/lopezpress.service" \
  "${SSH_TARGET}:${APP_DIR}/"
# 前端构建产物
( cd "$PROJECT_ROOT/web/dist" && tar cf - . ) | \
  ssh "${SSH_ARGS[@]}" "$SSH_TARGET" "cd ${APP_DIR}/web && tar xf -"
scp "${SSH_ARGS[@]}" \
  "$SCRIPT_DIR/.env.vps" \
  "${SSH_TARGET}:${APP_DIR}/.env"

echo "==> 在 VPS 上安装/重启 systemd 服务..."
ssh "${SSH_ARGS[@]}" "$SSH_TARGET" bash -s <<REMOTE
set -e
cd ${APP_DIR}
bash install-systemd.sh
REMOTE

echo "==> 部署完成。健康检查："
echo "    ssh ${SSH_TARGET} 'curl -s http://localhost:8080/health'"
echo "    （记得在 ${APP_DIR}/.env 里把 JWT_SECRET 换成强随机值）"
