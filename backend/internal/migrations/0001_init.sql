-- 0001_init.sql —— 博客的基础表结构、索引与触发器。
-- 该文件由 internal/store 的迁移器在单个事务中执行，因此这里不写 BEGIN/COMMIT。

-- pg_trgm 用于标题/摘要的模糊搜索加速。精简版 PostgreSQL 可能未附带 contrib，
-- 所以做成「可用则建索引，不可用则静默降级」。
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS pg_trgm;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pg_trgm 不可用，模糊搜索将退化为顺序扫描';
END
$$;

-- ---------------------------------------------------------------- 作者

CREATE TABLE IF NOT EXISTS authors (
    id         BIGSERIAL   PRIMARY KEY,
    slug       TEXT        NOT NULL UNIQUE,
    name       TEXT        NOT NULL,
    avatar_url TEXT        NOT NULL DEFAULT '',
    bio        TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------- 标签

CREATE TABLE IF NOT EXISTS tags (
    id          BIGSERIAL   PRIMARY KEY,
    slug        TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL UNIQUE,
    color       TEXT        NOT NULL DEFAULT '#64748b',
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------- 文章

CREATE TABLE IF NOT EXISTS posts (
    id              BIGSERIAL   PRIMARY KEY,
    slug            TEXT        NOT NULL UNIQUE,
    title           TEXT        NOT NULL,
    summary         TEXT        NOT NULL DEFAULT '',
    body_md         TEXT        NOT NULL DEFAULT '',
    cover_color     TEXT        NOT NULL DEFAULT '#334155',
    status          TEXT        NOT NULL DEFAULT 'published'
                                CHECK (status IN ('draft', 'published', 'archived')),
    featured        BOOLEAN     NOT NULL DEFAULT false,
    reading_minutes INTEGER     NOT NULL DEFAULT 1 CHECK (reading_minutes > 0),
    views           INTEGER     NOT NULL DEFAULT 0 CHECK (views >= 0),
    author_id       BIGINT      REFERENCES authors (id) ON DELETE SET NULL,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 列表页按发布时间倒序，索引与之对齐。
CREATE INDEX IF NOT EXISTS idx_posts_published_at
    ON posts (published_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_posts_status_published
    ON posts (status, published_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_posts_author
    ON posts (author_id);

-- ---------------------------------------------------------------- 文章 ↔ 标签

CREATE TABLE IF NOT EXISTS post_tags (
    post_id BIGINT NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    tag_id  BIGINT NOT NULL REFERENCES tags (id)  ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_post_tags_tag ON post_tags (tag_id);

-- ---------------------------------------------------------------- 独立页面

CREATE TABLE IF NOT EXISTS pages (
    id         BIGSERIAL   PRIMARY KEY,
    slug       TEXT        NOT NULL UNIQUE,
    title      TEXT        NOT NULL,
    body_md    TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------- 站点设置（JSONB）

CREATE TABLE IF NOT EXISTS settings (
    key        TEXT        PRIMARY KEY,
    value      JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------- updated_at 自动维护

CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_posts_touch ON posts;
CREATE TRIGGER trg_posts_touch BEFORE UPDATE ON posts
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

DROP TRIGGER IF EXISTS trg_pages_touch ON pages;
CREATE TRIGGER trg_pages_touch BEFORE UPDATE ON pages
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

DROP TRIGGER IF EXISTS trg_settings_touch ON settings;
CREATE TRIGGER trg_settings_touch BEFORE UPDATE ON settings
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- ---------------------------------------------------------------- 模糊搜索索引

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_posts_trgm
                 ON posts USING gin ((title || '' '' || summary) gin_trgm_ops)';
    END IF;
END
$$;

-- ---------------------------------------------------------------- 站点概览视图

CREATE OR REPLACE VIEW site_overview AS
SELECT
    (SELECT count(*) FROM posts WHERE status = 'published')                            AS post_count,
    (SELECT count(*) FROM tags)                                                        AS tag_count,
    (SELECT COALESCE(sum(length(body_md)), 0) FROM posts WHERE status = 'published')   AS total_chars,
    (SELECT max(published_at) FROM posts WHERE status = 'published')                   AS last_published_at;
