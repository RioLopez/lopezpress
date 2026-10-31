#!/usr/bin/env bash
# fix-deploy.sh — 把服务器上的端口与配置一次性对齐。可重复执行。
#
# 统一后的布局：
#   Go 后端   127.0.0.1:8081   只监听环回，不对公网开放
#   nginx     0.0.0.0:8088     对外唯一入口
#   8080      弃用（历史上被 nginx 占过，socket 可能有残留）
#
# 用法（在开发机项目根目录）：
#   scp deploy/fix-deploy.sh root@<这台IP>:/tmp/
#   ssh root@<这台IP> 'bash /tmp/fix-deploy.sh'
#
# 或者在服务器上直接：bash fix-deploy.sh
#
# 会修改的东西（旧文件都备份成 .bak）：
#   /opt/lopezpress/.env        只动 PORT，必要时补 JWT_SECRET
#   nginx 的 server 配置        整体替换成标准版（保留原 server_name）
# 不会动的东西：数据库、上传文件、web/dist。

set -uo pipefail

APP_DIR="${APP_DIR:-/opt/lopezpress}"
GO_PORT="${GO_PORT:-8081}"
WEB_PORT="${WEB_PORT:-8088}"
SERVICE="${SERVICE:-lopezpress}"
SVC_USER="${SVC_USER:-echoapi}"
NGINX_CONF="${NGINX_CONF:-/etc/nginx/sites-available/lopezpress}"

ok()   { printf '  [OK]   %s\n' "$*"; }
warn() { printf '  [WARN] %s\n' "$*"; }
step() { printf '\n== %s\n' "$*"; }
die()  { printf '\n[FATAL] %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------- 0. 前置检查
step "0. 前置检查"
[ -f "$APP_DIR/lopezpress" ] || die "找不到 $APP_DIR/lopezpress，先上传二进制"
[ -f "$APP_DIR/.env" ]       || die "找不到 $APP_DIR/.env"
[ -f "$NGINX_CONF" ]         || die "找不到 $NGINX_CONF"

MAGIC=$(head -c 4 "$APP_DIR/lopezpress" | od -An -tx1 | tr -d ' \n')
case "$MAGIC" in
7f454c46)
	ok "二进制是 ELF / Linux，大小 $(du -h "$APP_DIR/lopezpress" | cut -f1)"
	;;
4d5a*)
	die "二进制以 4d5a(MZ) 开头 —— 这是 Windows 程序，Linux 无法执行。
      在开发机上交叉编译后重新上传：
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o bin/lopezpress ./cmd/server
        scp bin/lopezpress root@<这台IP>:$APP_DIR/lopezpress
      然后再跑一次本脚本。"
	;;
*)
	warn "二进制 magic=$MAGIC，不是标准 ELF，继续尝试（日志里若有 Exec format error 就是这里的问题）"
	;;
esac

# ---------------------------------------------------------------- 1. 停服务
step "1. 停止服务（改配置期间不跑）"
systemctl stop "$SERVICE" 2>/dev/null
ok "已停止 $SERVICE（原本没在跑也算正常）"

# ---------------------------------------------------------------- 2. .env
step "2. 规范 .env（只改 PORT，必要时补 JWT_SECRET）"
cp -a "$APP_DIR/.env" "$APP_DIR/.env.bak"
OLD_PORT=$(grep -E '^PORT=' "$APP_DIR/.env" | tail -1 | cut -d= -f2 | tr -d ' "' || true)
if grep -qE '^PORT=' "$APP_DIR/.env"; then
	sed -i "s|^PORT=.*|PORT=${GO_PORT}|" "$APP_DIR/.env"
else
	echo "PORT=${GO_PORT}" >>"$APP_DIR/.env"
fi
ok "PORT: ${OLD_PORT:-<缺失>} -> ${GO_PORT}"

SEC=$(grep -E '^JWT_SECRET=' "$APP_DIR/.env" | tail -1 | cut -d= -f2- | tr -d ' "' || true)
case "$SEC" in
"" | CHANGE_ME_TO_32_RANDOM_BYTES_HEX | change-me* | dev-secret*)
	NEW=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
	if grep -qE '^JWT_SECRET=' "$APP_DIR/.env"; then
		sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${NEW}|" "$APP_DIR/.env"
	else
		echo "JWT_SECRET=${NEW}" >>"$APP_DIR/.env"
	fi
	warn "JWT_SECRET 是空/占位值，已换成 32 字节随机串。所有已登录用户会掉线，需重新登录。"
	;;
*)
	ok "JWT_SECRET 已配置（长度 ${#SEC}），保持不动"
	;;
esac

for k in DB_PATH UPLOAD_DIR; do
	if grep -qE "^${k}=" "$APP_DIR/.env"; then
		ok "$k = $(grep -E "^${k}=" "$APP_DIR/.env" | tail -1 | cut -d= -f2-)"
	else
		warn "缺少 $k，用默认值启动（数据可能落在意料外的位置）"
	fi
done

# ---------------------------------------------------------------- 3. 权限
step "3. 目录与权限"
mkdir -p "$APP_DIR/data" "$APP_DIR/logs"
chmod +x "$APP_DIR/lopezpress"
if id "$SVC_USER" >/dev/null 2>&1; then
	chown -R "$SVC_USER":"$SVC_USER" "$APP_DIR/data" "$APP_DIR/logs" 2>/dev/null
	ok "data / logs 属主设为 $SVC_USER"
else
	warn "系统里没有用户 $SVC_USER，跳过 chown（检查 systemd unit 里的 User=）"
fi

