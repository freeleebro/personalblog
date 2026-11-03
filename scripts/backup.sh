#!/usr/bin/env bash
# 备份博客的两个数据卷：数据库（逻辑导出）+ 上传的图片（打包）。
#
#   ./scripts/backup.sh                  # 手工跑一次
#   systemctl start beichuang-backup     # 用 systemd 跑（安装方式见 README）
#
# 环境变量（都有默认值）：
#   BACKUP_DIR         备份存放目录，默认 /var/backups/beichuang
#   RETENTION_DAYS     保留天数，默认 30（一周一次 ≈ 留 4~5 份）
#   DB_CONTAINER       跑着 PostgreSQL 的容器名，默认 beichuang
#   COMPOSE_PROJECT    卷名前缀，默认 beichuang
#   MEDIA_VOLUME       图片卷名，默认 ${COMPOSE_PROJECT}_beichuang-media
#   MEDIA_IMAGE        打 tar 用的镜像，默认就是应用镜像（本地已有，不用联网拉）
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/var/backups/beichuang}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
DB_CONTAINER="${DB_CONTAINER:-beichuang}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-beichuang}"
MEDIA_VOLUME="${MEDIA_VOLUME:-${COMPOSE_PROJECT}_beichuang-media}"
MEDIA_IMAGE="${MEDIA_IMAGE:-beichuang-beichuang}"

STAMP="$(date +%Y%m%dT%H%M%S)"
umask 077
mkdir -p "$BACKUP_DIR"

log() { printf '[%s] %s\n' "$(date '+%F %T')" "$*"; }

if ! docker inspect -f '{{.State.Running}}' "$DB_CONTAINER" 2>/dev/null | grep -q true; then
  log "错误：容器 $DB_CONTAINER 没在运行，放弃备份（避免导出一份空文件）"
  exit 1
fi

db_file="$BACKUP_DIR/beichuang-db-$STAMP.sql.gz"
media_file="$BACKUP_DIR/beichuang-media-$STAMP.tar.gz"

# 逻辑导出：比直接 tar 数据目录安全，跨 PostgreSQL 小版本也能还原。
# --clean --if-exists 让还原时可以覆盖已有对象。
log "导出数据库 -> $(basename "$db_file")"
docker exec "$DB_CONTAINER" sh -c \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --clean --if-exists' \
  | gzip -9 > "$db_file"

log "打包图片卷 $MEDIA_VOLUME -> $(basename "$media_file")"
docker run --rm \
  --entrypoint tar \
  -v "$MEDIA_VOLUME:/src:ro" \
  -v "$BACKUP_DIR:/backup" \
  "$MEDIA_IMAGE" -czf "/backup/$(basename "$media_file")" -C /src .

# 一份校验和，方便日后确认备份没坏
( cd "$BACKUP_DIR" && sha256sum "$(basename "$db_file")" "$(basename "$media_file")" > "$(basename "$db_file" .sql.gz).sha256" )

log "数据库 $(du -h "$db_file" | cut -f1)，图片 $(du -h "$media_file" | cut -f1)"

# 只删自己产生的文件，且按修改时间算，避免误删目录里别的东西
log "清理 ${RETENTION_DAYS} 天前的备份"
find "$BACKUP_DIR" -maxdepth 1 -type f \
  \( -name 'beichuang-db-*.sql.gz' -o -name 'beichuang-media-*.tar.gz' -o -name 'beichuang-db-*.sha256' \) \
  -mtime "+${RETENTION_DAYS}" -print -delete

log "完成。当前保留 $(find "$BACKUP_DIR" -maxdepth 1 -name 'beichuang-*' -type f | wc -l) 个文件，目录 $BACKUP_DIR"
