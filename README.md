# 个人博客（Next.js + Go + PostgreSQL）

一个前后端全自己搭的个人博客：**Next.js 16（App Router） + Go（标准库） + PostgreSQL 17**，
本地使用热更新开发脚本，服务器用 Docker 部署（应用容器 + Caddy 反代）。

- 文章列表 / 文章详情 / 标签分类 / 归档 / 关于 / 全文搜索
- Markdown 渲染（表格、代码高亮、目录锚点、阅读进度、浏览量）
- 浅色 / 深色双主题，服务端渲染为主
- **管理后台**（`/admin`）：登录后在线写作、管理已发布文章、标签与站点设置

> 仓库里只有代码和中性的占位文案，**没有任何个人内容**：站点名、副标题、关于页、
> 首页大图上的字全部存在数据库里，由后台编辑。克隆下来直接部署，就是一个干净的空白博客。

---

## 〇、第一次使用：初始化站点

用默认账号登录 `/admin`（用户名密码来自 `.env.server` 的 `ADMIN_USERNAME` / `ADMIN_PASSWORD`），
然后按这个顺序填一遍：

1. **站点设置**：站点名称、副标题、描述、站点网址；下面是「首页文案」和「页面文案」——
   首页大图标题、角标、图集标题与说明，以及文章页 / 归档页 / 标签页 / 搜索页的标题、
   副标题和 404 文案。留空就整段不渲染或回落到代码里的中性默认值。
2. **关于页面**：初次启动会写入一段占位说明，直接改成你自己的介绍。
3. **分类 / 标签**：建几个分类，写文章时勾选。
4. **文章**：新建或导入 Markdown，图片可以直接粘贴 / 拖进编辑框。

以上内容全部存在 PostgreSQL 里，换服务器只要带上数据卷即可，不需要改代码。
如果想让仓库里带一份自己的默认文案，在 `.env.server` 里配 `SITE_NAME` / `SITE_TAGLINE` /
`SITE_DESCRIPTION`，首次启动写入数据库后就不再覆盖了。

---

## 一、服务器部署（单机双容器）

```bash
cd personal-blog
cp .env.server.example .env.server
# 编辑 .env.server，填入数据库和管理员密码
docker compose up -d --build
```

容器分工：

- `beichuang`：内部依次启动 PostgreSQL → Go API → Next.js，只监听宿主机 `127.0.0.1:5000`，不直接对公网。
- `caddy`：唯一对外入口，负责 TLS、gzip/zstd 压缩、静态资源长缓存和安全响应头。

| 入口 | 地址 |
| --- | --- |
| 未配域名 | `http://服务器IP/` |
| 配了 `BLOG_DOMAIN` | `https://你的域名`（Caddy 自动申请并续期 Let's Encrypt 证书） |
| 管理后台 | 同上，路径 `/admin` |

`BLOG_DOMAIN` 留空时 Caddy 只监听 80 端口走 HTTP；填了域名（例如 `BLOG_DOMAIN=blog.example.com`）就会自动上 HTTPS 并强制跳转。
记得在云厂商安全组 / 本机防火墙放行 **80 和 443**（有些服务器默认只开了 22 和你原来的 5000）。
Caddy 反代依赖 `TRUSTED_PROXIES`（默认已包含 Docker 内网段），否则后端会把所有访客都算成同一个 IP，登录限流和浏览量统计都会失真。
因此部署后 `SITE_URL` / `ALLOWED_ORIGINS` 要改成访客实际访问的地址，否则 RSS、sitemap 和 CORS 里仍是旧地址。

首次启动时后端只会自动建库、建表、执行迁移，并创建一个初始管理员账号（默认 `admin` / `admin`）。
文章、页面、标签和站点文本不会自动注入，全部通过 `/admin` 上传或修改。
生产部署务必在 `.env.server` 里修改 `ADMIN_PASSWORD` 和 `ADMIN_SECRET`。

