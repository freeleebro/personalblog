import Link from 'next/link';

import type { Site } from '@/lib/types';

export function SiteFooter({ site }: { site: Site | null }) {
  const name = site?.name ?? '';
  const socials = site?.socials ?? [];
  const year = new Date().getFullYear();

  return (
    <footer className="site-footer">
      <div className="shell site-footer__inner">
        <span>
          © {year} {name}
        </span>

        <span className="site-footer__links">
          {socials.map((social) => (
            <a
              key={social.label}
              href={social.url}
              {...(/^https?:/.test(social.url) ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
            >
              {social.label}
            </a>
          ))}
          <Link href="/about">关于</Link>
        </span>
      </div>
    </footer>
  );
}
