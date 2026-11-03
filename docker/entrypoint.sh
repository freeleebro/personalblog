#!/bin/sh
set -eu

: "${POSTGRES_USER:=blog}"
: "${POSTGRES_PASSWORD:=change-me}"
: "${POSTGRES_DB:=beichuang}"
: "${PGDATA:=/var/lib/postgresql/data}"
: "${APP_ADDR:=:8080}"
: "${API_BASE_URL:=http://127.0.0.1:8080}"
: "${PORT:=3000}"
: "${HOSTNAME:=0.0.0.0}"

mkdir -p "$PGDATA"
chown -R postgres:postgres "$PGDATA"

# 上传目录挂在数据卷上，换镜像不会丢图。
mkdir -p "${UPLOAD_DIR:-/data/uploads}"

if [ ! -s "$PGDATA/PG_VERSION" ]; then
  echo "[app] initializing PostgreSQL"
  pwfile="$(mktemp)"
  printf '%s\n' "$POSTGRES_PASSWORD" > "$pwfile"
  chown postgres:postgres "$pwfile"
  su-exec postgres initdb -D "$PGDATA" -U "$POSTGRES_USER" --pwfile="$pwfile" --auth-host=scram-sha-256 --auth-local=trust --encoding=UTF8 --locale=C
  rm -f "$pwfile"
fi

echo "[app] starting PostgreSQL"
su-exec postgres postgres -D "$PGDATA" -c listen_addresses=127.0.0.1 -c port=5432 -c password_encryption=scram-sha-256 &
PG_PID=$!

cleanup() {
  kill "${WEB_PID:-}" "${API_PID:-}" "$PG_PID" 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

until pg_isready -h 127.0.0.1 -p 5432 -U "$POSTGRES_USER" >/dev/null 2>&1; do
  kill -0 "$PG_PID" 2>/dev/null || { echo "[app] PostgreSQL exited" >&2; exit 1; }
  sleep 1
done

export PGPASSWORD="$POSTGRES_PASSWORD"
if ! psql -h 127.0.0.1 -U "$POSTGRES_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$POSTGRES_DB'" | grep -q 1; then
  createdb -h 127.0.0.1 -U "$POSTGRES_USER" "$POSTGRES_DB"
fi

export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable"
export APP_ADDR

echo "[app] starting Go API"
/usr/local/bin/beichuang-api &
API_PID=$!

until wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1; do
  kill -0 "$API_PID" 2>/dev/null || { echo "[app] API exited" >&2; exit 1; }
  sleep 1
done

echo "[app] starting Next.js on port $PORT"
cd /app/web
API_BASE_URL="$API_BASE_URL" PORT="$PORT" HOSTNAME="$HOSTNAME" node server.js &
WEB_PID=$!

while :; do
  kill -0 "$PG_PID" 2>/dev/null || exit 1
  kill -0 "$API_PID" 2>/dev/null || exit 1
  kill -0 "$WEB_PID" 2>/dev/null || exit 1
  sleep 5
done
