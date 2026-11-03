import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';
import type { CSSProperties } from 'react';

import { PostCard } from '@/components/post-card';
import { TagChip } from '@/components/tag-chip';
import { ApiError, getTag, getTags } from '@/lib/api';
import type { Tag } from '@/lib/types';

type Params = Promise<{ slug: string }>;

/** 标签 slug 可能是中文，Next 会解码；这里做一次防御性兜底。 */
function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { slug } = await params;
  const key = safeDecode(slug);
  try {
    const { tag } = await getTag(key);
    return {
      title: `标签：${tag.name}`,
      description: `「${tag.name}」下的全部文章，共 ${tag.post_count} 篇。`,
    };
  } catch {
    return { title: '标签' };
  }
}

export default async function TagPage({ params }: { params: Params }) {
  const { slug } = await params;
  const key = safeDecode(slug);

  let detail;
  try {
    detail = await getTag(key);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      notFound();
    }
    throw error;
  }

  const { tag, posts } = detail;

  let allTags: Tag[] = [];
  try {
    allTags = await getTags();
  } catch {
    allTags = [];
  }

  return (
    <div className="shell page">
      <div className="page__head">
        <Link href="/tags" className="back-link">
          ← 全部标签
        </Link>
        <h1 className="page__title">
          <span
            className="tag-card__dot"
            aria-hidden="true"
            style={{
              display: 'inline-block',
              verticalAlign: 'middle',
              marginRight: 10,
              '--chip': tag.color,
            } as CSSProperties}
          />
          {tag.name}
        </h1>
        <p className="page__desc">
          共 {tag.post_count} 篇文章。
          {tag.description ? ` ${tag.description}` : ''}
        </p>
      </div>

      <div className="layout">
        <div>
          {posts.length > 0 ? (
            <div className="post-list">
              {posts.map((post, index) => (
                <PostCard key={post.id} post={post} index={index} />
              ))}
            </div>
          ) : (
            <div className="panel" style={{ textAlign: 'center', padding: '40px 20px' }}>
              <p style={{ margin: 0, fontWeight: 600 }}>这个标签下还没有文章</p>
            </div>
          )}
        </div>

        <aside className="sidebar">
          {allTags.length > 0 ? (
            <div className="panel">
              <h2 className="panel__title">其他标签</h2>
              <div className="tag-cloud">
                {allTags.map((item) => (
                  <TagChip key={item.id} tag={item} active={item.slug === tag.slug} showCount />
                ))}
              </div>
            </div>
          ) : null}
        </aside>
      </div>
    </div>
  );
}
