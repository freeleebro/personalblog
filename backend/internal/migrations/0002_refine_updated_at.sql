-- 0002_refine_updated_at.sql
--
-- 让 posts.updated_at 只在「内容字段」发生变化时才更新。
--
-- 背景：阅读量自增是高频写操作（每次打开文章都会触发一次 UPDATE）。
-- 如果它也刷新 updated_at，会带来两个副作用：
--   1. 「最近更新」时间被无意义地刷脏，不再代表内容真的变过；
--   2. sitemap 的 <lastmod> 天天在变，搜索引擎会降低对它的信任。
--
-- 这里不改 0001（迁移一旦执行过就不再修改），而是用一个新的迁移替换触发器定义。

DROP TRIGGER IF EXISTS trg_posts_touch ON posts;

CREATE TRIGGER trg_posts_touch
    BEFORE UPDATE ON posts
    FOR EACH ROW
    WHEN (
        OLD.title        IS DISTINCT FROM NEW.title
     OR OLD.summary      IS DISTINCT FROM NEW.summary
     OR OLD.body_md      IS DISTINCT FROM NEW.body_md
     OR OLD.cover_color  IS DISTINCT FROM NEW.cover_color
     OR OLD.status       IS DISTINCT FROM NEW.status
     OR OLD.featured     IS DISTINCT FROM NEW.featured
     OR OLD.published_at IS DISTINCT FROM NEW.published_at
     OR OLD.author_id    IS DISTINCT FROM NEW.author_id
    )
    EXECUTE FUNCTION touch_updated_at();
