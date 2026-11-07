#!/usr/bin/env bash
# LopezPress 本地开发环境（Linux）
#
# 数据隔离：本脚本永远不会读写 data/app.db 等生产副本，所有运行期数据落在
# data/dev/ 下（app.db / uploads / backups / logs / run），可随时整目录删除重建。
#
# 用法：scripts/dev.sh [up|down|restart|status|logs|build|gate|reset-root|help]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEV_DIR="${DEV_DIR:-$ROOT/data/dev}"
RUN_DIR="$DEV_DIR/run"
LOG_DIR="$DEV_DIR/logs"

# 工具链：仓库声明 Go 1.26.10，本机用 GOTOOLCHAIN 自动拉取/切换。
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.10}"
export GOPROXY="${GOPROXY:-https://goproxy.cn}"
export GOSUMDB="${GOSUMDB:-sum.golang.org}"
export GOTMPDIR="${GOTMPDIR:-$HOME/.cache/gotmp}"

PORT="${PORT:-8080}"
WEB_PORT="${WEB_PORT:-5173}"

# 隔离的运行配置：显式 export 优先级高于仓库根 .env
export PORT
export HOST="${HOST:-127.0.0.1}"
export APP_ENV=dev
export DB_PATH="$DEV_DIR/app.db"
export UPLOAD_DIR="$DEV_DIR/uploads"
export BACKUP_DIR="$DEV_DIR/backups"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export CAPTCHA_ENABLED="${CAPTCHA_ENABLED:-false}"
export LP_API_PORT="$PORT"

mkdir -p "$RUN_DIR" "$LOG_DIR" "$UPLOAD_DIR" "$BACKUP_DIR" "$GOTMPDIR"

SECRET_FILE="$DEV_DIR/.jwt-secret"
if [ ! -f "$SECRET_FILE" ]; then
	( umask 077; head -c 48 /dev/urandom | base64 | tr -d '\n' >"$SECRET_FILE" )
fi
export JWT_SECRET="$(cat "$SECRET_FILE")"

BACKEND_BIN="$DEV_DIR/lopezpress"
BACKEND_PID="$RUN_DIR/backend.pid"
FRONTEND_PID="$RUN_DIR/frontend.pid"

alive() { [ -f "$1" ] && kill -0 "$(cat "$1")" 2>/dev/null; }

# 脱离当前进程组（避免 agent/CI 的 shell 等待后台服务退出），同时把真实 PID
# 写进 pidfile：包装进程先记录 $$，再 exec 成目标程序，因此 PID 不会漂移。
launch() {
	local pidfile="$1" logfile="$2"
	shift 2
	local runner=(bash -c 'echo $$ >"$1"; shift; exec "$@"' _ "$pidfile")
	if command -v setsid >/dev/null 2>&1; then
		setsid "${runner[@]}" "$@" >>"$logfile" 2>&1 &
	else
		nohup "${runner[@]}" "$@" >>"$logfile" 2>&1 &
	fi
}

wait_http() {
	local url="$1" i
	for i in $(seq 1 60); do
		curl -fsS --noproxy '*' --max-time 2 "$url" >/dev/null 2>&1 && return 0
		sleep 0.5
	done
	return 1
}

stop_one() {
	local pidfile="$1" name="$2" pid i
	if alive "$pidfile"; then
		pid="$(cat "$pidfile")"
		echo "停止 $name (pid $pid)"
		kill "$pid" 2>/dev/null || true
		for i in $(seq 1 20); do
			kill -0 "$pid" 2>/dev/null || break
			sleep 0.2
		done
		kill -9 "$pid" 2>/dev/null || true
	else
		echo "$name 未在运行"
	fi
	rm -f "$pidfile"
}

build_backend() {
	( cd "$ROOT" && go build -o "$BACKEND_BIN" ./cmd/server )
}

