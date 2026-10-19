#!/usr/bin/env bash
# install-systemd.sh — 在 VPS 上安装 systemd 服务
# 用法：把本脚本 + lopezpress + lopezpress.env 放到 /opt/lopezpress/，然后：
#   cd /opt/lopezpress
#   sudo bash /path/to/install-systemd.sh
# 前置：lopezpress 二进制、.env、lopezpress.service 必须就位。

set -euo pipefail

APP_DIR="/opt/lopezpress"
SERVICE_NAME="lopezpress"
SERVICE_USER="echoapi"
SERVICE_GROUP="echoapi"

# 1. 检查前置文件
for f in "$APP_DIR/lopezpress" "$APP_DIR/.env"; do
  if [[ ! -e "$f" ]]; then
    echo "错误：缺少 $f" >&2
    exit 1
  fi
done
chmod +x "$APP_DIR/lopezpress" 2>/dev/null || true

# 2. 创建系统用户（非交互，nologin）
if ! id "$SERVICE_USER" &>/dev/null; then
  useradd --system --create-home --home-dir "$APP_DIR" --shell /usr/sbin/nologin \
    --comment "Echo Content API service" "$SERVICE_USER"
  echo "已创建用户 $SERVICE_USER"
fi
groupadd "$SERVICE_GROUP" 2>/dev/null || true
chown -R "$SERVICE_USER:$SERVICE_GROUP" "$APP_DIR"

# 3. 确保 data 目录存在（SQLite 落盘位置）
mkdir -p "$APP_DIR/data"
chown "$SERVICE_USER:$SERVICE_GROUP" "$APP_DIR/data"

# 4. 安装 systemd 单元
cp "$APP_DIR/lopezpress.service" "/etc/systemd/system/${SERVICE_NAME}.service"
# 让 systemd 重新读取
systemctl daemon-reload

# 5. 启用 + 启动
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

echo
echo "=== 服务状态 ==="
systemctl --no-pager status "$SERVICE_NAME" || true
echo
echo "=== 最近日志 ==="
journalctl -u "$SERVICE_NAME" -n 20 --no-pager || true
echo
echo "完成。健康检查：curl http://localhost:8080/health"
