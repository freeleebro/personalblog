import type { Metadata } from 'next';
import type { ReactNode } from 'react';

import './globals.css';

import { MobileNav } from '@/components/nav-links';
import { Particles } from '@/components/particles';
import { BackToTop } from '@/components/reading-progress';
import { SiteFooter } from '@/components/site-footer';
import { SiteHeader } from '@/components/site-header';
import { getSite } from '@/lib/api';
import type { Site } from '@/lib/types';

// 全站使用 SSR：内容随时可能变，且后端查询极快，不做静态化更简单也更可靠。
export const dynamic = 'force-dynamic';

// 后端不可用或还没初始化时的兜底文案；首次部署后请到后台「站点设置」里改成自己的。
const FALLBACK = {
  name: '我的博客',
  tagline: '记录，然后分享',
  description: '一个用 Next.js、Go 与 PostgreSQL 搭起来的个人博客。',
  url: 'http://localhost:3000',
};

// 在 <head> 里同步执行，把主题写到 <html data-theme>，避免首屏白闪。
const THEME_INIT = `(function(){try{var t=localStorage.getItem('theme');if(t!=='light'&&t!=='dark'){t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}document.documentElement.setAttribute('data-theme',t);}catch(e){document.documentElement.setAttribute('data-theme','light');}})();`;

/** 后端不可用时也要能渲染出页面框架，所以这里吞掉异常。 */
export async function loadSite(): Promise<Site | null> {
  try {
    return await getSite();
  } catch {
    return null;
  }
}

export async function generateMetadata(): Promise<Metadata> {
  const site = await loadSite();
  const name = site?.name ?? FALLBACK.name;
  const tagline = site?.tagline ?? FALLBACK.tagline;
  const description = site?.description ?? FALLBACK.description;

  return {
    title: { default: `${name} · ${tagline}`, template: `%s · ${name}` },
    description,
    metadataBase: resolveBaseUrl(site),
    icons: {
      icon: [{ url: '/icon.svg', type: 'image/svg+xml' }],
      shortcut: '/icon.svg',
    },
    openGraph: {
      type: 'website',
      siteName: name,
      title: `${name} · ${tagline}`,
      description,
      locale: 'zh_CN',
      images: [
        {
          url: '/og-image.jpg',
          width: 1200,
          height: 630,
          alt: `${name} · ${tagline}`,
        },
      ],
    },
    twitter: {
      card: 'summary_large_image',
      title: `${name} · ${tagline}`,
      description,
      images: ['/og-image.jpg'],
    },
    alternates: {
      types: { 'application/rss+xml': '/feed.xml' },
    },
  };
}

/**
 * 解析站点根 URL。
 * 优先级：后台「站点设置」→ 环境变量 SITE_URL → 开发用 localhost。
 * 生产环境如果最终仍指向 localhost，说明配置漏了——这里只告警不抛错，
 * 因为抛错会让整站 500，代价远大于 canonical 暂时不准。
 */
function resolveBaseUrl(site: Site | null): URL {
  const candidate = site?.url?.trim() || process.env.SITE_URL?.trim() || '';

  if (candidate) {
    try {
      const url = new URL(candidate);
      const isLocal = ['localhost', '127.0.0.1', '0.0.0.0'].includes(url.hostname);
      if (isLocal && process.env.NODE_ENV === 'production') {
        console.error(
          `[metadata] 站点地址仍是 ${url.origin}，生产环境下 canonical / OG 会指错。` +
            '请在后台「站点设置」或环境变量 SITE_URL 中填写真实域名。',
        );
      }
      return url;
    } catch {
      console.error(`[metadata] 站点地址 ${candidate} 不是合法 URL，已回退到 ${FALLBACK.url}`);
    }
  }

  return new URL(process.env.SITE_URL?.trim() || FALLBACK.url);
}

export default async function RootLayout({ children }: { children: ReactNode }) {
  const site = await loadSite();
  const name = site?.name ?? FALLBACK.name;

  return (
    <html lang="zh-CN" data-theme="light" suppressHydrationWarning>
      <body>
        {/* 同步执行，在首屏绘制前把主题写到 <html data-theme>，避免白闪 */}
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT }} />
        <Particles />
        <SiteHeader siteName={name} />
        <main>{children}</main>
        <SiteFooter site={site} />
        <MobileNav />
        <BackToTop />
      </body>
    </html>
  );
}
