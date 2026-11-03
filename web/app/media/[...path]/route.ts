import { NextResponse } from 'next/server';

import { API_BASE } from '@/lib/api';

/**
 * 文章配图的同源代理。
 *
 * 生产环境 Caddy 会对 /media/* 下发一年强缓存，真正打到这里的只有首次访问；
 * 文件本身由 Go 落在 UPLOAD_DIR 数据卷里，换镜像不会丢。
 */
export const dynamic = 'force-dynamic';

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const target = `${API_BASE}/media/${path.map((seg) => encodeURIComponent(seg)).join('/')}`;

  let res: Response;
  try {
    res = await fetch(target, { cache: 'no-store' });
  } catch {
    return new NextResponse('upstream unreachable', { status: 503 });
  }
  if (!res.ok || !res.body) {
    return new NextResponse('not found', { status: res.status === 404 ? 404 : 502 });
  }

  const headers = new Headers();
  headers.set('Content-Type', res.headers.get('content-type') ?? 'application/octet-stream');
  headers.set(
    'Cache-Control',
    res.headers.get('cache-control') ?? 'public, max-age=31536000, immutable',
  );
  const length = res.headers.get('content-length');
  if (length) headers.set('Content-Length', length);

  return new NextResponse(res.body, { status: 200, headers });
}
