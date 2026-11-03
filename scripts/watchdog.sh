#!/usr/bin/env bash
# 服务器巡检：磁盘、内存、容器健康、备份新鲜度。异常时写 journal 并可推送 webhook。
#
# 用法：
#   ./scripts/watchdog.sh
#   DISK_WARN=95 MEM_MIN_MB=1 ./scripts/watchdog.sh   # 手动触发一次告警，验证通知链路
#
# systemd：beichuang-watchdog.timer 每天 09:00（Asia/Shanghai）跑一次。
# 可选环境变量（写进 /etc/default/beichuang-watchdog 即可被 service 读到）：
#   WEBHOOK_URL   告警地址，POST {"text": "..."}（Slack / 飞书机器人 / 自建都可用）
#   DISK_WARN     磁盘占用告警阈值，默认 80（%）
#   MEM_MIN_MB    可用内存下限，默认 150（MB）
#   BACKUP_MAX_AGE_DAYS  最近备份的最大天数，默认 8
#   BACKUP_DIR    备份目录，默认 /var/backups/beichuang
#   APP_CONTAINER 应用容器名，默认 beichuang

set -uo pipefail

DISK_WARN="${DISK_WARN:-80}"
MEM_MIN_MB="${MEM_MIN_MB:-150}"
BACKUP_MAX_AGE_DAYS="${BACKUP_MAX_AGE_DAYS:-8}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/beichuang}"
APP_CONTAINER="${APP_CONTAINER:-beichuang}"
WEBHOOK_URL="${WEBHOOK_URL:-}"

problems=()

disk_used=$(df -P / | awk 'NR==2 {gsub("%","",$5); print $5}')
if [ -n "$disk_used" ] && [ "$disk_used" -ge "$DISK_WARN" ]; then
  problems+=("磁盘占用 ${disk_used}%（阈值 ${DISK_WARN}%）")
fi

mem_avail=$(awk '/^MemAvailable:/ {print int($2/1024)}' /proc/meminfo)
if [ -n "$mem_avail" ] && [ "$mem_avail" -lt "$MEM_MIN_MB" ]; then
  problems+=("可用内存 ${mem_avail}MB（阈值 ${MEM_MIN_MB}MB）")
fi

health=$(docker inspect --format '{{.State.Health.Status}}' "$APP_CONTAINER" 2>/dev/null || echo missing)
if [ "$health" != "healthy" ]; then
  problems+=("容器 ${APP_CONTAINER} 状态异常：${health}")
fi

latest_backup=$(ls -1t "$BACKUP_DIR"/beichuang-db-*.sql.gz 2>/dev/null | head -1 || true)
if [ -z "$latest_backup" ]; then
  problems+=("在 ${BACKUP_DIR} 找不到数据库备份")
else
  age_days=$(( ( $(date +%s) - $(stat -c %Y "$latest_backup") ) / 86400 ))
  if [ "$age_days" -gt "$BACKUP_MAX_AGE_DAYS" ]; then
    problems+=("最近备份已 ${age_days} 天（阈值 ${BACKUP_MAX_AGE_DAYS} 天）：${latest_backup}")
  fi
fi

if [ ${#problems[@]} -eq 0 ]; then
  echo "ok: 磁盘 ${disk_used}% / 可用内存 ${mem_avail}MB / 容器 ${health} / 备份 $(basename "$latest_backup")"
  exit 0
fi

msg="beichuang 巡检告警：$(printf '%s；' "${problems[@]}")"
echo "$msg" >&2

if [ -n "$WEBHOOK_URL" ]; then
  if ! curl -fsS -m 10 -H 'Content-Type: application/json' \
      -d "{\"text\":\"${msg}\"}" "$WEBHOOK_URL" >/dev/null; then
    echo "webhook 推送失败：$WEBHOOK_URL" >&2
  fi
fi

exit 1