import { NextResponse } from 'next/server';

import { API_BASE } from '@/lib/api';

/**
 * 搜索代理。
 * 客户端不直连 Go API：既避免了跨域配置，也不会把后端地址暴露到浏览器。
 */
export const dynamic = 'force-dynamic';

export async function GET(request: Request) {
  const keyword = new URL(request.url).searchParams.get('q')?.trim() ?? '';
  if (!keyword) {
    return NextResponse.json({ data: [] });
  }

  try {
    const res = await fetch(
      `${API_BASE}/api/v1/posts?q=${encodeURIComponent(keyword)}&per_page=8`,
      { cache: 'no-store' },
    );
    if (!res.ok) {
      return NextResponse.json({ data: [], error: 'search_failed' }, { status: 502 });
    }
    const json = await res.json();
    return NextResponse.json({ data: json.data ?? [] });
  } catch {
    return NextResponse.json({ data: [], error: 'upstream_unreachable' }, { status: 503 });
  }
}