> 改 `ADMIN_PASSWORD` 只会影响「首次创建账号」那一次。账号已经存在时，后端启动会比对
> 环境变量里的密码与库里的哈希，不一致只打一条警告；确实要重置就把 `ADMIN_PASSWORD_RESET=1`
> 加进 `.env.server` 重启一次，重置完再改回 `0`。
>
> **日常改密码不用重启**：登录后台 →「站点设置」→ 页面底部的「修改密码」，输入当前密码和新密码
> 即可，立即生效。改完之后 `ADMIN_PASSWORD` 就过期作废了（留着旧值也不会再触发警告）。
>
> 注意登录 token 是无状态 HMAC 签名，改密码**不会**让已经登录的设备掉线。要把所有设备踢下线，
> 只能换掉 `ADMIN_SECRET` 再重启一次容器。

常用命令：

```bash
docker compose ps               # 查看容器健康状态
docker compose logs -f          # 跟踪日志（已限制单文件 10MB × 5 份）
docker compose down             # 停止（数据保留在 beichuang-data 卷里）
docker compose down -v          # 停止并删除数据卷（彻底重置）
```

改端口不要动 `docker-compose.yml`，在 `personal-blog/.env` 里覆盖即可（见下方环境变量）。

### 发布与回滚

本机改完代码后，在仓库根目录跑一条命令：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release.ps1 -SshHost leebro
```

它负责打包（排除 `node_modules`、`.next`、`.env*`、本地数据库等）、上传，然后在服务器上执行
`deploy.sh`：解包到 `.next` → 继承旧的 `.env.server` → 切换目录 → `docker compose up -d --build`
→ 等容器 `healthy` 且首页 200。任何一步失败都会自动切回上一版并重新构建，
全程日志在 `/var/log/beichuang-deploy.log`。

手工发布也建议照同样顺序：解包到 `/opt/beichuang.next`、复制 `.env.server`、把旧目录改名
`/opt/beichuang.old`、再构建，验证通过后再删旧目录。回滚就是把 `.old` 换回来（脚本失败时已自动做）；
脚本还会把上一版镜像标记成 `beichuang:previous`，应急时可以用。

备份要同时覆盖两个卷：数据库用 `pg_dump` 逻辑导出（比直接 tar 数据目录安全，还原时不会
因为数据文件版本或半写状态出问题），图片卷打成 tar.gz。仓库里带了脚本：

```bash
sudo /opt/beichuang/scripts/backup.sh        # 手工跑一次，输出在 /var/backups/beichuang
```

装成每周自动跑（周日 03:30 北京时间，服务器时区是 UTC 也不影响）：

```bash
sudo install -m 644 docker/systemd/beichuang-backup.service /etc/systemd/system/
sudo install -m 644 docker/systemd/beichuang-backup.timer   /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now beichuang-backup.timer
systemctl list-timers beichuang-backup.timer   # 看下次触发时间
journalctl -u beichuang-backup.service -n 20   # 看上次跑的结果
```

默认保留 30 天（一周一次 ≈ 留 4~5 份），可用环境变量覆盖：`BACKUP_DIR`（默认
`/var/backups/beichuang`）、`RETENTION_DAYS`（默认 `30`）。每次备份都会写一份
`sha256` 校验和。

还原：

```bash
# 数据库（dump 带 --clean，会先删同名对象再重建）
gunzip -c /var/backups/beichuang/beichuang-db-20261103T081456.sql.gz \
  | docker exec -i beichuang psql -U blog -d beichuang

# 图片卷（先把旧内容清掉再解包，注意别在 -C 上写错路径）
docker run --rm --entrypoint sh -v beichuang_beichuang-media:/dst beichuang-beichuang \
  -c 'rm -rf /dst/*'
docker run --rm --entrypoint tar -v beichuang_beichuang-media:/dst \
  -v /var/backups/beichuang:/backup beichuang-beichuang \
  -xzf /backup/beichuang-media-20261103T081456.tar.gz -C /dst
