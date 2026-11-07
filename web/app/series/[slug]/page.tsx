import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';

import { ApiError, getSeries, getSeriesList } from '@/lib/api';
import { formatDate, isoDate } from '@/lib/format';
import type { Series } from '@/lib/types';

type Params = Promise<{ slug: string }>;

/** 系列 slug 可能是中文，Next 会解码；这里做一次防御性兜底。 */
function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { slug } = await params;
  try {
    const { series, posts } = await getSeries(safeDecode(slug));
    return {
      title: `系列：${series.title}`,
      description:
        series.description || `「${series.title}」系列，共 ${posts.length} 篇，按阅读顺序排列。`,
    };
  } catch {
    return { title: '系列' };
  }
}

export default async function SeriesDetailPage({ params }: { params: Params }) {
  const { slug } = await params;

  let detail;
  try {
    detail = await getSeries(safeDecode(slug));
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      notFound();
    }
    throw error;
  }

  const { series, posts } = detail;

  let others: Series[] = [];
  try {
    others = (await getSeriesList()).filter((item) => item.slug !== series.slug);
  } catch {
    others = [];
  }

  return (
    <div className="shell page">
      <div className="page__head">
        <Link href="/series" className="back-link">
          ← 全部系列
        </Link>
        <span className="series-card__badge">系列</span>
        <h1 className="page__title">{series.title}</h1>
        <p className="page__desc">
          {series.description ? `${series.description} ` : ''}
          共 {posts.length} 篇，按阅读顺序排列。
        </p>
      </div>

      <div className="layout">
        <div>
          <ol className="series-toc">
            {posts.map((post, index) => (
              <li key={post.id} className="series-toc__item">
                <span className="series-toc__index" aria-hidden="true">
                  {String(index + 1).padStart(2, '0')}
                </span>
                <div className="series-toc__body">
                  <h2 className="series-toc__title">
                    <Link href={`/posts/${encodeURIComponent(post.slug)}`}>{post.title}</Link>
                  </h2>
                  {post.summary ? <p className="series-toc__desc">{post.summary}</p> : null}
                  <div className="series-toc__meta">
                    <time dateTime={isoDate(post.published_at)}>{formatDate(post.published_at)}</time>
                    <span className="post-card__dot" />
                    <span>{post.reading_minutes} 分钟</span>
                  </div>
                </div>
              </li>
            ))}
          </ol>
        </div>

        <aside className="sidebar">
          {others.length > 0 ? (
            <div className="panel">
              <h2 className="panel__title">其他系列</h2>
              <ul className="sidebar-list">
                {others.map((item) => (
                  <li key={item.id} className="sidebar-list__item">
                    <Link
                      href={`/series/${encodeURIComponent(item.slug)}`}
                      className="sidebar-list__title"
                    >
                      {item.title}
                    </Link>
                    <span className="sidebar-list__date">{item.post_count} 篇</span>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </aside>
      </div>
    </div>
  );
}
