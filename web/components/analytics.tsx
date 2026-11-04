'use client';

import { usePathname } from 'next/navigation';
import { useEffect, useRef } from 'react';

/**
 * 访问统计信标。
 *
 * 只上报「路径 + 来源站点」，不带 cookie、不带 UA 解析结果；
 * 服务端用「IP + UA + 日期 + 密钥」算一个按天失效的访客哈希做 UV。
 * 后台和草稿预览不上报，避免自己的访问污染数据。
 */
export function Analytics() {
  const pathname = usePathname();
  const last = useRef<string | null>(null);

  useEffect(() => {
    if (!pathname) return;
    if (pathname.startsWith('/admin') || pathname.startsWith('/preview')) return;
    // 开发环境的数据没有参考价值，只在生产上报。
    if (process.env.NODE_ENV !== 'production') return;
    if (last.current === pathname) return;
    last.current = pathname;

    const payload = JSON.stringify({
      path: pathname,
      referrer: document.referrer || '',
    });

    // sendBeacon 在页面关闭时也能发出去；不支持时退回 fetch keepalive。
    if (navigator.sendBeacon) {
      const blob = new Blob([payload], { type: 'application/json' });
      if (navigator.sendBeacon('/api-proxy/api/v1/collect', blob)) return;
    }
    void fetch('/api-proxy/api/v1/collect', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: payload,
      keepalive: true,
    }).catch(() => {
      // 统计失败不影响页面，静默忽略。
    });
  }, [pathname]);

  return null;
}