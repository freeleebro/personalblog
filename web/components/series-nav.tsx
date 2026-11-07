import Link from 'next/link';

import type { Post } from '@/lib/types';

/**
 * 文章顶部的系列提示条。
 *
 * 只在文章确实属于某个系列时渲染。index 是它在系列中的位置（0 起），
 * total 是系列已发布篇数；拿不到完整列表时退化成只显示系列名，
 * 不猜「第几篇」——猜错比不显示更让人困惑。
 */
export function SeriesBanner({
  slug,
  title,
  index,
  total,
}: {
  slug: string;
  title: string;
  index: number;
  total: number;
}) {
  const hasPosition = index >= 0 && total > 0;

  return (
    <Link href={`/series/${encodeURIComponent(slug)}`} className="series-banner">
      <span className="series-banner__label">系列</span>
      <span className="series-banner__title">{title}</span>
      {hasPosition ? (
        <span className="series-banner__pos">
          第 {index + 1} / {total} 篇
        </span>
      ) : null}
      <span className="series-banner__arrow" aria-hidden="true">
        →
      </span>
    </Link>
  );
}

/**
 * 文末的系列目录：按顺序列出整个合集，当前这篇高亮。
 * 读者读到这里通常已经想「还有没有下一篇」，目录比单纯的上一篇/下一篇更好用。
 */
export function SeriesDirectory({
  slug,
  title,
  posts,
  currentSlug,
}: {
  slug: string;
  title: string;
  posts: Post[];
  currentSlug: string;
}) {
  if (posts.length === 0) return null;

  return (
    <section className="series-dir" aria-label={`系列：${title}`}>
      <div className="series-dir__head">
        <h2 className="series-dir__title">
          系列：<Link href={`/series/${encodeURIComponent(slug)}`}>{title}</Link>
        </h2>
        <span className="series-dir__count">共 {posts.length} 篇</span>
      </div>
      <ol className="series-dir__list">
        {posts.map((item, index) => {
          const active = item.slug === currentSlug;
          return (
            <li key={item.id} className="series-dir__item" data-active={active}>
              <span className="series-dir__index" aria-hidden="true">
                {String(index + 1).padStart(2, '0')}
              </span>
              {active ? (
                <span className="series-dir__link">
                  {item.title}
                  <span className="series-dir__here">当前</span>
                </span>
              ) : (
                <Link href={`/posts/${encodeURIComponent(item.slug)}`} className="series-dir__link">
                  {item.title}
                </Link>
              )}
            </li>
          );
        })}
      </ol>
    </section>
  );
}
