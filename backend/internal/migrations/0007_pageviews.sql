-- 访问统计：只存页面路径、来源站点、按天加盐的访客哈希，不存 IP / UA 原文。
CREATE TABLE IF NOT EXISTS page_views (
    id            BIGSERIAL PRIMARY KEY,
    path          TEXT        NOT NULL,
    referrer_host TEXT        NOT NULL DEFAULT '',
    visitor       TEXT        NOT NULL DEFAULT '',
    is_bot        BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS page_views_created_idx ON page_views (created_at);
CREATE INDEX IF NOT EXISTS page_views_path_idx ON page_views (path, created_at);