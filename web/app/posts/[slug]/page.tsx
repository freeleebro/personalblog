import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';

import { ReadingProgress } from '@/components/reading-progress';
import { Comments } from '@/components/comments';
import { Markdown } from '@/components/markdown';
import { PostCard } from '@/components/post-card';
import { TableOfContents } from '@/components/table-of-contents';
import { TagChip } from '@/components/tag-chip';
import { ViewCounter } from '@/components/view-counter';
import { IconArrowLeft, IconCalendar, IconClock } from '@/components/icons';
import { ApiError, getPost, getRelated } from '@/lib/api';
import { formatDate, isoDate } from '@/lib/format';
import { buildToc } from '@/lib/toc';
import type { Post } from '@/lib/types';

type Params = Promise<{ slug: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { slug } = await params;
  try {
    const { post } = await getPost(slug);
    // 有封面就用封面，没有就退回站点默认卡片图；相对路径由 metadataBase 补全成绝对地址。
    // 封面尺寸由用户上传决定，不要硬写 1200×630，否则部分平台会按错误比例裁切。
    const images = post.cover_url
      ? [{ url: post.cover_url, alt: post.title }]
      : [{ url: '/og-image.jpg', width: 1200, height: 630, alt: post.title }];
    return {
      title: post.title,
      description: post.summary,
      openGraph: {
        type: 'article',
        title: post.title,
        description: post.summary,
        publishedTime: post.published_at ?? undefined,
        tags: post.tags.map((tag) => tag.name),
        images,
      },
      twitter: {
        card: 'summary_large_image',
        title: post.title,
        description: post.summary,
        images: images.map((image) => image.url),
      },
    };
  } catch {
    return { title: '文章' };
  }
}

export default async function PostPage({ params }: { params: Params }) {
  const { slug } = await params;

  let detail;
  try {
    detail = await getPost(slug);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      notFound();
    }
    throw error;
  }

  const { post, prev, next } = detail;
  const body = post.body_md ?? '';
  const toc = buildToc(body);

  let related: Post[] = [];
  try {
    related = await getRelated(post.slug, 3);
  } catch {
    related = [];
  }

  return (
    <>
      <ReadingProgress />

      <div className="shell">
        <div className="article-head">
          <Link href="/posts" className="back-link">
            <IconArrowLeft width={14} height={14} />
            返回文章列表
          </Link>

          <h1 className="article-title">{post.title}</h1>

          {post.summary ? <p className="article-summary">{post.summary}</p> : null}

          <div className="article-meta">
            <span className="article-meta__item">
              <IconCalendar width={14} height={14} />
              <time dateTime={isoDate(post.published_at)}>{formatDate(post.published_at)}</time>
            </span>
            <span className="article-meta__item">
              <IconClock width={14} height={14} />
              {post.reading_minutes} 分钟阅读
            </span>
            <ViewCounter slug={post.slug} initial={post.views} />
            {post.author ? <span className="article-meta__item">作者：{post.author.name}</span> : null}
          </div>

          {post.tags.length > 0 ? (
            <div className="post-card__tags" style={{ marginTop: 16 }}>
              {post.tags.map((tag) => (
                <TagChip key={tag.id} tag={tag} />
              ))}
            </div>
          ) : null}
        </div>

        <div className="article-body">
          <div className="article-grid">
            <article>
              <Markdown>{body}</Markdown>

              <nav className="post-nav">
                {prev ? (
                  <Link href={`/posts/${encodeURIComponent(prev.slug)}`} className="post-nav__item">
                    <span className="post-nav__label">← 上一篇</span>
                    <span className="post-nav__title">{prev.title}</span>
                  </Link>
                ) : (
                  <span className="post-nav__spacer">已经是最新一篇</span>
                )}

                {next ? (
                  <Link
                    href={`/posts/${encodeURIComponent(next.slug)}`}
                    className="post-nav__item post-nav__item--next"
                  >
                    <span className="post-nav__label">下一篇 →</span>
                    <span className="post-nav__title">{next.title}</span>
                  </Link>
                ) : (
                  <span className="post-nav__spacer">已经是最后一篇</span>
                )}
              </nav>

              <Comments slug={post.slug} />

              {related.length > 0 ? (
                <>
                  <div className="divider" />
                  <div className="section-head" style={{ marginTop: 34 }}>
                    <h2 className="section-head__title">相关文章</h2>
                  </div>
                  <div className="post-list">
                    {related.map((item, index) => (
                      <PostCard key={item.id} post={item} index={index} />
                    ))}
                  </div>
                </>
              ) : null}
            </article>

            <TableOfContents items={toc} />
          </div>
        </div>
      </div>
    </>
  );
}
