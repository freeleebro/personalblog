import type { Metadata } from 'next';
import Link from 'next/link';
import type { CSSProperties } from 'react';

import { getArchive, getPosts } from '@/lib/api';
import { formatDateShort, isoDate } from '@/lib/format';
import { siteText, SITE_TEXT_KEYS } from '@/lib/site-text';
import type { ArchiveYear, Post } from '@/lib/types';
import { loadSite } from '../layout';

export const metadata: Metadata = {
  title: '归档',
  description: '按年份浏览全部文章。',
};

export default async function ArchivePage() {
  const [postsResult, yearsResult] = await Promise.allSettled([
    getPosts({ perPage: 50 }),
    getArchive(),
  ]);
  const site = await loadSite().catch(() => null);

  const posts: Post[] = postsResult.status === 'fulfilled' ? postsResult.value.items : [];
  const years: ArchiveYear[] = yearsResult.status === 'fulfilled' ? yearsResult.value : [];

  // 按年份分组，年份内部已由后端按时间倒序返回
  const grouped = new Map<number, Post[]>();
  for (const post of posts) {
    const year = post.published_at ? new Date(post.published_at).getFullYear() : 0;
    const bucket = grouped.get(year);
    if (bucket) {
      bucket.push(post);
    } else {
      grouped.set(year, [post]);
    }
  }
  const sortedYears = [...grouped.keys()].sort((a, b) => b - a);

  return (
    <div className="shell page">
      <div className="page__head">
        <h1 className="page__title">{siteText(site, SITE_TEXT_KEYS.archiveTitle)}</h1>
        <p className="page__desc">
          {site?.texts?.[SITE_TEXT_KEYS.archiveLead]?.trim() || (
            <>
              共 {posts.length} 篇文章，跨越 {sortedYears.length || 0} 个年份。
            </>
          )}
        </p>
      </div>

      {years.length > 0 ? (
        <div className="panel" style={{ marginBottom: 30 }}>
          <h2 className="panel__title">按年速览</h2>
          <div className="tag-cloud">
            {years.map((item) => (
              <a
                key={item.year}
                href={`#year-${item.year}`}
                className="chip"
                style={{ '--chip': 'var(--accent)' } as CSSProperties}
              >
                {item.year}
                <span className="chip__count">{item.count}</span>
              </a>
            ))}
          </div>
        </div>
      ) : null}

      {sortedYears.length === 0 ? (
        <div className="panel" style={{ textAlign: 'center', padding: '40px 20px' }}>
          <p style={{ margin: 0, fontWeight: 600 }}>还没有可归档的文章</p>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
            后端服务可能还没启动。
          </p>
        </div>
      ) : (
        <div className="layout">
          <div>
            {sortedYears.map((year) => {
              const items = grouped.get(year) ?? [];
              return (
                <section key={year} className="archive-year" id={`year-${year}`}>
                  <div className="archive-year__head">
                    <span className="archive-year__num">{year || '未标注'}</span>
                    <span className="archive-year__count">{items.length} 篇</span>
                  </div>

                  <ul className="archive-list">
                    {items.map((post) => (
                      <li key={post.id} className="archive-list__item">
                        <time className="archive-list__date" dateTime={isoDate(post.published_at)}>
                          {formatDateShort(post.published_at)}
                        </time>
                        <Link
                          href={`/posts/${encodeURIComponent(post.slug)}`}
                          className="archive-list__title"
                        >
                          {post.title}
                        </Link>
                        <span className="archive-list__tags">
                          {post.tags.slice(0, 2).map((tag) => (
                            <span
                              key={tag.id}
                              className="chip"
                              style={{ '--chip': tag.color } as CSSProperties}
                            >
                              {tag.name}
                            </span>
                          ))}
                        </span>
                      </li>
                    ))}
                  </ul>
                </section>
              );
            })}
          </div>

          <aside className="sidebar">
            <div className="panel">
              <h2 className="panel__title">月度分布</h2>
              <ul className="sidebar-list">
                {years.slice(0, 8).map((item) => (
                  <li key={item.year} className="sidebar-list__item">
                    <span className="sidebar-list__title" style={{ fontWeight: 600 }}>
                      {item.year} 年 · {item.count} 篇
                    </span>
                    <span className="sidebar-list__date">
                      {item.months.map((m) => `${m.month}月(${m.count})`).join(' ')}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          </aside>
        </div>
      )}
    </div>
  );
}
