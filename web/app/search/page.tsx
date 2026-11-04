import type { Metadata } from 'next';
import Link from 'next/link';

import { PostCard } from '@/components/post-card';
import { getPosts } from '@/lib/api';
import { siteText, SITE_TEXT_KEYS } from '@/lib/site-text';
import type { Post } from '@/lib/types';
import { loadSite } from '../layout';

export const metadata: Metadata = {
  title: '搜索',
  description: '按关键词搜索文章。',
};

type SearchParams = Promise<{ q?: string }>;

export default async function SearchPage({ searchParams }: { searchParams: SearchParams }) {
  const sp = await searchParams;
  const keyword = sp.q?.trim() ?? '';
  // 搜索词里可以带 tag:xxx，标题里只展示真正用于高亮的关键词。
  const displayKeyword = keyword.replace(/(?:^|\s)tag:\S+/g, ' ').trim();
  const tagHint = /(?:^|\s)tag:(\S+)/.exec(keyword)?.[1] ?? '';

  let posts: Post[] = [];
  if (keyword) {
    try {
      posts = (await getPosts({ q: keyword, perPage: 30 })).items;
    } catch {
      posts = [];
    }
  }

  const site = await loadSite().catch(() => null);

  return (
    <div className="shell page">
      <div className="page__head">
        <h1 className="page__title">{siteText(site, SITE_TEXT_KEYS.searchTitle)}</h1>
        <p className="page__desc">
          {keyword
            ? `${displayKeyword ? `关键词「${displayKeyword}」` : ''}${
                tagHint ? `${displayKeyword ? ' · ' : ''}标签 #${tagHint}` : ''
              }，找到 ${posts.length} 篇文章。`
            : siteText(site, SITE_TEXT_KEYS.searchLead)}
        </p>
      </div>

      <form className="panel" style={{ display: 'flex', gap: 10, marginBottom: 26 }} action="/search" method="get">
        <input
          type="search"
          name="q"
          defaultValue={keyword}
          placeholder="例如：PostgreSQL、索引、Markdown"
          style={{
            flex: 1,
            height: 42,
            padding: '0 14px',
            borderRadius: 10,
            border: '1px solid var(--border)',
            background: 'var(--bg)',
            color: 'var(--fg)',
            fontSize: 15,
            outline: 'none',
          }}
        />
        <button type="submit" className="btn btn--primary">
          搜索
        </button>
      </form>
      <p className="page__desc" style={{ marginTop: -14, marginBottom: 26, fontSize: 13 }}>
        支持 <code>tag:标签</code> 语法，例如 <code>tag:go 缓存</code>。
      </p>

      {posts.length > 0 ? (
        <div className="post-list">
          {posts.map((post, index) => (
            <PostCard key={post.id} post={post} index={index} highlight={displayKeyword} />
          ))}
        </div>
      ) : (
        <div className="panel" style={{ textAlign: 'center', padding: '44px 20px' }}>
          <p style={{ margin: 0, fontWeight: 600 }}>
            {keyword ? '没有找到匹配的文章' : '还没有输入关键词'}
          </p>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
            {keyword ? '换个词试试，或者浏览全部文章。' : '试试搜索技术栈、标签名或者某个具体的概念。'}
          </p>
          <div className="state__actions">
            <Link href="/posts" className="btn btn--ghost">
              浏览全部文章
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
