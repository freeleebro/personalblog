import type { Metadata } from 'next';

import type { Site } from '@/lib/types';
import { loadSite } from '../layout';

export const metadata: Metadata = {
  title: '友链',
  description: '一些常去的站点。',
};

/**
 * 友链页。
 *
 * 数据来自站点设置里的 links，后台「站点设置」可以增删；
 * 站长没填时这里是空的，只保留结构——不预置任何别人的站。
 */
export default async function LinksPage() {
  const site: Site | null = await loadSite().catch(() => null);
  const links = site?.links ?? [];

  return (
    <div className="shell page">
      <div className="page__head">
        <h1 className="page__title">友链</h1>
        <p className="page__desc">
          {links.length > 0
            ? `一共 ${links.length} 个站点，都是我自己会点开的。`
            : '一些常去的站点。'}
        </p>
      </div>

      {links.length === 0 ? (
        <div className="panel" style={{ textAlign: 'center', padding: '40px 20px' }}>
          <p style={{ margin: 0, fontWeight: 600 }}>还没有友链</p>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
            这一页的架子已经搭好了，在后台「站点设置 → 友链」里加上就会自动显示。
          </p>
        </div>
      ) : (
        <ul className="link-grid">
          {links.map((link) => (
            <li key={link.url}>
              <a
                href={link.url}
                className="link-card"
                target="_blank"
                rel="noopener noreferrer"
              >
                <span className="link-card__avatar" aria-hidden="true">
                  {link.avatar ? (
                    // 头像是外部图片，尺寸不固定，交给 CSS 裁成圆形。
                    <img src={link.avatar} alt="" loading="lazy" decoding="async" />
                  ) : (
                    link.name.slice(0, 1)
                  )}
                </span>
                <span className="link-card__body">
                  <span className="link-card__name">{link.name}</span>
                  {link.description ? (
                    <span className="link-card__desc">{link.description}</span>
                  ) : null}
                  <span className="link-card__url">{link.url.replace(/^https?:\/\//, '')}</span>
                </span>
              </a>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
