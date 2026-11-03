# 性能基线（T0.3）

采集时间：2026-11-04（优化阶段 0–4 完成后）
采集环境：生产部署（Docker Compose + Caddy，2 vCPU / 2GB，Ubuntu），站点地址以 `https://<域名>/` 代指。

这份文件是「改完到底快不快」的对照表。以后调整图片、缓存或渲染方式，重跑同样的命令再比一次即可。

## 1. 首屏图片总字节

| 资源 | 字节 |
|---|---|
| `anime-night-1200.webp`（hero 大图） | 125,326 |
| `anime-night-640.webp`（图集） | 57,720 |
| `anime-crossing-800.webp`（图集） | 55,194 |
| `anime-underground-800.webp`（图集） | 51,706 |
| **合计（首页可见图片）** | **289,946 ≈ 283 KB** |
| `web/public` 全部静态资源 | 427,905 ≈ 418 KB |

改造前同一批图片是 PNG/JPG（citypop 2.25 MB + 三张 0.62/0.37/0.15 MB），首页约 **3.4 MB**；
现在 **283 KB**，降低约 92%。

采集方式：

```powershell
Get-ChildItem web\public -File | Select-Object Name, Length
```

## 2. 响应头

首页 HTML：

```
Cache-Control: private, no-cache, no-store, max-age=0, must-revalidate
```

静态图片（Caddy 直接下发，`header_down -Cache-Control` 删掉上游那条）：

```
Cache-Control: public, max-age=31536000, immutable
Content-Length: 125326
Content-Type: image/webp
```

`/_next/static/*`、`/media/*`、`/icon.svg`、`/og-image.jpg` 与图片同策略；`/api/*`、`/admin*` 为 `no-store`。

采集方式：

```bash
curl -sI https://<域名>/
curl -sI https://<域名>/anime-night-1200.webp
```

## 3. TTFB / 总耗时

`curl` 连测 5 次（服务器本机回环，排除公网 RTT）：

| # | time_starttransfer | time_total |
|---|---|---|
| 1 | 0.1854 | 0.1995 |
| 2 | 0.0666 | 0.0741 |
| 3 | 0.0457 | 0.0517 |
| 4 | 0.0489 | 0.0571 |
| 5 | 0.0817 | 0.0883 |
| **中位** | **0.0666 ≈ 67 ms** | **0.0741 ≈ 74 ms** |

公网首次访问（浏览器实测，含 RTT 与首次 JS 下载）：

| 指标 | 值 |
|---|---|
| TTFB | 725 ms |
| First Paint / FCP | 864 ms / 864 ms |
| DOMContentLoaded / load | 836 ms / 844 ms |
| LCP（元素 `DIV.hero__visual`） | 880 ms |
| 文档传输量 | 6,869 B（压缩后 6,569 B） |

> 公网样本只有一条链路，绝对值受所在地网络影响；看趋势时以「服务器回环 TTFB 中位数」为准。

采集方式：

```bash
for i in 1 2 3 4 5; do
  curl -o /dev/null -s -w '%{time_starttransfer} %{time_total}\n' https://<域名>/
done
```

浏览器侧：

```js
performance.getEntriesByType('paint')
performance.getEntriesByType('navigation')[0]
// LCP: new PerformanceObserver(...).observe({ type: 'largest-contentful-paint', buffered: true })
```

## 4. 首页数据库查询数

`GET /api/v1/site` 是首屏唯一的数据请求，内部按顺序执行：

1. `settings['site']`（站点名/副标题/文案 texts）
2. `settings['author']`
3. `authors`（按 slug 取作者）
4. `stats`（文章数/字数，一条带子查询的 SQL）
5. `tags`
6. `posts`（最近 5 篇）
7. `posts`（推荐文章 1 篇）

**合计 7 次**（改造前前端还会额外请求 `/featured`、`/tags` 等，瀑布 3–4 个往返）。

采集方式：读 `backend/internal/api/handlers.go` 的 `handleSite`，或临时把 `log_min_duration_statement`
设成 0 数一次，然后改回。

## 5. Lighthouse

未接入 CI 自动化（需要 Chromium + Lighthouse CLI，2GB 服务器上跑会挤占内存）。
上面的「公网首次访问」指标（FCP/LCP/TTFB）在本地 Chrome 的 Performance 面板里可复现，
分数以本地手动跑一次 `lighthouse https://<域名>/ --preset=desktop` 为准，
结果记在这里即可。

## 6. 构建与镜像

| 指标 | 值 |
|---|---|
| `npm run build`（本机，Turbopack） | 约 30 s |
| `docker compose up -d --build`（服务器，2 vCPU） | 2–4 min（首次含 `npm ci` 与 Go 编译） |
| 镜像大小 | 见 `docker images beichuang-beichuang`（Postgres + Node + Caddy 共用基础镜像） |

## 后续怎么用

- 改图片、缓存头、渲染方式后，重跑第 1/2/3 节命令，把新值追加成一行「YYYY-MM-DD」对比。
- 只看两个数就够了：**首页图片总字节** 与 **回环 TTFB 中位数**；其余是解释变量。