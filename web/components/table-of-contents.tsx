'use client';

import { useEffect, useState } from 'react';

import type { TocItem } from '@/lib/toc';

/**
 * 文章目录，带滚动高亮。
 * 用 IntersectionObserver 而不是 scroll 事件，避免每帧都做 DOM 查询。
 */
export function TableOfContents({ items }: { items: TocItem[] }) {
  const [active, setActive] = useState<string>('');

  useEffect(() => {
    if (items.length === 0) return;

    const headings = items
      .map((item) => document.getElementById(item.id))
      .filter((el): el is HTMLElement => el !== null);

    if (headings.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]) {
          setActive(visible[0].target.id);
        }
      },
      // 顶部让出 84px（吸顶导航高度），底部收掉 70% 视口，保证「当前」只有一条
      { rootMargin: '-84px 0px -70% 0px', threshold: 0 },
    );

    headings.forEach((heading) => observer.observe(heading));
    return () => observer.disconnect();
  }, [items]);

  if (items.length === 0) return null;

  return (
    <nav className="toc" aria-label="文章目录">
      <div className="toc__title">本文目录</div>
      <ul className="toc__list">
        {items.map((item) => (
          <li key={item.id} className="toc__item" data-level={item.level}>
            <a href={`#${item.id}`} data-active={active === item.id}>
              {item.text}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
