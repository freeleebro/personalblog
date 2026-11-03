import { NextResponse } from 'next/server';

import { API_BASE } from '@/lib/api';

/**
 * 管理后台通用代理。
 * 把 /api-proxy/api/v1/** 原样转发到 Go 后端，透传方法、请求体与 Authorization 头。
 * 这样管理后台全程同源，既不跨域，也不把后端地址暴露给浏览器。
 */
export const dynamic = 'force-dynamic';

export async function GET(request: Request, { params }: { params: Promise<{ path: string[] }> }) {
  return proxy(request, await params);
}

export async function POST(request: Request, { params }: { params: Promise<{ path: string[] }> }) {
  return proxy(request, await params);
}

export async function PUT(request: Request, { params }: { params: Promise<{ path: string[] }> }) {
  return proxy(request, await params);
}

export async function DELETE(request: Request, { params }: { params: Promise<{ path: string[] }> }) {
  return proxy(request, await params);
}

async function proxy(request: Request, params: { path: string[] }) {
  // 浏览器路径 /api-proxy/api/v1/xxx 里的 path 已经带 /api/v1 前缀，
  // 直接原样拼接，不要再补一次前缀（否则后端收到 /api/v1/api/v1/...）。
  const path = params.path.map((seg) => encodeURIComponent(seg)).join('/');
  const upstream = `${API_BASE}/${path}`;

  const url = new URL(request.url);
  const query = url.searchParams.toString();
  const target = query ? `${upstream}?${query}` : upstream;

  // 请求体必须按字节转发。图片上传是 multipart，用 request.text() 会被按 UTF-8
  // 解码一遍，二进制随手就毁了（后端只会回 415 unsupported_type）。
  const payload = ['GET', 'HEAD'].includes(request.method) ? undefined : await request.arrayBuffer();

  try {
    const res = await fetch(target, {
      method: request.method,
      headers: {
        Accept: request.headers.get('accept') ?? 'application/json',
        'Content-Type': request.headers.get('content-type') ?? 'application/json',
        Authorization: request.headers.get('authorization') ?? '',
      },
      body: payload,
      cache: 'no-store',
    });

    // 响应体同样必须按字节转发：res.text() 会按 UTF-8 解码，zip / 图片之类
    // 的二进制会被替换成 U+FFFD（导出功能一开始就是这么坏掉的）。
    const body = await res.arrayBuffer();
    const headers: Record<string, string> = {
      'Content-Type': res.headers.get('content-type') ?? 'application/json',
    };
    // 导出的 zip 靠这个头告诉浏览器存成什么文件名
    const disposition = res.headers.get('content-disposition');
    if (disposition) headers['Content-Disposition'] = disposition;

    const upstreamCache = res.headers.get('cache-control');
    if (request.method !== 'GET') {
      // 写接口不缓存
      headers['Cache-Control'] = 'no-store';
    } else if (upstreamCache) {
      headers['Cache-Control'] = upstreamCache;
    }

    return new NextResponse(body.byteLength > 0 ? body : null, { status: res.status, headers });
  } catch {
    return NextResponse.json(
      { error: { code: 'upstream_unreachable', message: '无法连接后端服务' } },
      { status: 503 },
    );
  }
}
