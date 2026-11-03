#!/usr/bin/env bash
# 服务器端部署脚本：把一个源码包原子地换成线上版本，健康检查失败自动回滚。
#
# 用法：
#   bash deploy.sh /opt/beichuang-upload.tar.gz
#
# 约定：
#   - 项目目录默认 /opt/beichuang，可用 ROOT 覆盖
#   - 运行期配置 .env.server 不进源码包，部署时从当前线上目录继承
#   - 旧版本目录保留到新版本健康检查通过；失败则切回去并重新构建
#   - 日志追加到 /var/log/beichuang-deploy.log
#
# 由 scripts/release.ps1 在本机调用（打包 → 上传 → 执行本脚本）。

set -euo pipefail

ROOT="${ROOT:-/opt/beichuang}"
TARBALL="${1:-}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-240}"
LOG_FILE="${LOG_FILE:-/var/log/beichuang-deploy.log}"
APP_CONTAINER="${APP_CONTAINER:-beichuang}"

log() { echo "[$(date '+%F %T')] $*" | tee -a "$LOG_FILE"; }
die() { log "ERROR: $*"; exit 1; }

compose() { ( cd "$ROOT" && docker compose "$@" ); }

[ -n "$TARBALL" ] || die "用法: $0 <tar.gz>"
[ -f "$TARBALL" ] || die "找不到源码包: $TARBALL"
tar -tzf "$TARBALL" >/dev/null 2>&1 || die "源码包损坏: $TARBALL"
[ -d "$ROOT" ] || die "项目目录不存在: $ROOT"
[ -f "$ROOT/.env.server" ] || die "缺少 $ROOT/.env.server"

NEXT="${ROOT}.next"
OLD="${ROOT}.old"
FAILED="${ROOT}.failed"

log "==== 部署开始: $TARBALL ===="
rm -rf "$NEXT" "$FAILED"
mkdir -p "$NEXT"
tar -xzf "$TARBALL" -C "$NEXT"

# 包结构检查：不合规就直接退出，线上目录一点没动
[ -f "$NEXT/docker-compose.yml" ] || die "源码包里缺少 docker-compose.yml"
[ -f "$NEXT/Dockerfile" ] || die "源码包里缺少 Dockerfile"
[ -d "$NEXT/web" ] || die "源码包里缺少 web/"

cp -a "$ROOT/.env.server" "$NEXT/.env.server"
chmod 600 "$NEXT/.env.server"

if docker image inspect beichuang-beichuang >/dev/null 2>&1; then
  docker image tag beichuang-beichuang beichuang:previous >/dev/null
  log "已把当前镜像标记为 beichuang:previous"
fi

rollback() {
  log "部署失败，开始回滚"
  compose down --remove-orphans >>"$LOG_FILE" 2>&1 || true
  rm -rf "$FAILED"
  mv "$ROOT" "$FAILED"
  mv "$OLD" "$ROOT"
  if ! compose up -d --build >>"$LOG_FILE" 2>&1; then
    log "回滚重建也失败了，需要人工介入：cd $ROOT && docker compose up -d --build"
    exit 1
  fi
  log "回滚完成，线上仍是上一版（失败版本留在 $FAILED，可删）"
  exit 1
}

cd /
log "切换目录: $ROOT -> $OLD"
mv "$ROOT" "$OLD"
mv "$NEXT" "$ROOT"

log "构建并启动新版本"
if ! compose up -d --build >>"$LOG_FILE" 2>&1; then
  rollback
fi

log "等待健康检查（最长 ${HEALTH_TIMEOUT}s）"
deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
while :; do
  status=$(docker inspect --format '{{.State.Health.Status}}' "$APP_CONTAINER" 2>/dev/null || echo missing)
  http=$(curl -fsS -o /dev/null -w '%{http_code}' --max-time 5 http://127.0.0.1/ 2>/dev/null || echo 000)
  if [ "$status" = "healthy" ] && [ "$http" = "200" ]; then
    log "健康检查通过（容器 healthy，首页 200）"
    break
  fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    log "健康检查超时：容器=$status 首页HTTP=$http"
    rollback
  fi
  sleep 5
done

rm -rf "$OLD"
log "==== 部署成功 ===="