```

> 备份默认落在同一块盘上，防的是「误删 / 升级搞坏」，防不了「整台机器没了」。
> 真在意就把 `/var/backups/beichuang` 用 rsync 或 rclone 同步到别处，一行定时任务的事。

内容层面还有一层出口：后台「站点设置 → 导出全部内容」会生成一个 zip，里面每篇文章是带
front matter 的 `.md`、独立页面单独一个文件、上传过的图片在 `media/` 下、外加 `posts.json`。
换服务器或者哪天不想用这套系统了，直接拿这个 zip 走人；解压后也能用编辑器的
「导入 Markdown 文件」逐篇导回去。

### 每日巡检（磁盘 / 内存 / 容器 / 备份新鲜度）

```bash
sudo install -m 644 docker/systemd/beichuang-watchdog.service /etc/systemd/system/
sudo install -m 644 docker/systemd/beichuang-watchdog.timer   /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now beichuang-watchdog.timer
journalctl -u beichuang-watchdog.service -n 20
```

每次检查四件事：根分区占用 ≥ 80%（`DISK_WARN`）、可用内存 < 150MB（`MEM_MIN_MB`）、
`beichuang` 容器不是 `healthy`、最近一次数据库备份超过 8 天（`BACKUP_MAX_AGE_DAYS`）。
有异常就写 journal 并返回非 0；想推到手机或群里，把 webhook 写进
`/etc/default/beichuang-watchdog`：

```bash
sudo tee /etc/default/beichuang-watchdog >/dev/null <<'EOF'
WEBHOOK_URL=https://example.com/webhook
EOF
```

告警内容以 `{"text": "..."}` POST 出去，Slack、飞书机器人、自建接口都能直接收。
手动触发一次告警验证链路：`DISK_WARN=1 sudo -E /opt/beichuang/scripts/watchdog.sh`（会返回 1）。

> 反代与服务在同一台机器时，`5000` 端口只绑在 `127.0.0.1`；如需换端口，改 `docker-compose.yml` 里 `ports` 的 `published` 部分。

---

## 二、环境变量

所有配置都有默认值，**不配也能直接跑**。需要定制时：

```bash
cp .env.example .env
```

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `blog` / `blog` / `beichuang` | 数据库凭据 |
| `AUTHOR_SLUG` | `author` | 首页作者卡片指向 `authors` 表里的哪一行；没有对应记录就不显示 |
| `SITE_TIMEZONE` | `Asia/Shanghai` | 站点时区；中文标题自动 slug 的日期、后台「排到明天 09:00」都按它算 |
| `BLOG_DOMAIN` | 空 | Caddy 对外域名；空则按 IP 走 HTTP，填了自动申请证书 |
| `SITE_URL` | `http://localhost:3000` | 用于 RSS / sitemap / Open Graph 的绝对地址，部署后改成真实地址 |
| `SITE_NAME` / `SITE_TAGLINE` / `SITE_DESCRIPTION` | 空 | 站点信息；也可以在管理后台保存 |
| `ALLOWED_ORIGINS` | `http://localhost:3000,http://127.0.0.1:3000` | 后端 CORS 白名单 |
| `TRUSTED_PROXIES` | Docker 内网段 + 回环 | 允许提供 `X-Forwarded-For` 的代理网段，公网直连的伪造头会被忽略 |

> 生产部署请务必修改数据库密码、管理员密码和 `ADMIN_SECRET`。

### 图片资源

首屏插画已按显示尺寸压成 WebP（约 283KB，原来是 1.18MB 的 JPG/PNG）。
替换或新增插画后，把原图放进 `web/public/`，然后执行：

```bash
cd web && npm run optimize-images
```

脚本会用 `sharp` 生成 `anime-*-*.webp`，记得同步修改 `globals.css` 和 `page.tsx` 里的引用。

分享卡片图（`og:image`，微信 / Twitter / Slack 抓取的那张）是 `web/public/og-image.jpg`（1200×630）。换底图后重新生成：

```bash
cd web && npm run og-image
```

文章如果在后台设置了封面图，分享卡片会优先用封面；没有封面才用这张默认图。

---

## 三、本地开发（热更新）

日常开发推荐用这个方式：不依赖 Docker，改代码即时生效。

**首次准备（一次性）**：

```bash
# 1. 安装 Go 热重载工具 air
go install github.com/air-verse/air@latest

# 2. 安装前端依赖（已装过可跳过）
cd web && npm install && cd ..
```

**一键启动**（会同时拉起 PostgreSQL + Go 后端 + Next.js 前台）：

```bash
bash scripts/dev.sh          # 启动，日志聚合在 .devdb/logs/
bash scripts/dev.sh status   # 查看运行状态
bash scripts/dev.sh stop     # 全部停止
bash scripts/dev.sh logs api # 跟踪某个服务的日志
```

热更新能力：

