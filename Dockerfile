# ---------- Go API ----------
FROM golang:1.27-alpine AS api-builder
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---------- Next.js dependencies ----------
FROM node:26-alpine AS web-deps
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund

# ---------- Next.js build ----------
FROM node:26-alpine AS web-builder
WORKDIR /src/web
COPY --from=web-deps /src/web/node_modules ./node_modules
COPY web ./
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ---------- PostgreSQL + Go API + Next.js in one container ----------
FROM postgres:17-alpine
RUN apk add --no-cache wget su-exec
COPY --from=web-builder /usr/local/bin/node /usr/local/bin/node
COPY --from=api-builder /out/server /usr/local/bin/beichuang-api
COPY --from=web-builder /src/web/public /app/web/public
COPY --from=web-builder /src/web/.next/standalone /app/web
COPY --from=web-builder /src/web/.next/static /app/web/.next/static
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh \
    && mkdir -p /var/lib/postgresql/data /data/uploads \
    && chown -R postgres:postgres /var/lib/postgresql/data
ENV APP_ENV=production APP_ADDR=:8080 AUTO_MIGRATE=true API_BASE_URL=http://127.0.0.1:8080 PORT=3000 HOSTNAME=0.0.0.0 NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PGDATA=/var/lib/postgresql/data UPLOAD_DIR=/data/uploads
VOLUME ["/data/uploads"]
EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
