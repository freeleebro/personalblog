import type { Metadata } from 'next';
import Link from 'next/link';
import type { CSSProperties } from 'react';

import { TagChip } from '@/components/tag-chip';
import { getTags } from '@/lib/api';
import { siteText, SITE_TEXT_KEYS } from '@/lib/site-text';
import type { Tag } from '@/lib/types';
import { loadSite } from '../layout';

export const metadata: Metadata = {
  title: '标签',
  description: '按主题浏览全部标签。',
};

export default async function TagsPage() {
  let tags: Tag[] = [];
  try {
    tags = await getTags();
  } catch {
    tags = [];
  }

  const total = tags.reduce((sum, tag) => sum + tag.post_count, 0);
  const site = await loadSite().catch(() => null);

  return (
    <div className="shell page">
      <div className="page__head">
        <h1 className="page__title">{siteText(site, SITE_TEXT_KEYS.tagsTitle)}</h1>
        <p className="page__desc">
          {site?.texts?.[SITE_TEXT_KEYS.tagsLead]?.trim() || (
            <>
              {tags.length} 个标签，共 {total} 次引用。点进去可以看这个主题下的全部文章。
            </>
          )}
        </p>
      </div>

      {tags.length === 0 ? (
        <div className="panel" style={{ textAlign: 'center', padding: '40px 20px' }}>
          <p style={{ margin: 0, fontWeight: 600 }}>还没有标签</p>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
            后端服务可能还没启动。
          </p>
        </div>
      ) : (
        <>
          <div className="panel" style={{ marginBottom: 26 }}>
            <h2 className="panel__title">全部标签</h2>
            <div className="tag-cloud">
              {tags.map((tag) => (
                <TagChip key={tag.id} tag={tag} showCount />
              ))}
            </div>
          </div>

          <div className="tag-grid">
            {tags.map((tag) => (
              <Link
                key={tag.id}
                href={`/tags/${encodeURIComponent(tag.slug)}`}
                className="tag-card"
                style={{ '--chip': tag.color } as CSSProperties}
              >
                <span className="tag-card__name">
                  <span className="tag-card__dot" aria-hidden="true" />
                  {tag.name}
                </span>
                <span className="tag-card__count">{tag.post_count} 篇</span>
              </Link>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
