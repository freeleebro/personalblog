import Link from 'next/link';

import { TagChip } from './tag-chip';
import { IconClock, IconSparkle } from './icons';
import { compactNumber, formatDate, isoDate } from '@/lib/format';
import type { Post } from '@/lib/types';

/** 首页顶部的精选文章卡片，比列表卡片更大的排版层级。 */
export function FeaturedPost({ post }: { post: Post }) {
  return (
    <article className="featured fade-up">
      <div className="featured__bar" />
      <div className="featured__body">
        <div className="featured__label">
          <IconSparkle width={14} height={14} />
          精选
        </div>

        <h2 className="featured__title">
          <Link href={`/posts/${encodeURIComponent(post.slug)}`}>{post.title}</Link>
        </h2>

        <p className="featured__excerpt">{post.summary}</p>

        <div className="featured__foot">
          <span className="post-card__meta">
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
          </span>

          <span className="post-card__tags" style={{ marginTop: 0 }}>
            {post.tags.map((tag) => (
              <TagChip key={tag.id} tag={tag} />
            ))}
          </span>
        </div>
      </div>
    </article>
  );
}
