import type { Metadata } from 'next';
import Link from 'next/link';

import { Markdown } from '@/components/markdown';
import { getPage } from '@/lib/api';
import { formatDate } from '@/lib/format';
import { loadSite } from '../layout';

export const metadata: Metadata = {
  title: '关于',
  description: '关于这个博客，以及关于我。',
};

export default async function AboutPage() {
  const [pageResult, site] = await Promise.allSettled([getPage('about'), loadSite()]);

  const page = pageResult.status === 'fulfilled' ? pageResult.value : null;
  const siteData = site.status === 'fulfilled' ? site.value : null;

  return (
    <div className="shell page">
      <div className="layout">
        <div>
          <div className="page__head">
            <h1 className="page__title">{page?.title ?? '关于'}</h1>
            {page ? (
              <p className="page__desc">最近更新于 {formatDate(page.updated_at)}</p>
            ) : null}
          </div>

          {page ? (
            <Markdown>{page.body_md}</Markdown>
          ) : (
            <div className="panel" style={{ textAlign: 'center', padding: '40px 20px' }}>
              <p style={{ margin: 0, fontWeight: 600 }}>读不到「关于」页面</p>
              <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
                后端服务可能还没启动。
              </p>
              <Link href="/" className="btn btn--ghost" style={{ marginTop: 16 }}>
                回到首页
              </Link>
            </div>
          )}
        </div>

        <aside className="sidebar">
          {siteData ? (
            <div className="panel">
              <h2 className="panel__title">站点</h2>
              <ul className="sidebar-list">
                <li className="sidebar-list__item">
                  <span className="sidebar-list__title" style={{ fontWeight: 600 }}>
                    {siteData.name}
                  </span>
                  <span className="sidebar-list__date">{siteData.tagline}</span>
                </li>
                <li className="sidebar-list__item">
                  <span className="sidebar-list__title">
                    {siteData.stats.posts} 篇文章 · {siteData.stats.tags} 个标签
                  </span>
                  <span className="sidebar-list__date">
                    最近更新 {formatDate(siteData.stats.last_published_at)}
                  </span>
                </li>
              </ul>

              {siteData.socials && siteData.socials.length > 0 ? (
                <>
                  <h2 className="panel__title" style={{ marginTop: 20 }}>
                    联系
                  </h2>
                  <ul className="sidebar-list">
                    {siteData.socials.map((social) => (
                      <li key={social.label} className="sidebar-list__item">
                        <a
                          href={social.url}
                          className="sidebar-list__title"
                          {...(/^https?:/.test(social.url)
                            ? { target: '_blank', rel: 'noopener noreferrer' }
                            : {})}
                        >
                          {social.label}
                        </a>
                      </li>
                    ))}
                  </ul>
                </>
              ) : null}
            </div>
          ) : null}

          <div className="panel">
            <h2 className="panel__title">技术栈</h2>
            <ul className="sidebar-list">
              <li className="sidebar-list__item">
                <span className="sidebar-list__title">Next.js（App Router）</span>
                <span className="sidebar-list__date">服务端渲染与 Markdown 渲染</span>
              </li>
              <li className="sidebar-list__item">
                <span className="sidebar-list__title">Go + net/http</span>
                <span className="sidebar-list__date">JSON API、RSS、Sitemap</span>
              </li>
              <li className="sidebar-list__item">
                <span className="sidebar-list__title">PostgreSQL</span>
                <span className="sidebar-list__date">pgx 连接池、pg_trgm 模糊搜索</span>
              </li>
            </ul>
          </div>
        </aside>
      </div>
    </div>
  );
}
