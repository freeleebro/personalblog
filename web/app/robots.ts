import type { MetadataRoute } from 'next';

import { loadSite } from './layout';

export const dynamic = 'force-dynamic';

export default async function robots(): Promise<MetadataRoute.Robots> {
  const site = await loadSite();
  const base = (site?.url ?? 'http://localhost:3000').replace(/\/+$/, '');

  return {
    rules: [
      // 预览链接是给别人看草稿用的，不该出现在搜索结果里。
      { userAgent: '*', allow: '/', disallow: ['/preview/', '/admin', '/api-proxy/'] },
    ],
    sitemap: `${base}/sitemap.xml`,
  };
}
