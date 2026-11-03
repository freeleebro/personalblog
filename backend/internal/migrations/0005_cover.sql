-- 文章封面图：为空时前台回退到色块（保持向后兼容）。
ALTER TABLE posts ADD COLUMN IF NOT EXISTS cover_url TEXT NOT NULL DEFAULT '';
