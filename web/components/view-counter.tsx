'use client';

import { useEffect, useState } from 'react';

import { IconEye } from './icons';
import { compactNumber } from '@/lib/format';

/**
 * 阅读量。首次进入文章时通过同源路由代理自增一次，
 * 服务端已做按 IP 的限流，这里再用 sessionStorage 避免同一次会话重复计数。
 */
export function ViewCounter({ slug, initial }: { slug: string; initial: number }) {
  const [views, setViews] = useState(initial);

  useEffect(() => {
    const key = `viewed:${slug}`;
    try {
      if (sessionStorage.getItem(key)) return;
      sessionStorage.setItem(key, '1');
    } catch {
      // 隐私模式下 sessionStorage 可能不可用，忽略即可
    }

    let cancelled = false;
    fetch(`/api/views/${encodeURIComponent(slug)}`, { method: 'POST' })
      .then((res) => (res.ok ? res.json() : null))
      .then((json) => {
        if (!cancelled && json?.data?.views) setViews(json.data.views);
      })
      .catch(() => {
        /* 计数失败不影响阅读 */
      });

    return () => {
      cancelled = true;
    };
  }, [slug]);

  return (
    <span className="article-meta__item">
      <IconEye width={14} height={14} />
      {compactNumber(views)} 次阅读
    </span>
  );
}