# ---------------------------------------------------------------- 4. nginx
step "4. 重写 nginx 配置（listen ${WEB_PORT} -> 后端 ${GO_PORT}）"
SERVER_NAME=$(grep -E '^[[:space:]]*server_name' "$NGINX_CONF" | head -1 |
	sed 's/.*server_name//; s/;//' | xargs)
[ -n "$SERVER_NAME" ] || SERVER_NAME="_"
ok "保留原 server_name: $SERVER_NAME"

cp -a "$NGINX_CONF" "$NGINX_CONF.bak"
cat >"$NGINX_CONF" <<'NGINXEOF'
# 由 deploy/fix-deploy.sh 生成。改动前先看 .bak。
# 布局：nginx 对外 8088，后端 Go 在环回 8081。
server {
    listen __WEB_PORT__;
    server_name __SERVER_NAME__;
    server_tokens off;

    client_max_body_size 32m;
    client_body_timeout 60s;

    root /opt/lopezpress/web;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # 前缀必须带 ^~：下面的正则 location 会抢走 /uploads/xxx.jpg，
    # 让图片不走代理而直接在 web 根目录找不到文件（线上全 404）。
    location ^~ /uploads/ {
        proxy_pass http://127.0.0.1:__GO_PORT__;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }

    location = /feed        { proxy_pass http://127.0.0.1:__GO_PORT__; proxy_set_header Host $host; proxy_set_header X-Forwarded-Proto $scheme; }
    location = /sitemap.xml { proxy_pass http://127.0.0.1:__GO_PORT__; proxy_set_header Host $host; proxy_set_header X-Forwarded-Proto $scheme; }
    location = /robots.txt  { proxy_pass http://127.0.0.1:__GO_PORT__; proxy_set_header Host $host; proxy_set_header X-Forwarded-Proto $scheme; }

    location /api/ {
        proxy_pass http://127.0.0.1:__GO_PORT__;
        proxy_http_version 1.1;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
        proxy_connect_timeout 5s;
    }

    location /health {
        proxy_pass http://127.0.0.1:__GO_PORT__;
        proxy_http_version 1.1;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Host $host;
    }

    # 构建产物文件名带 hash，可以长缓存。nginx 的 add_header 不继承，
    # 本 block 声明后必须把安全头重复一遍。
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
        add_header X-Content-Type-Options nosniff always;
        add_header X-Frame-Options DENY always;
        add_header Referrer-Policy strict-origin-when-cross-origin always;
    }

    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header Referrer-Policy strict-origin-when-cross-origin always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'" always;

    gzip on;
    gzip_min_length 1024;
    gzip_types application/json text/plain application/javascript text/css image/svg+xml;

    access_log /var/log/nginx/lopezpress.access.log;
    error_log  /var/log/nginx/lopezpress.error.log;
}
NGINXEOF

sed -i "s|__WEB_PORT__|${WEB_PORT}|g; s|__GO_PORT__|${GO_PORT}|g; s|__SERVER_NAME__|${SERVER_NAME}|g" \
	"$NGINX_CONF"

if ! nginx -t 2>/tmp/fixdeploy.ngx.err; then
	cat /tmp/fixdeploy.ngx.err
	cp -a "$NGINX_CONF.bak" "$NGINX_CONF"
	die "nginx 配置校验失败，已回滚到 $NGINX_CONF.bak。上面是错误原因。"
fi
ok "nginx -t 通过"

# 确保被 sites-enabled 启用
if ! ls /etc/nginx/sites-enabled/ 2>/dev/null | grep -qx "$(basename "$NGINX_CONF")"; then
	ln -sf "$NGINX_CONF" "/etc/nginx/sites-enabled/$(basename "$NGINX_CONF")"
	ok "已补建 sites-enabled 软链"
fi

systemctl restart nginx || die "nginx 起不来，看 journalctl -u nginx -n 30"
ok "nginx 已重启"

# ---------------------------------------------------------------- 5. 起后端
step "5. 启动后端"
systemctl daemon-reload
systemctl restart "$SERVICE"
sleep 3

if ! systemctl is-active --quiet "$SERVICE"; then
	printf '\n---- journalctl -u %s -n 30 ----\n' "$SERVICE"
	journalctl -u "$SERVICE" -n 30 --no-pager
	die "后端没起来。上面是日志，原因基本都写在那里。"
fi
ok "后端进程在跑"

# ---------------------------------------------------------------- 6. 验证
step "6. 验证"
echo "-- 监听端口（期望 8088 是 nginx、8081 是 lopezpress，没有 8080）--"
ss -lntp 2>/dev/null | grep -E ":(${GO_PORT}|${WEB_PORT})\b" || warn "两个端口都没监听上"

H=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:${GO_PORT}/health")
C=$(curl -s -o /tmp/fixdeploy.cap --max-time 10 "http://127.0.0.1:${GO_PORT}/api/auth/captcha")
printf '  后端 /health          -> %s\n' "$H"
printf '  后端 /api/auth/captcha -> %s\n' "$(head -c 70 /tmp/fixdeploy.cap)"

W=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:${WEB_PORT}/login")
printf '  nginx :%s/login       -> %s\n' "$WEB_PORT" "$W"

echo
if [ "$H" = "200" ] && printf '%s' "$C" | grep -q '"enabled"'; then
	printf '全部通了。浏览器打开： http://<这台机器IP>:%s/login\n' "$WEB_PORT"
	printf '别忘了云控制台安全组放行 %s/TCP，并且不要放行 %s。\n' "$WEB_PORT" "$GO_PORT"
else
	warn "后端起来了但接口还不对，把上面的输出和 journalctl 一起发回来"
fi
