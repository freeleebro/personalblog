-- 搜索排序用 trigram 相似度：标题命中权重最高，其次是摘要。
-- pg_trgm 是 PostgreSQL 官方 contrib 模块，官方镜像自带。
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS posts_title_trgm_idx ON posts USING gin (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS posts_summary_trgm_idx ON posts USING gin (summary gin_trgm_ops);