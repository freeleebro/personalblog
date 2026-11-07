import type { Metadata } from 'next';
import Link from 'next/link';

import { getSeriesList } from '@/lib/api';
import { siteText, SITE_TEXT_KEYS } from '@/lib/site-text';
import type { Series } from '@/lib/types';
import { loadSite } from '../layout';

export const metadata: Metadata = {
  title: '系列',
  description: '把连载和长文按顺序串起来的系列合集。',
};

export default async function SeriesIndexPage() {
  let series: Series[] = [];
  try {
    series = await getSeriesList();
  } catch {
    series = [];
  }

  const site = await loadSite().catch(() => null);
  const lead = siteText(site, SITE_TEXT_KEYS.seriesLead);

  return (
    <div className="shell page">
      <div className="page__head">
        <h1 className="page__title">{siteText(site, SITE_TEXT_KEYS.seriesTitle)}</h1>
        {lead ? <p className="page__desc">{lead}</p> : null}
      </div>

      {series.length === 0 ? (
        <div className="panel" style={{ textAlign: 'center', padding: '40px 20px' }}>
          <p style={{ margin: 0, fontWeight: 600 }}>还没有系列</p>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
            在后台「系列」里建一个，把相关的几篇文章按顺序放进去。
          </p>
        </div>
      ) : (
        <div className="series-grid">
          {series.map((item) => (
            <Link
              key={item.id}
              href={`/series/${encodeURIComponent(item.slug)}`}
              className="series-card"
            >
              <span className="series-card__badge">系列</span>
              <h2 className="series-card__title">{item.title}</h2>
              {item.description ? (
                <p className="series-card__desc">{item.description}</p>
              ) : (
                <p className="series-card__desc series-card__desc--muted">这个合集还没有写简介。</p>
              )}
              <span className="series-card__meta">{item.post_count} 篇</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
