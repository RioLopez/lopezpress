#!/usr/bin/env bash
# 只读诊断：把部署状态一次性打印出来。不改任何文件、不重启任何服务。
#
# 用法（在 VPS 上）：
#   bash diagnose.sh
# 可选环境变量：
#   APP_DIR=/opt/lopezpress BACKEND_PORT=8080 bash diagnose.sh
#
# 故意不打印 .env 全文：里面有 JWT_SECRET，只挑端口和路径这几行。

set -uo pipefail

APP_DIR="${APP_DIR:-/opt/lopezpress}"
BACKEND_PORT="${BACKEND_PORT:-8080}"

line() { printf '\n==== %s ====\n' "$1"; }

line "1. 二进制（格式 + 大小 + 时间）"
if [ -f "$APP_DIR/lopezpress" ]; then
	ls -l "$APP_DIR/lopezpress"
	# ELF 以 7f 45 4c 46 开头；Windows PE 以 4d 5a（MZ）开头。
	printf 'magic: '
	head -c 4 "$APP_DIR/lopezpress" | od -An -tx1 | tr -s ' ' | sed 's/^ //'
else
	echo "缺失: $APP_DIR/lopezpress"
fi

line "2. systemd 状态"
systemctl is-active lopezpress 2>&1 || true
systemctl is-enabled lopezpress 2>&1 || true
systemctl show -p ExecMainStartTimestamp -p ExecMainStatus -p NRestarts \
	lopezpress 2>/dev/null || true

line "3. 最近 25 行日志（真正的原因通常在这里）"
journalctl -u lopezpress -n 25 --no-pager 2>&1 | tail -25

line "4. 谁在监听哪些端口"
ss -lntp 2>/dev/null | grep -E ':(80|443|8080|8081|8082|8090)[[:space:]]' || echo "无匹配端口"

line "5. 服务配置里的端口（不含密钥）"
grep -E '^(PORT|DB_PATH|UPLOAD_DIR|CAPTCHA_ENABLED)=' "$APP_DIR/.env" 2>/dev/null ||
	echo "读不到 $APP_DIR/.env"

line "6. nginx 的 listen 与 proxy_pass"
grep -rnE '^[[:space:]]*(listen|proxy_pass)' \
	/etc/nginx/conf.d/ /etc/nginx/sites-enabled/ /etc/nginx/nginx.conf 2>/dev/null |
	head -24 || echo "没找到 nginx 配置"

line "7. 直连后端端口"
echo "-- 期望是 Go 的 JSON；如果返回 nginx 的 502 页面，说明这个端口上是 nginx 而不是 Go"
curl -s -o /tmp/_h.txt -w 'health=%{http_code} ' --max-time 5 \
	"http://127.0.0.1:${BACKEND_PORT}/health" 2>&1 || true
head -c 60 /tmp/_h.txt 2>/dev/null
echo
curl -s -o /tmp/_c.txt -w 'captcha=%{http_code} ' --max-time 5 \
	"http://127.0.0.1:${BACKEND_PORT}/api/auth/captcha" 2>&1 || true
head -c 60 /tmp/_c.txt 2>/dev/null
echo

line "完成"
echo "把 1-7 全部输出贴回去即可。脚本没有修改任何东西。"
