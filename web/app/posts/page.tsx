import type { Metadata } from 'next';
import Link from 'next/link';

import { Pagination } from '@/components/pagination';
import { PostCard } from '@/components/post-card';
import { TagChip } from '@/components/tag-chip';
import { getPosts, getTags } from '@/lib/api';
import { siteText, SITE_TEXT_KEYS } from '@/lib/site-text';
import type { Post, Tag } from '@/lib/types';
import { loadSite } from '../layout';

export const metadata: Metadata = {
  title: '全部文章',
  description: '按时间倒序浏览全部文章。',
};

type SearchParams = Promise<{ tag?: string; page?: string }>;

const PER_PAGE = 8;

export default async function PostsPage({ searchParams }: { searchParams: SearchParams }) {
  const sp = await searchParams;
  const page = Math.max(1, Number(sp.page ?? '1') || 1);
  const tagSlug = sp.tag?.trim() || undefined;

  const [postsResult, tagsResult] = await Promise.allSettled([
    getPosts({ tag: tagSlug, page, perPage: PER_PAGE }),
    getTags(),
  ]);
  const site = await loadSite().catch(() => null);
  const lead = site?.texts?.[SITE_TEXT_KEYS.postsLead]?.trim() ?? '';

  const posts: Post[] = postsResult.status === 'fulfilled' ? postsResult.value.items : [];
  const meta =
    postsResult.status === 'fulfilled'
      ? postsResult.value.meta
      : { page: 1, total: 0, total_pages: 1, per_page: PER_PAGE };
  const tags: Tag[] = tagsResult.status === 'fulfilled' ? tagsResult.value : [];

  const activeTag = tagSlug ? tags.find((t) => t.slug === tagSlug) : undefined;
  const totalPages = meta.total_pages ?? 1;

  const buildHref = (target: number) => {
    const params = new URLSearchParams();
    if (tagSlug) params.set('tag', tagSlug);
    if (target > 1) params.set('page', String(target));
    const qs = params.toString();
    return qs ? `/posts?${qs}` : '/posts';
  };

  return (
    <div className="shell page">
      <div className="page__head">
        <h1 className="page__title">
          {activeTag ? `标签：${activeTag.name}` : siteText(site, SITE_TEXT_KEYS.postsTitle)}
        </h1>
        <p className="page__desc">
          {lead || (
            <>
              共 {meta.total ?? posts.length} 篇
              {tagSlug ? '，已按标签筛选。' : '，按发布时间倒序。'}
            </>
          )}
        </p>
      </div>

      <div className="layout">
        <div>
          {posts.length === 0 ? (
            <div className="panel" style={{ textAlign: 'center', padding: '40px 20px' }}>
              <p style={{ margin: 0, fontWeight: 600 }}>{siteText(site, SITE_TEXT_KEYS.postsEmpty)}</p>
              <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
                {tagSlug ? '换个标签看看，或者浏览全部文章。' : '后端服务可能还没启动。'}
              </p>
              {tagSlug ? (
                <Link href="/posts" className="btn btn--ghost" style={{ marginTop: 16 }}>
                  查看全部文章
                </Link>
              ) : null}
            </div>
          ) : (
            <div className="post-list">
              {posts.map((post, index) => (
                <PostCard key={post.id} post={post} index={(page - 1) * PER_PAGE + index} />
              ))}
            </div>
          )}

          <Pagination page={page} totalPages={totalPages} buildHref={buildHref} />
        </div>

        <aside className="sidebar">
          {tags.length > 0 ? (
            <div className="panel">
              <h2 className="panel__title">按标签筛选</h2>
              <div className="tag-cloud">
                {tags.map((tag) => (
                  <TagChip key={tag.id} tag={tag} active={tag.slug === tagSlug} showCount />
                ))}
              </div>
              {tagSlug ? (
                <Link
                  href="/posts"
                  className="section-head__more"
                  style={{ display: 'inline-block', marginTop: 14 }}
                >
                  清除筛选
                </Link>
              ) : null}
            </div>
          ) : null}

          <div className="panel">
            <h2 className="panel__title">换个方式看</h2>
            <ul className="sidebar-list">
              <li className="sidebar-list__item">
                <Link href="/archive" className="sidebar-list__title">
                  按时间归档
                </Link>
              </li>
              <li className="sidebar-list__item">
                <Link href="/tags" className="sidebar-list__title">
                  全部标签
                </Link>
              </li>
              <li className="sidebar-list__item">
                <a href="/feed.xml" className="sidebar-list__title">
                  订阅 RSS
                </a>
              </li>
            </ul>
          </div>
        </aside>
      </div>
    </div>
  );
}