start_backend() {
	if alive "$BACKEND_PID"; then
		echo "后端已在运行 (pid $(cat "$BACKEND_PID"))"
		return 0
	fi
	echo "编译后端（GOTOOLCHAIN=$GOTOOLCHAIN）..."
	build_backend
	( cd "$ROOT" && launch "$BACKEND_PID" "$LOG_DIR/backend.log" "$BACKEND_BIN" )
	if wait_http "http://127.0.0.1:$PORT/health"; then
		echo "后端就绪：http://127.0.0.1:$PORT"
	else
		echo "后端启动失败，最近日志："
		tail -n 40 "$LOG_DIR/backend.log" || true
		return 1
	fi
}

start_frontend() {
	if alive "$FRONTEND_PID"; then
		echo "前端已在运行 (pid $(cat "$FRONTEND_PID"))"
		return 0
	fi
	if [ ! -d "$ROOT/web/node_modules" ]; then
		echo "缺少 web/node_modules，先执行：cd web && npm ci"
		return 1
	fi
	( cd "$ROOT/web" && launch "$FRONTEND_PID" "$LOG_DIR/frontend.log" \
		node node_modules/vite/bin/vite.js \
		--host 127.0.0.1 --port "$WEB_PORT" --strictPort )
	if wait_http "http://127.0.0.1:$WEB_PORT/"; then
		echo "前端就绪：http://127.0.0.1:$WEB_PORT"
	else
		echo "前端启动失败，最近日志："
		tail -n 40 "$LOG_DIR/frontend.log" || true
		return 1
	fi
}

up() {
	echo "隔离数据目录：$DEV_DIR"
	start_backend
	start_frontend
	echo
	echo "后端  http://127.0.0.1:$PORT   （API / 健康检查）"
	echo "前端  http://127.0.0.1:$WEB_PORT"
	echo "日志  $LOG_DIR/{backend,frontend}.log"
	echo "首启 root 随机密码只打印在后端日志横幅里：scripts/dev.sh logs"
}

status() {
	if alive "$BACKEND_PID" && wait_http "http://127.0.0.1:$PORT/health"; then
		echo "后端  运行中  http://127.0.0.1:$PORT  (pid $(cat "$BACKEND_PID"))"
	else
		echo "后端  未运行或健康检查失败"
	fi
	if alive "$FRONTEND_PID" && wait_http "http://127.0.0.1:$WEB_PORT/"; then
	echo "前端  运行中  http://127.0.0.1:$WEB_PORT  (pid $(cat "$FRONTEND_PID"))"
	else
		echo "前端  未运行"
	fi
}

logs() {
	tail -n 60 -f "$LOG_DIR/backend.log" "$LOG_DIR/frontend.log"
}

case "${1:-up}" in
up) up ;;
down)
	stop_one "$FRONTEND_PID" "前端"
	stop_one "$BACKEND_PID" "后端"
	;;
restart)
	stop_one "$FRONTEND_PID" "前端"
	stop_one "$BACKEND_PID" "后端"
	up
	;;
status) status ;;
logs) logs ;;
build)
	( cd "$ROOT" && bash deploy/build.sh )
	( cd "$ROOT/web" && npm run build )
	;;
gate)
	( cd "$ROOT" \
		&& python3 scripts/arch_check.py \
		&& python3 scripts/doc_check.py \
		&& python3 scripts/contract_check.py \
		&& python3 scripts/security_check.py \
		&& python3 scripts/truth_check.py --declared \
		&& python3 scripts/vuln_check.py )
	;;
reset-root)
	echo "在隔离库 $DB_PATH 上重置 root 密码"
	build_backend
	( cd "$ROOT" && "$BACKEND_BIN" -reset-root-password )
	;;
help | -h | --help)
	sed -n '2,10p' "$0" | sed 's/^# \{0,1\}/  /'
	;;
*)
	echo "未知子命令：$1" >&2
	exit 2
	;;
esac
