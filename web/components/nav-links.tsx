'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

import {
  IconArchive,
  IconHome,
  IconList,
  IconTagIcon,
  IconUser,
} from './icons';

export const NAV_ITEMS = [
  { href: '/', label: '首页' },
  { href: '/posts', label: '文章' },
  { href: '/tags', label: '标签' },
  { href: '/archive', label: '归档' },
  { href: '/about', label: '关于' },
];

const MOBILE_ICONS: Record<string, typeof IconHome> = {
  '/': IconHome,
  '/posts': IconList,
  '/tags': IconTagIcon,
  '/archive': IconArchive,
  '/about': IconUser,
};

function isActive(pathname: string, href: string): boolean {
  if (href === '/') return pathname === '/';
  return pathname === href || pathname.startsWith(`${href}/`);
}

/** 顶部主导航，用 pathname 判断高亮。 */
export function NavLinks() {
  const pathname = usePathname();

  return (
    <nav className="nav" aria-label="主导航">
      {NAV_ITEMS.map((item) => (
        <Link
          key={item.href}
          href={item.href}
          className="nav__link"
          data-active={isActive(pathname, item.href)}
        >
          {item.label}
        </Link>
      ))}
    </nav>
  );
}

/** 移动端底部导航。 */
export function MobileNav() {
  const pathname = usePathname();
  const iconFor = (href: string) => href;

  return (
    <nav className="mobile-nav" aria-label="移动端导航">
      {NAV_ITEMS.map((item) => (
        <Link
          key={item.href}
          href={item.href}
          className="mobile-nav__item"
          data-active={isActive(pathname, item.href)}
        >
          <span aria-hidden="true" data-icon={iconFor(item.href)} />
          {item.label}
        </Link>
      ))}
    </nav>
  );
}
