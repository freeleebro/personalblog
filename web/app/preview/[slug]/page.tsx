import type { Metadata } from 'next';
import { notFound } from 'next/navigation';

import { Markdown } from '@/components/markdown';
import { ApiError, getPreview } from '@/lib/api';
import { formatDate } from '@/lib/format';

/**
 * 草稿预览页。
 *
 * 只有后台生成的签名链接（1 小时有效）能打开，内容可以是草稿、归档或定时发布中的文章。
 * 所以这一页既不允许被搜索引擎收录，也不允许被任何中间层缓存。
 */
export const dynamic = 'force-dynamic';

export const metadata: Metadata = {
  title: '草稿预览',
  robots: { index: false, follow: false },
};

type Params = Promise<{ slug: string }>;
type SearchParams = Promise<{ t?: string; exp?: string }>;

export default async function PreviewPage({
  params,
  searchParams,
}: {
  params: Params;
  searchParams: SearchParams;
}) {
  const { slug } = await params;
  const { t, exp } = await searchParams;
  if (!t || !exp) notFound();

  let post;
  try {
    post = await getPreview(slug, t, exp);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    if (error instanceof ApiError && error.status === 410) {
      return (
        <div className="shell">
          <div className="article-head">
            <h1 className="article-title">预览链接已过期</h1>
            <p className="article-summary">
              预览链接的有效期是 1 小时。回后台重新点「预览链接」生成一条新的即可。
            </p>
          </div>
        </div>
      );
    }
    throw error;
  }

  return (
    <div className="shell">
      <div className="article-head">
        <p className="admin-notice" style={{ marginBottom: 18 }}>
          草稿预览 · 这篇文章还没有发布，只有拿到这条链接的人能看到
        </p>
        <h1 className="article-title">{post.title}</h1>
        {post.summary ? <p className="article-summary">{post.summary}</p> : null}
        <div className="article-meta">
          <span className="article-meta__item">
            {post.published_at ? `${formatDate(post.published_at)} · ` : ''}
            {post.reading_minutes} 分钟阅读
          </span>
        </div>
      </div>

      <div className="article-body">
        <div className="article-grid">
          <article className="prose">
            <Markdown>{post.body_md ?? ''}</Markdown>
          </article>
        </div>
      </div>
    </div>
  );
}