| 服务 | 热更新方式 | 说明 |
| --- | --- | --- |
| Go 后端 | `air` 自动重编译重启 | 改 `.go` / `.sql` 保存即自动重启 |
| Next.js 前台 | `next dev` 自带 HMR | 改组件/样式浏览器即时刷新 |
| PostgreSQL | 便携版（`.devdb/`） | 数据持久化在本地，不依赖 Docker |

本地开发入口是前台 3000、后台 `/admin`、接口 8080、数据库 5433。

> 脚本用的是便携版 PostgreSQL（`scripts/dev-postgres.py`，zonky embedded-postgres 约 23MB，
> 解压到 `.devdb/`）。它不带 `psql` / `createdb`，所以后端内置了 `AUTO_CREATE_DATABASE`：
> 连不上目标库时自动 `CREATE DATABASE`，省掉手工建库。

### 管理后台导入 Markdown

打开 `http://localhost:3000/admin` 后，文章工作台支持直接导入 `.md` / `.markdown` 文件。
导入后会自动读取标题、摘要、slug、状态、日期与分类标签，并在预览面板中渲染 Markdown；确认内容后点击保存即可写入博客。

Markdown 文件可以使用可选的 front matter：

```yaml
---
title: 一篇导入进来的文章
slug: unity-input-system-notes
summary: 这里是一段摘要，会出现在列表页。
tags: [随笔, 技术]
status: draft
date: 2026-10-23
---
```

分类和标签在后台的“分类 / 标签”面板中维护；一篇文章可以同时选择多个分类。

---

## 四、目录结构

```
personal-blog/
├── docker-compose.yml          # 服务器启动入口（博客容器 + Caddy）
├── Dockerfile                  # PostgreSQL + API + Next.js 单容器镜像
├── .env.example
├── docker/
│   ├── Caddyfile               # 反代 / TLS / 压缩 / 静态长缓存 / 安全头
│   └── systemd/                # backup / watchdog 的 service + timer
├── scripts/
│   ├── deploy.sh               # 服务器端部署：原子切换 + 健康检查 + 失败自动回滚
│   ├── release.ps1             # 本机一键发布：打包 → 上传 → 调用 deploy.sh
│   ├── watchdog.sh             # 每日巡检：磁盘 / 内存 / 容器 / 备份新鲜度
│   ├── backup.sh               # 数据库 + 媒体卷备份
│   └── dev-postgres.py         # 免 Docker 的便携版 PostgreSQL
├── backend/                    # Go API
│   ├── cmd/server/main.go      # 启动顺序：建库 → 连接 → 迁移 → HTTP
│   ├── internal/
│   │   ├── api/                # 路由、handler、RSS / sitemap、响应缓存
│   │   ├── config/             # 环境变量 + .env 加载
│   │   ├── domain/             # 数据模型
│   │   ├── markdown/           # 纯文本摘要、阅读时长、字数
│   │   ├── middleware/         # RequestID / 日志 / Recover / CORS / 限流 / 真实 IP
│   │   ├── migrations/         # 内嵌 SQL 迁移（go:embed）
│   │   └── store/              # pgx 连接池 + 查询
└── web/                        # Next.js 前台
    ├── app/                    # 路由（App Router）
    ├── components/             # 组件
    ├── lib/                    # API 客户端、格式化、TOC
    ├── public/                 # 图片等静态资源（Caddy 给长缓存）
    └── scripts/                # optimize-images.mjs：插画压 WebP；generate-og.mjs：生成分享卡片图
```

---

## 五、接口一览

