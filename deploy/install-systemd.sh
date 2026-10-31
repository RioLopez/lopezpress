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
# 二进制必须可执行，而且这里不能静默失败。
#
# 踩过的坑：scp 覆盖文件会把权限位重置成 644，而 systemd 对此报的是
#   Failed to locate executable ...: Permission denied
#   Main process exited, code=exited, status=203/EXEC
# 这两行只出现在 journalctl 里；从外面看只是 nginx 的 502，很容易被误判成
# 端口占用或者代码问题，于是在端口和配置上来回折腾。所以显式设位 + 校验，
# 不通过就直接退出。
chmod 755 "$APP_DIR/lopezpress"
if [[ ! -x "$APP_DIR/lopezpress" ]]; then
  echo "错误：$APP_DIR/lopezpress 不可执行（chmod 755 未生效？）" >&2
  exit 1
fi

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
PORT="$(grep -E '^PORT=' "$APP_DIR/.env" 2>/dev/null | tail -1 | cut -d= -f2)"
echo "完成。健康检查：curl http://127.0.0.1:${PORT:-8080}/health"
