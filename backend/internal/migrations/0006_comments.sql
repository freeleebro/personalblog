-- 读者评论：默认进审核队列（status=pending），后台通过后才公开。
-- 关联用 post_slug 而不是 post_id：文章删掉后评论留着也无害，
-- 反而方便站长看到「这篇文章曾有讨论」。
CREATE TABLE IF NOT EXISTS comments (
    id         BIGSERIAL   PRIMARY KEY,
    post_slug  TEXT        NOT NULL,
    author     TEXT        NOT NULL,
    email      TEXT        NOT NULL DEFAULT '',
    body       TEXT        NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'pending',
    ip_hash    TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 前台读：某篇文章已通过的评论，按时间正序。
CREATE INDEX IF NOT EXISTS comments_post_idx
    ON comments (post_slug, status, created_at);

-- 后台读：按状态倒序分页。
CREATE INDEX IF NOT EXISTS comments_status_idx
    ON comments (status, created_at DESC);