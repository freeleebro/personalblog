import Link from 'next/link';

import { NavLinks } from './nav-links';
import { SearchDialog } from './search-dialog';
import { ThemeToggle } from './theme-toggle';

/** 站点顶栏。siteName 由 layout 从后端取，失败时回退成默认值。 */
export function SiteHeader({ siteName }: { siteName: string }) {
  const initial = siteName.slice(0, 1) || '北';

  return (
    <header className="site-header">
      <div className="shell site-header__inner">
        <Link href="/" className="brand" aria-label={`${siteName} 首页`}>
          <span className="brand__mark" aria-hidden="true">
            {initial}
          </span>
          <span className="brand__name">{siteName}</span>
        </Link>

        <NavLinks />

        <div className="header-actions">
          <SearchDialog />
          <Link href="/admin" className="admin-entry" title="管理后台">
            <span className="admin-entry__text">管理</span>
          </Link>
          <ThemeToggle />
        </div>
      </div>
    </header>
  );
}
