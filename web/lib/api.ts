import { cache } from 'react';

import type {
  ArchiveYear,
  Meta,
  Page,
  Post,
  PostDetail,
  Site,
  Tag,
  TagDetail,
} from './types';

/**
 * 服务端访问 Go API 的地址。
 * 单容器生产环境和本地开发都通过 http://127.0.0.1:8080 访问 Go API。
 */
const API_BASE = (process.env.API_BASE_URL ?? 'http://127.0.0.1:8080').replace(/\/+$/, '');

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

type Envelope<T> = { data: T; meta?: Meta };

async function request<T>(path: string): Promise<Envelope<T>> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      // 页面保持动态渲染：构建镜像时后端还不存在，把页面静态化会把
      // 「后端不可用」的空白页烤进产物。数据库层面的重复查询由 Go 侧的
      // 进程内响应缓存承担（15s TTL，后台写入后立即整体失效）。
      cache: 'no-store',
      headers: { Accept: 'application/json' },
    });
  } catch {
    throw new ApiError(`无法连接后端服务（${API_BASE}）`, 503);
  }

  if (res.status === 404) {
    throw new ApiError('内容不存在', 404);
  }
  if (!res.ok) {
    throw new ApiError(`后端返回 ${res.status}`, res.status);
  }
  return (await res.json()) as Envelope<T>;
}

function qs(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value));
    }
  }
  const s = search.toString();
  return s ? `?${s}` : '';
}

/**
 * getSite 会在 generateMetadata 与 layout 中被各调用一次，
 * 用 React.cache 保证同一请求内只打一次后端。
 */
export const getSite = cache(async (): Promise<Site> => (await request<Site>('/api/v1/site')).data);

export const getPost = cache(
  async (slug: string): Promise<PostDetail> =>
    (await request<PostDetail>(`/api/v1/posts/${encodeURIComponent(slug)}`)).data,
);

export const getPage = cache(
  async (slug: string): Promise<Page> =>
    (await request<Page>(`/api/v1/pages/${encodeURIComponent(slug)}`)).data,
);

export async function getPosts(params: {
  tag?: string;
  q?: string;
  year?: number;
  page?: number;
  perPage?: number;
} = {}): Promise<{ items: Post[]; meta: Meta }> {
  const res = await request<Post[]>(
    `/api/v1/posts${qs({
      tag: params.tag,
      q: params.q,
      year: params.year,
      page: params.page,
      per_page: params.perPage,
    })}`,
  );
  return {
    items: res.data ?? [],
    meta: res.meta ?? { page: 1, per_page: res.data?.length ?? 0, total: res.data?.length ?? 0, total_pages: 1 },
  };
}

export async function getRelated(slug: string, limit = 3): Promise<Post[]> {
  const res = await request<Post[]>(
    `/api/v1/posts/${encodeURIComponent(slug)}/related${qs({ limit })}`,
  );
  return res.data ?? [];
}

export async function getTags(): Promise<Tag[]> {
  const res = await request<Tag[]>('/api/v1/tags');
  return res.data ?? [];
}

export async function getTag(slug: string): Promise<TagDetail> {
  const res = await request<TagDetail>(`/api/v1/tags/${encodeURIComponent(slug)}`);
  return res.data;
}

export async function getArchive(): Promise<ArchiveYear[]> {
  const res = await request<ArchiveYear[]>('/api/v1/archive');
  return res.data ?? [];
}

/**
 * 草稿预览：后端校验签名和有效期后才返回内容。
 * 410 表示链接过期，404 表示签名不对或文章不存在。
 */
export async function getPreview(slug: string, token: string, exp: string): Promise<Post> {
  const search = new URLSearchParams({ t: token, exp });
  return (await request<Post>(`/api/v1/preview/${encodeURIComponent(slug)}?${search}`)).data;
}

export { API_BASE };
