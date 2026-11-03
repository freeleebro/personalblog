import Link from 'next/link';

import { siteText, SITE_TEXT_KEYS } from '@/lib/site-text';
import { loadSite } from './layout';

export default async function NotFound() {
  // 站点读不到也要能出 404 页，所以这里吞掉错误用默认文案。
  const site = await loadSite().catch(() => null);

  return (
    <div className="shell">
      <div className="state">
        <div className="state__code">404</div>
        <h1 className="state__title">{siteText(site, SITE_TEXT_KEYS.notFoundTitle)}</h1>
        <p className="state__desc">{siteText(site, SITE_TEXT_KEYS.notFoundDesc)}</p>
        <div className="state__actions">
          <Link href="/" className="btn btn--primary">
            回到首页
          </Link>
          <Link href="/posts" className="btn btn--ghost">
            浏览全部文章
          </Link>
        </div>
      </div>
    </div>
  );
}