后端全部挂在 `/api/v1` 下，返回 `{"data": ...}` 或 `{"error": {"code","message"}}`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/health` | 健康检查（compose 的 healthcheck 就靠它） |
| GET | `/api/v1/site` | 站点设置 + 作者信息 |
| GET | `/api/v1/posts` | 文章列表，支持 `?q=` 模糊搜索、`?tag=`、`?page=`、`?per_page=` |
| GET | `/api/v1/posts/{slug}` | 文章详情（含上下篇） |
| GET | `/api/v1/posts/{slug}/related` | 相关文章 |
| POST | `/api/v1/posts/{slug}/views` | 浏览量 +1 |
| GET | `/api/v1/tags` | 标签列表（含文章数） |
| GET | `/api/v1/tags/{slug}` | 单个标签 |
| GET | `/api/v1/archive` | 按年份归档 |
| GET | `/api/v1/pages/{slug}` | 单页（如 about） |
| GET | `/api/v1/feed.xml` | RSS 2.0 |
| GET | `/api/v1/sitemap.xml` | 站点地图 |
| GET | `/media/{path}` | 文章配图（数据卷里的静态文件，一年强缓存） |
| GET | `/api/v1/preview/{slug}?t=&exp=` | 草稿预览，校验签名与有效期 |

前台另有同源代理路由，避免浏览器直连后端（跨域 + 后端地址暴露）：
`/api/search`、`/api/views/[slug]`、`/feed.xml`，以及管理后台专用的 `/api-proxy/[...path]`。

**管理后台接口**（均需 `Authorization: Bearer <token>`，token 由登录接口签发）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/v1/admin/login` | 登录，返回 token（无需鉴权） |
| GET | `/api/v1/admin/whoami` | 校验 token、返回当前账号 |
| POST | `/api/v1/admin/password` | 修改当前账号密码（`{current_password,new_password}`） |
| GET | `/api/v1/admin/posts` | 文章列表（含草稿/归档） |
| GET | `/api/v1/admin/posts/{slug}` | 单篇（含正文，供编辑器回填） |
| POST/PUT | `/api/v1/admin/posts` / `.../{slug}` | 创建/更新文章 |
| DELETE | `/api/v1/admin/posts/{slug}` | 删除文章 |
| POST | `/api/v1/admin/uploads` | 上传配图（multipart，字段名 `file`），返回 `/media/...` |
| GET | `/api/v1/admin/preview/{slug}` | 生成 1 小时有效的草稿预览链接 |
| GET | `/api/v1/admin/export` | 导出全部内容为 zip（文章 `.md` + 页面 + 图片 + `posts.json`） |
| POST/PUT | `/api/v1/admin/tags` / `.../{slug}` | 创建/更新标签 |
| DELETE | `/api/v1/admin/tags/{slug}` | 删除标签 |
| POST | `/api/v1/admin/pages` | 创建/更新独立页面 |
| DELETE | `/api/v1/admin/pages/{slug}` | 删除独立页面 |
| PUT | `/api/v1/admin/site` | 更新站点设置 |

---

## 六、几个实现上的取舍

**1. 后端零框架。** 只用 Go 标准库 `net/http`。Go 1.22+ 的 `ServeMux` 已支持
`GET /api/v1/posts/{slug}` 这种带方法与路径参数的模式，路由需求完全覆盖，没必要引框架。

**2. 用 `LATERAL + json_agg` 一次性带出标签。** 文章列表要显示标签，朴素写法是每篇文章
再查一次标签（N+1）。这里在一条 SQL 里用 `LATERAL` 子查询配合 `json_agg` 把标签聚合成
JSON 一并返回，列表页恒定为一次查询。

**3. 浏览量自增不污染 `updated_at`。** 触发器加了条件：
`WHEN (OLD.title IS DISTINCT FROM NEW.title OR ...)`，
所以 `view_count + 1` 这类更新不会把 `updated_at` 顶掉——「最近更新」才是真的内容变更时间。

**4. 迁移是内嵌的、幂等的、按文件分事务。** SQL 通过 `go:embed` 编进二进制，
用 `schema_migrations` 表记录已执行版本；每个文件一个事务，失败整体回滚，重复启动无副作用。
`pg_trgm` 扩展用 DO 块做了降级兼容，权限不足时不会让整个迁移崩掉。

**5. 目录锚点和正文标题必须严格对齐。** 目录是自己根据 Markdown 标题生成的，
锚点必须和 `rehype-slug` 渲染出来的 `id` 完全一致，否则点了跳不过去。
中文标题 + 重复标题的去重计数很容易对不齐，所以直接用 `github-slugger`
（也就是 `rehype-slug` 内部用的同一个实现），并且**所有层级都参与计数**，
包括被目录过滤掉的 h1/h4+，保持去重序号一致。

