'use client';

// 管理后台的 API 客户端。所有写操作都带 Bearer token。
// token 存在 localStorage，由调用方（登录后）注入。

import type { Page, Post, Site, Tag } from './types';

// 管理后台走同源代理 /api-proxy，避免跨域，也不把后端地址暴露给浏览器。
const ADMIN_BASE = '/api-proxy';

let token: string | null = null;
if (typeof window !== 'undefined') {
  try {
    token = localStorage.getItem('admin_token');
  } catch {
    token = null;
  }
}

export function setToken(t: string | null) {
  token = t;
  try {
    if (t) localStorage.setItem('admin_token', t);
    else localStorage.removeItem('admin_token');
  } catch {
    // ignore
  }
}

export function getToken() {
  return token;
}

export class AdminError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'AdminError';
  }
}

async function adminRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) headers.Authorization = `Bearer ${token}`;

  let res: Response;
  try {
    res = await fetch(`${ADMIN_BASE}/api/v1${path}`, {
      ...init,
      headers: { ...headers, ...(init?.headers ?? {}) },
    });
  } catch {
    throw new AdminError(`无法连接后端（${ADMIN_BASE}）`, 503);
  }

  if (res.status === 401) {
    throw new AdminError('登录已过期，请重新登录', 401);
  }

  const body = await res.json().catch(() => null);
  if (!res.ok) {
    const msg = (body as { error?: { message?: string } })?.error?.message ?? `请求失败（${res.status}）`;
    throw new AdminError(msg, res.status);
  }
  return (body as { data: T }).data;
}

export const adminApi = {
  login: (username: string, password: string) =>
    adminRequest<{ token: string; username: string; displayName: string }>('/admin/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  whoami: () => adminRequest<{ username: string; displayName: string }>('/admin/whoami'),

  /**
   * 修改当前账号密码。旧密码由服务端校验。
   * 注意：改完 token 仍然有效，其他设备不会被踢下线（无状态签名，见 README）。
   */
  changePassword: (currentPassword: string, newPassword: string) =>
    adminRequest<{ changed_at: string; sessions_revoked: boolean }>('/admin/password', {
      method: 'POST',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    }),

  listPosts: () => adminRequest<Post[]>('/admin/posts'),
  getPost: (slug: string) => adminRequest<Post>(`/admin/posts/${encodeURIComponent(slug)}`),
  savePost: (post: {
    slug: string;
    title: string;
    summary: string;
    body_md: string;
    cover_color: string;
    cover_url?: string;
    status: string;
    featured: boolean;
    tag_ids: number[];
    published_at?: string | null;
  }) =>
    adminRequest<{ id: number; slug: string }>('/admin/posts', {
      method: 'POST',
      body: JSON.stringify(post),
    }),
  deletePost: (slug: string) =>
    adminRequest<{ deleted: boolean }>(`/admin/posts/${encodeURIComponent(slug)}`, {
      method: 'DELETE',
    }),

  /** 生成一条 1 小时有效的草稿预览链接。 */
  previewLink: (slug: string) =>
    adminRequest<{ url: string; expires_at: string }>(
      `/admin/preview/${encodeURIComponent(slug)}`,
    ),

  /**
   * 上传一张配图。走 multipart，不能复用 adminRequest（它会写死 JSON 头）。
   * 返回的 url 是 /media/... 站内路径，可直接写进 Markdown。
   */
  uploadImage: async (file: Blob, filename: string) => {
    const form = new FormData();
    form.append('file', file, filename);

    let res: Response;
    try {
      res = await fetch(`${ADMIN_BASE}/api/v1/admin/uploads`, {
        method: 'POST',
        headers: {
          Accept: 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: form,
      });
    } catch {
      throw new AdminError(`无法连接后端（${ADMIN_BASE}）`, 503);
    }

    if (res.status === 401) throw new AdminError('登录已过期，请重新登录', 401);

    const body = await res.json().catch(() => null);
    if (!res.ok) {
      const msg =
        (body as { error?: { message?: string } })?.error?.message ?? `上传失败（${res.status}）`;
      throw new AdminError(msg, res.status);
    }
    return (body as { data: { url: string; width: number; height: number; size: number } }).data;
  },

  listTags: () => adminRequest<Tag[]>('/tags'),
  saveTag: (tag: { slug: string; name: string; color: string; description: string }) =>
    adminRequest<{ id: number; slug: string }>('/admin/tags', {
      method: 'POST',
      body: JSON.stringify(tag),
    }),
  deleteTag: (slug: string) =>
    adminRequest<{ deleted: boolean }>(`/admin/tags/${encodeURIComponent(slug)}`, {
      method: 'DELETE',
    }),

  getPage: (slug: string) => adminRequest<Page>(`/pages/${encodeURIComponent(slug)}`),
  savePage: (page: { slug: string; title: string; body_md: string }) =>
    adminRequest<{ slug: string }>('/admin/pages', {
      method: 'POST',
      body: JSON.stringify(page),
    }),
  deletePage: (slug: string) =>
    adminRequest<{ deleted: boolean }>(`/admin/pages/${encodeURIComponent(slug)}`, {
      method: 'DELETE',
    }),

  getSite: () => adminRequest<Site>('/site'),
  saveSite: (site: {
    name: string;
    tagline: string;
    description: string;
    url: string;
    socials: { label: string; url: string }[];
    texts?: Record<string, string>;
  }) =>
    adminRequest<{ saved: boolean }>('/admin/site', {
      method: 'PUT',
      body: JSON.stringify(site),
    }),
};
