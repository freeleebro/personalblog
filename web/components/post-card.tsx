import Link from 'next/link';
import type { CSSProperties, ReactNode } from 'react';

import { TagChip } from './tag-chip';
import { IconClock } from './icons';
import { compactNumber, formatDate, isoDate } from '@/lib/format';
import type { Post } from '@/lib/types';

/** 把关键词包成 <mark>，搜索结果里一眼能看到命中位置。 */
function highlightText(text: string, keyword?: string): ReactNode {
  const needle = keyword?.trim();
  if (!needle) {
    return text;
  }
  const escaped = needle.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const parts = text.split(new RegExp(`(${escaped})`, 'gi'));
  if (parts.length === 1) {
    return text;
  }
  return parts.map((part, i) =>
    i % 2 === 1 ? (
      <mark key={i} className="hl">
        {part}
      </mark>
    ) : (
      part
    ),
  );
}

/**
 * 文章列表卡片。
 * index 用来在色块上显示序号，让列表有节奏感，也不依赖任何图片资源。
 */
export function PostCard({ post, index, highlight }: { post: Post; index: number; highlight?: string }) {
  return (
    <article className="post-card" style={{ '--cover': post.cover_color } as CSSProperties}>
      <div>
        <div className="post-card__meta">
          <time dateTime={isoDate(post.published_at)}>{formatDate(post.published_at)}</time>
          <span className="post-card__dot" />
          <span className="inline-flex items-center gap-1">
            <IconClock width={13} height={13} />
            {post.reading_minutes} 分钟
          </span>
          {post.views > 0 ? (
            <>
              <span className="post-card__dot" />
              <span>{compactNumber(post.views)} 阅读</span>
            </>
          ) : null}
        </div>

        <h2 className="post-card__title">
          <Link href={`/posts/${encodeURIComponent(post.slug)}`}>
            {highlightText(post.title, highlight)}
          </Link>
        </h2>

        <p className="post-card__excerpt">{highlightText(post.summary, highlight)}</p>

        {post.tags.length > 0 ? (
          <div className="post-card__tags">
            {post.tags.map((tag) => (
              <TagChip key={tag.id} tag={tag} />
            ))}
          </div>
        ) : null}
      </div>

      <Link
        href={`/posts/${encodeURIComponent(post.slug)}`}
        className={`post-card__cover${post.cover_url ? ' post-card__cover--image' : ''}`}
        tabIndex={-1}
        aria-hidden="true"
      >
        {post.cover_url ? (
          // 封面来自媒体库，尺寸不固定，交给 CSS 裁切；懒加载避免列表页一起下。
          <img src={post.cover_url} alt="" loading="lazy" decoding="async" />
        ) : null}
        <span>{String(index + 1).padStart(2, '0')}</span>
      </Link>
    </article>
  );
}
