-- 0009: 系列 / 合集。
--
-- 一篇长文拆成几篇发，读者需要一个「从头读」的入口。这里用独立的 series 表
-- 加 posts.series_id 外键，而不是复用标签：标签是多对多、无顺序的；系列有
-- 明确的阅读顺序和一段简介，是「一个合集里的第几篇」这种一对多关系。
--
-- 删除系列时把成员文章的 series_id 置空（ON DELETE SET NULL），文章本身留着。
-- 站长删合集通常只是不想再分组了，不是想把文章一起删掉。

CREATE TABLE IF NOT EXISTS series (
    id          BIGSERIAL   PRIMARY KEY,
    slug        TEXT        NOT NULL UNIQUE,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE posts ADD COLUMN IF NOT EXISTS series_id BIGINT
    REFERENCES series (id) ON DELETE SET NULL;

-- 系列内的阅读序号，从 1 开始；同一系列内不强制唯一，
-- 站长留空（0）时按发布时间兜底排序，不会因为序号重复而报错。
ALTER TABLE posts ADD COLUMN IF NOT EXISTS series_order INT NOT NULL DEFAULT 0;

-- 前台按 series_id 取整个合集并排序，这一个复合索引够用；
-- 反向查「这篇文章属于哪个系列」走外键列的前缀。
CREATE INDEX IF NOT EXISTS posts_series_idx ON posts (series_id, series_order);
