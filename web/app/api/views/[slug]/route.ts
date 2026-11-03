import { NextResponse } from 'next/server';

import { API_BASE } from '@/lib/api';

/** 阅读量自增代理。限流由 Go 侧按客户端 IP 执行。 */
export const dynamic = 'force-dynamic';

export async function POST(
  request: Request,
  { params }: { params: Promise<{ slug: string }> },
) {
  const { slug } = await params;

  try {
    const res = await fetch(`${API_BASE}/api/v1/posts/${encodeURIComponent(slug)}/views`, {
      method: 'POST',
      cache: 'no-store',
      headers: {
        // 把真实客户端 IP 透传给后端，否则所有请求会被算成同一个来源
        'X-Forwarded-For': request.headers.get('x-forwarded-for') ?? '',
      },
    });

    const json = await res.json().catch(() => null);
    return NextResponse.json(json ?? { data: null }, { status: res.status });
  } catch {
    return NextResponse.json({ error: 'upstream_unreachable' }, { status: 503 });
  }
}
