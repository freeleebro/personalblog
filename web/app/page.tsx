import Link from 'next/link';

import { FeaturedPost } from '@/components/featured-post';
import { PostCard } from '@/components/post-card';
import { TagChip } from '@/components/tag-chip';
import { IconArrowRight, IconRss } from '@/components/icons';
import { getPosts } from '@/lib/api';
import { formatChars, formatDate } from '@/lib/format';
import { SITE_TEXT_KEYS, siteFlag, siteText } from '@/lib/site-text';
import type { Post, Site } from '@/lib/types';
import { loadSite } from './layout';

export default async function HomePage() {
  // 用 allSettled：即使后端挂了，首屏框架依然要能渲染出来。
  const [siteResult, postsResult] = await Promise.allSettled([
    loadSite(),
    getPosts({ perPage: 9 }),
  ]);

  const site: Site | null = siteResult.status === 'fulfilled' ? siteResult.value : null;
  let posts: Post[] = postsResult.status === 'fulfilled' ? postsResult.value.items : [];

  const featured = site?.featured ?? posts.find((p) => p.featured) ?? null;
  if (featured) {
    posts = posts.filter((p) => p.slug !== featured.slug);
  }

  const stats = site?.stats;
  const offline = posts.length === 0 && !featured;

  // 这几段文案都可以在后台「站点设置」里改，代码里只留中性的兜底。
  const heroVisualTitle = siteText(site, SITE_TEXT_KEYS.heroVisualTitle);
  const heroVisualMeta = siteText(site, SITE_TEXT_KEYS.heroVisualMeta);
  const notesEnabled = siteFlag(site, SITE_TEXT_KEYS.notesEnabled);
  const notesTitle = siteText(site, SITE_TEXT_KEYS.notesTitle);
  const notesLead = siteText(site, SITE_TEXT_KEYS.notesLead);
  const galleryAlt = notesTitle || '首页配图';

  return (
    <>
      <section className="hero">
        <div className="shell hero__inner">
          <div className="hero__copy">
            <h1 className="hero__title">{site?.tagline ?? '代码、阅读与生活'}</h1>

            <p className="hero__subtitle">
              {site?.description
                ? `个人博客 · ${site.description}`
                : '个人博客，记录代码、阅读与生活，用 Next.js、Go 与 PostgreSQL 搭起来。'}
            </p>

            {stats ? (
              <div className="hero__stats">
                <span className="hero__stat">
                  共 <strong>{stats.posts}</strong> 篇文章
                </span>
                <span className="hero__stat">
                  <strong>{stats.tags}</strong> 个标签
                </span>
                <span className="hero__stat">
                  累计 <strong>{formatChars(stats.chars)}</strong>
                </span>
                <span className="hero__stat">
                  最近更新 <strong>{formatDate(stats.last_published_at)}</strong>
                </span>
              </div>
            ) : null}

            <div className="hero__actions">
              <Link href="/posts" className="btn btn--primary">
                浏览全部文章
                <IconArrowRight width={15} height={15} />
              </Link>
              <Link href="/about" className="btn btn--ghost">
                关于我
              </Link>
              <a href="/feed.xml" className="btn btn--ghost">
                <IconRss width={15} height={15} />
                RSS
              </a>
            </div>
          </div>

          <div className="hero__visual" role="img" aria-label={galleryAlt}>
            <div className="hero__visual-grid" aria-hidden="true" />
            <div className="hero__visual-copy">
              <div>
                {heroVisualTitle ? <div className="hero__visual-title">{heroVisualTitle}</div> : null}
              </div>
              {heroVisualMeta ? <span className="hero__visual-meta">{heroVisualMeta}</span> : null}
            </div>
          </div>
        </div>
      </section>

      {notesEnabled ? (
        <section className="visual-notes" aria-labelledby="visual-notes-title">
          <div className="shell">
            <div className="visual-notes__head">
              <h2 id="visual-notes-title">{notesTitle}</h2>
              {notesLead ? <p>{notesLead}</p> : null}
            </div>
            <div className="visual-notes__grid">
              <figure>
                <img
                  src="/anime-night-640.webp"
                  alt={`${galleryAlt} 1`}
                  width={640}
                  height={640}
                  decoding="async"
                />
              </figure>
              <figure>
                <img
                  src="/anime-underground-800.webp"
                  alt={`${galleryAlt} 2`}
                  width={750}
                  height={500}
                  loading="lazy"
                  decoding="async"
                />
              </figure>
              <figure>
                <img
                  src="/anime-crossing-800.webp"
                  alt={`${galleryAlt} 3`}
                  width={800}
                  height={525}
                  loading="lazy"
                  decoding="async"
                />
              </figure>
            </div>
          </div>
        </section>
      ) : null}

      <div className="shell page">
        <div className="layout">
          <div>
            {featured ? <FeaturedPost post={featured} /> : null}

            <div className="section-head">
              <h2 className="section-head__title">最新文章</h2>
              <Link href="/posts" className="section-head__more">
                全部文章 →
              </Link>
            </div>

            {offline ? (
              <div className="panel" style={{ textAlign: 'center', padding: '36px 20px' }}>
                <p style={{ margin: 0, fontWeight: 600 }}>暂时读不到文章</p>
                <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
                  后端服务可能还没启动。执行 <code>docker compose up -d</code> 之后再刷新试试。
                </p>
              </div>
            ) : (
              <div className="post-list">
                {posts.map((post, index) => (
                  <PostCard key={post.id} post={post} index={index} />
                ))}
              </div>
            )}
          </div>

          <aside className="sidebar">
            {site?.author ? (
              <div className="panel">
                <div className="author-card__top">
                  <span className="avatar" aria-hidden="true">
                    {site.author.name.slice(0, 1)}
                  </span>
                  <div>
                    <div className="author-card__name">{site.author.name}</div>
                    <div className="author-card__role">作者</div>
                  </div>
                </div>
                <p className="author-card__bio">{site.author.bio}</p>
                <Link href="/about" className="section-head__more" style={{ display: 'inline-block', marginTop: 12 }}>
                  了解更多 →
                </Link>
              </div>
            ) : null}

            {site && site.tags.length > 0 ? (
              <div className="panel">
                <h3 className="panel__title">标签</h3>
                <div className="tag-cloud">
                  {site.tags.slice(0, 12).map((tag) => (
                    <TagChip key={tag.id} tag={tag} showCount />
                  ))}
                </div>
              </div>
            ) : null}

            {site && site.recent.length > 0 ? (
              <div className="panel">
                <h3 className="panel__title">最近更新</h3>
                <ul className="sidebar-list">
                  {site.recent.map((post) => (
                    <li key={post.id} className="sidebar-list__item">
                      <Link href={`/posts/${encodeURIComponent(post.slug)}`} className="sidebar-list__title">
                        {post.title}
                      </Link>
                      <span className="sidebar-list__date">{formatDate(post.published_at)}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </aside>
        </div>
      </div>
    </>
  );
}