**6. 前台以 SSR 为主。** 页面统一 `force-dynamic`，内容改动立刻可见，不用操心缓存失效。
之所以不做 ISR / 静态化：镜像构建时后端还没起来，构建期静态化会把「后端不可用」的空页面烤进产物。
性能靠两层缓存兜底——Go 侧 15 秒进程内响应缓存（只缓存匿名 GET，后台一写就整体失效），
Caddy 侧对 `/_next/static`、图片等不可变资源下发一年的 `immutable` 缓存。
`error.tsx` / `not-found.tsx` 都做了，后端挂掉时给出可读的提示而不是白屏。
`sitemap.ts` 在后端不可用时降级成只输出静态页面，不至于整个站点地图报错。

**7. 容器内部走回环地址，外部只经过 Caddy。** Next.js 通过 `http://127.0.0.1:8080` 访问 Go API，
博客容器的 5000 端口只绑在宿主机 `127.0.0.1`，公网流量统一由 Caddy 转发并附加安全响应头。
后端只信任 `TRUSTED_PROXIES` 里的代理传来的 `X-Forwarded-For`，直接暴露时伪造的转发头一律忽略。

**8. 已发布但未到时间的文章不会提前泄露。** 所有前台查询（列表、详情、归档、搜索、相关文章、
RSS、sitemap 里的统计视图）都带 `published_at <= now()`，后台编辑器则走单独的「任意状态」查询。
所以「定时发布」不需要 cron：查询条件本身就是调度器，时间一到下一篇请求就能看到。

**9. 配图存在数据卷里，不进镜像。** 上传走 `POST /api/v1/admin/uploads`，落盘到 `UPLOAD_DIR`
（容器里是 `/data/uploads`，compose 挂了 `beichuang-media` 卷），对外由 `/media/{path}` 提供。
文件名是随机串、内容不会变，所以直接给一年 `immutable` 缓存；换镜像、重建容器都不会丢图。
前端上传前会先用 canvas 压到长边 1600px / quality 0.82，服务端只做类型嗅探（认内容不认扩展名）。

**10. 草稿预览用 HMAC 签名，不额外存表。** 后台点「预览链接」时后端用 `ADMIN_SECRET`
对 `slug|过期时间` 签名，1 小时有效；前台 `/preview/{slug}` 校验签名后才返回内容，
签名不对返回 404、过期返回 410，并带 `X-Robots-Tag: noindex`。

**11. 改密码不吊销已签发的 token。** token 是无状态 HMAC，服务端不存会话表，
所以「改密码后所有设备重新登录」需要额外的会话版本机制，个人博客不值得为它加一张表。
换 `ADMIN_SECRET` 会让所有旧 token 立即失效，这就是「强制下线」的开关。
后台改密码时会往 `settings` 写一条 `admin_meta.password_updated_at`，
启动检查据此跳过「环境变量密码与数据库不一致」的警告。

**12. 中文标题不会生成 `%E4%B8%AD` 这种 URL。** 新建文章时 slug 可以留空：纯英文标题会
压缩成 `hello-world`；只要标题里含中文（或全角标点），就改用 `post-20261103-a1b2c3`
（日期 + 标题哈希），对搜索引擎和分享都更友好。想手写可读地址仍然可以直接填，
比如 `ce-shi`。标签不同：标签量少、URL 短，`/tags/中文` 这种形式保留原样。

---

## 七、常见问题

**`docker compose up` 报连不上 Docker API？**
Docker Desktop 没启动。先打开 Docker Desktop 等它变成 Running，再执行。

**5000 被占用？**
改 `docker-compose.yml` 里 `beichuang` 的 `published` 端口。

**80 / 443 被占用？**
多半是宿主机上还跑着 Nginx/Apache。停掉它，或者把 `caddy` 服务的端口映射改成别的（域名 + HTTPS 必须是 443）。

**页面报「无法连接后端服务」？**
容器内部服务没有启动完成。执行 `docker compose ps` 和 `docker compose logs` 查看日志。

**想彻底重置数据？**
`docker compose down -v`，再 `docker compose up -d --build`。数据库会重新初始化，内容仍需从后台上传。

**改了端口后浏览器打不开？**
`SITE_URL` 和 `ALLOWED_ORIGINS` 也要跟着改，否则 RSS / sitemap 里的链接和跨域白名单会对不上。

**配了域名但证书没签发？**
Caddy 需要 80/443 能从公网访问，且域名 A 记录已经指向这台服务器。用 `docker compose logs caddy` 看 ACME 报错。
