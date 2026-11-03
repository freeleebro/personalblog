import type { MetadataRoute } from 'next';

import { getPosts, getTags } from '@/lib/api';
import { loadSite } from './layout';

export const dynamic = 'force-dynamic';

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const site = await loadSite();
  const base = (site?.url ?? 'http://localhost:3000').replace(/\/+$/, '');

  const staticEntries: MetadataRoute.Sitemap = [
    { url: `${base}/`, changeFrequency: 'daily', priority: 1 },
    { url: `${base}/posts`, changeFrequency: 'daily', priority: 0.9 },
    { url: `${base}/tags`, changeFrequency: 'weekly', priority: 0.7 },
    { url: `${base}/archive`, changeFrequency: 'weekly', priority: 0.6 },
    { url: `${base}/about`, changeFrequency: 'monthly', priority: 0.5 },
  ];

  try {
    const [{ items: posts }, tags] = await Promise.all([getPosts({ perPage: 50 }), getTags()]);

    return [
      ...staticEntries,
      ...posts.map((post) => ({
        url: `${base}/posts/${encodeURIComponent(post.slug)}`,
        lastModified: new Date(post.updated_at),
        changeFrequency: 'monthly' as const,
        priority: 0.8,
      })),
      ...tags.map((tag) => ({
        url: `${base}/tags/${encodeURIComponent(tag.slug)}`,
        changeFrequency: 'weekly' as const,
        priority: 0.5,
      })),
    ];
  } catch {
    // 后端不可用时至少给出静态页面列表，不要整个 sitemap 报错
    return staticEntries;
  }
}
