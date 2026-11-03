#!/usr/bin/env bash
# ============================================================
#  本地开发一键启动（热更新）
#
#  同时拉起三个服务，全部支持热更新：
#    1. PostgreSQL  便携版（scripts/dev-postgres.py），监听 127.0.0.1:5433
#    2. Go 后端     air 热重载（改 .go 自动重编译重启），监听 :8080
#    3. Next.js 前台 next dev 自带 HMR，监听 :3000
#
#  用法：
#    bash scripts/dev.sh          # 启动全部（前台聚合日志，Ctrl+C 全部停止）
#    bash scripts/dev.sh stop     # 停止所有后台进程
#    bash scripts/dev.sh status   # 查看运行状态
#
#  首次使用需安装 air（一次性）：
#    go install github.com/air-verse/air@latest
# ============================================================

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# 便携版 PostgreSQL 与 Go 后端共用的连接串
export DATABASE_URL="${DATABASE_URL:-postgres://blog:blog@127.0.0.1:5433/beichuang?sslmode=disable}"
export API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:8080}"

# 各进程的 PID 文件，用于 stop
PID_DIR="$ROOT/.devdb/pids"
mkdir -p "$PID_DIR"

PYTHON="${PYTHON:-python}"
AIR="$(go env GOPATH)/bin/air"
AIR="${AIR//\\//}" # Windows 下把反斜杠转正斜杠
NODE="node"

# 后台进程的日志
LOG_DIR="$ROOT/.devdb/logs"
mkdir -p "$LOG_DIR"

# ------------------------------------------------------------ 工具函数
log() { printf '\033[1;36m[dev]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[dev]\033[0m %s\n' "$*"; }

pid_of() { local f="$PID_DIR/$1.pid"; [ -f "$f" ] && cat "$f" || true; }
is_running() { local p; p="$(pid_of "$1")"; [ -n "$p" ] && kill -0 "$p" 2>/dev/null; }

start_postgres() {
  if is_running postgres; then
    log "PostgreSQL 已在运行（PID $(pid_of postgres)）"
    return 0
  fi
  log "启动便携版 PostgreSQL…"
  "$PYTHON" scripts/dev-postgres.py >"$LOG_DIR/postgres.log" 2>&1 &
  echo $! > "$PID_DIR/postgres.pid"
  log "PostgreSQL PID $!"
}

start_api() {
  if is_running api; then
    log "Go 后端已在运行（PID $(pid_of api)）"
    return 0
  fi
  if [ ! -x "$AIR" ] && [ ! -f "$AIR" ] && [ ! -f "${AIR}.exe" ]; then
    warn "未找到 air，尝试用 go run 兜底（无热重载，改代码需手动重启）"
    warn "建议安装：go install github.com/air-verse/air@latest"
    (cd backend && go run ./cmd/server >"$LOG_DIR/api.log" 2>&1 & echo $! > "$PID_DIR/api.pid")
    return 0
  fi
  log "启动 Go 后端（air 热重载）…"
  AIR_BIN="$AIR"
  [ -f "${AIR}.exe" ] && AIR_BIN="${AIR}.exe"
  (cd backend && "$AIR_BIN" >"$LOG_DIR/api.log" 2>&1 & echo $! > "$PID_DIR/api.pid")
  log "Go 后端 PID $(cat "$PID_DIR/api.pid")（air 进程，日志在 .devdb/logs/api.log）"
}

start_web() {
  if is_running web; then
    log "Next.js 前台已在运行（PID $(pid_of web)）"
    return 0
  fi
  log "启动 Next.js 前台（next dev，HMR）…"
  (cd web && API_BASE_URL="$API_BASE_URL" "$NODE" ./node_modules/.bin/next dev >"$LOG_DIR/web.log" 2>&1 & echo $! > "$PID_DIR/web.pid")
  log "Next.js PID $(cat "$PID_DIR/web.pid")（日志在 .devdb/logs/web.log）"
}

stop_one() {
  local name="$1"
  local p
  p="$(pid_of "$name")"
  if [ -n "$p" ] && kill -0 "$p" 2>/dev/null; then
    # 停掉整棵进程树（air / next 会派生子进程）
    kill "$p" 2>/dev/null || true
    log "已停止 $name（PID $p）"
  else
    log "$name 未在运行"
  fi
  rm -f "$PID_DIR/$name.pid"
}

stop_all() {
  # 先停 web（它依赖 api），再停 api，最后 postgres
  stop_one web
  stop_one api
  stop_one postgres
}

status() {
  for s in postgres api web; do
    if is_running "$s"; then
      printf '  \033[32m●\033[0m %-10s running (PID %s)\n' "$s" "$(pid_of "$s")"
    else
      printf '  \033[90m○\033[0m %-10s stopped\n' "$s"
    fi
  done
}

# ------------------------------------------------------------ 主流程
cmd="${1:-start}"
case "$cmd" in
  start)
    log "本地开发环境启动中（数据目录 .devdb/，日志 .devdb/logs/）"
    start_postgres
    start_api
    start_web
    echo
    log "全部启动完成："
    echo "   博客前台  http://localhost:3000"
    echo "   管理后台  http://localhost:3000/admin  （默认账号 admin / admin123）"
    echo "   后端接口  http://localhost:8080/api/v1/health"
    echo "   数据库    127.0.0.1:5433（blog/beichuang）"
    echo
    log "热更新：改 Go 代码 air 自动重启；改前端 next dev 自动刷新。"
    log "停止全部：bash scripts/dev.sh stop"
    log "看日志：tail -f .devdb/logs/api.log  或  .devdb/logs/web.log"
    ;;
  stop)
    log "停止本地开发环境…"
    stop_all
    ;;
  status)
    log "本地开发环境状态："
    status
    ;;
  logs)
    tail -f "$LOG_DIR/${2:-api}.log"
    ;;
  *)
    echo "用法：bash scripts/dev.sh [start|stop|status|logs [api|web|postgres]]"
    exit 1
    ;;
esac
