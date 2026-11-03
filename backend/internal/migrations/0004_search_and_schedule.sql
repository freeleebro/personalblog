-- 0004: 搜索索引覆盖正文，并让站点统计遵守「定时发布」。
--
-- 背景：
-- 1) 搜索接口同时对 title / summary / body_md 做 ILIKE，但 0001 里的
--    idx_posts_trgm 只建在 (title || ' ' || summary) 上，正文匹配用不到索引，
--    数据一多就退化成全表扫描。
-- 2) site_overview 只过滤 status，没有过滤 published_at。于是把文章时间
--    填到未来，它照样进首页统计、进 RSS、进 sitemap——「定时发布」是假的。

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') THEN
        -- 与查询里的匹配表达式保持一致，才能命中。
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_posts_search_all_trgm
                 ON posts USING gin (
                     (coalesce(title, '''') || '' '' || coalesce(summary, '''') || '' '' || coalesce(body_md, '''')) gin_trgm_ops
                 )';
    END IF;
END
$$;

-- 覆盖旧索引后，旧的 (title || summary) 索引就成了纯写放大，删掉。
DROP INDEX IF EXISTS idx_posts_trgm;

CREATE OR REPLACE VIEW site_overview AS
SELECT
    (SELECT count(*) FROM posts
      WHERE status = 'published' AND published_at IS NOT NULL AND published_at <= now())
        AS post_count,
    (SELECT count(*) FROM tags)
        AS tag_count,
    (SELECT COALESCE(sum(length(body_md)), 0) FROM posts
      WHERE status = 'published' AND published_at IS NOT NULL AND published_at <= now())
        AS total_chars,
    (SELECT max(published_at) FROM posts
      WHERE status = 'published' AND published_at <= now())
        AS last_published_at;
