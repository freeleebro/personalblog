import { NextResponse } from 'next/server';

import { API_BASE } from '@/lib/api';

/**
 * 把 Go 后端的 RSS 透传到站点自身的域名下。
 * 这样订阅地址是稳定的 /feed.xml，也不必把后端地址暴露给浏览器。
 */
export const dynamic = 'force-dynamic';

export async function GET() {
  try {
    const res = await fetch(`${API_BASE}/api/v1/feed.xml`, { cache: 'no-store' });
    if (!res.ok) {
      return new NextResponse('订阅源暂时不可用', { status: 502 });
    }
    const body = await res.text();
    return new NextResponse(body, {
      headers: {
        'Content-Type': 'application/rss+xml; charset=utf-8',
        'Cache-Control': 'public, max-age=600',
      },
    });
  } catch {
    return new NextResponse('订阅源暂时不可用', { status: 502 });
  }
}
