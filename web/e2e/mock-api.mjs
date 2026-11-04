/**
 * 端到端测试用的假后端。
 *
 * 只实现前端关键路径真正会打到的接口，数据写死在这个文件里。
 * 这样 e2e 不依赖 PostgreSQL，也不需要在 CI 里起 Go 服务；
 * 但页面仍然是真实的 Next.js SSR 渲染 + 真实浏览器交互。
 *
 * 端口由 MOCK_API_PORT 指定，默认 8080（正好是 lib/api.ts 的默认值）。
 */
import { createServer } from 'node:http';

const PORT = Number(process.env.MOCK_API_PORT ?? 8080);

const author = { id: 1, slug: 'author', name: '站长', avatar_url: '', bio: '写代码，也写生活。' };
const tag = { id: 1, slug: 'go', name: 'Go', color: '#00add8', post_count: 1 };

const post = {
  id: 1,
  slug: 'demo-post',
  title: '演示文章：用于端到端测试',
  summary: '这是一篇只存在于测试环境里的文章。',
  body_md: '# 演示文章\n\n正文第一段。\n\n- 列表项一\n- 列表项二\n',
  cover_color: '#1f2937',
  cover_url: '',
  status: 'published',
  featured: true,
  reading_minutes: 1,
  views: 12,
  author,
  tags: [tag],
  published_at: '2026-11-01T02:00:00Z',
  created_at: '2026-11-01T02:00:00Z',
  updated_at: '2026-11-02T02:00:00Z',
};

const draft = { ...post, id: 2, slug: 'draft-post', title: '草稿：还没写完', status: 'draft', featured: false };

const site = {
  name: '测试博客',
  tagline: '记录，然后分享',
  description: '端到端测试站点',
  url: `http://127.0.0.1:${process.env.E2E_WEB_PORT ?? 3100}`,
  author,
  socials: [],
  texts: { 'home.visual_title': '写代码，也写生活' },
  stats: { posts: 2, tags: 1, chars: 320, last_published_at: '2026-11-02T02:00:00Z' },
  tags: [tag],
  featured: post,
  recent: [post, draft],
};

/** 记录收到的请求，测试里可以通过 /__requests 断言「确实发出去了」。 */
const requests = [];
let commentSeq = 1;

function json(res, status, body) {
  const buf = Buffer.from(JSON.stringify(body));
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': buf.length,
    'Cache-Control': 'no-store',
  });
  res.end(buf);
}

function readBody(req) {
  return new Promise((resolve) => {
    const chunks = [];
    req.on('data', (c) => chunks.push(c));
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
  });
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url ?? '/', `http://127.0.0.1:${PORT}`);
  const path = url.pathname;
  const method = req.method ?? 'GET';
  requests.push({ method, path, query: url.search, at: Date.now() });

  if (path === '/__requests') return json(res, 200, requests);

  // ---- 公开接口 ----
  if (method === 'GET' && path === '/api/v1/site') return json(res, 200, { data: site });

  if (method === 'GET' && path === '/api/v1/tags') return json(res, 200, { data: [tag] });

  if (method === 'GET' && path === '/api/v1/archive') {
    return json(res, 200, { data: [{ year: 2026, count: 2, months: [{ year: 2026, month: 11, count: 2 }] }] });
  }

  if (method === 'GET' && path === '/api/v1/pages/about') {
    return json(res, 200, { data: { slug: 'about', title: '关于', body_md: '关于这个博客。', updated_at: '2026-11-01T00:00:00Z' } });
  }

  if (method === 'GET' && path === '/api/v1/posts') {
    return json(res, 200, {
      data: [post],
      meta: { page: 1, per_page: 9, total: 1, total_pages: 1 },
    });
  }

  if (method === 'GET' && path === '/api/v1/posts/demo-post') {
    return json(res, 200, { data: { post, prev: null, next: null } });
  }

  if (method === 'GET' && path === '/api/v1/posts/demo-post/related') {
    return json(res, 200, { data: [] });
  }

  if (method === 'GET' && path === '/api/v1/posts/demo-post/comments') {
    return json(res, 200, { data: { enabled: true, comments: [], total: 0 } });
  }

  if (method === 'POST' && path === '/api/v1/posts/demo-post/comments') {
    const body = JSON.parse((await readBody(req)) || '{}');
    return json(res, 200, {
      data: {
        status: 'pending',
        comment: {
          id: commentSeq++,
          author: body.author ?? '',
          body: body.body ?? '',
          status: 'pending',
          created_at: new Date().toISOString(),
        },
      },
    });
  }

  if (method === 'POST' && /^\/api\/v1\/posts\/[^/]+\/views$/.test(path)) {
    return json(res, 200, { data: { views: post.views + 1 } });
  }

  // ---- 后台接口 ----
  if (method === 'POST' && path === '/api/v1/admin/login') {
    const body = JSON.parse((await readBody(req)) || '{}');
    if (body.username === 'admin' && body.password === 'test-password') {
      return json(res, 200, { data: { token: 'e2e-token', username: 'admin', displayName: '管理员' } });
    }
    return json(res, 401, { error: { code: 'invalid_credentials', message: '用户名或密码不正确' } });
  }

  if (method === 'GET' && path === '/api/v1/admin/whoami') {
    return json(res, 200, { data: { username: 'admin', displayName: '管理员' } });
  }

  if (method === 'GET' && path === '/api/v1/admin/posts') return json(res, 200, { data: [post, draft] });

  if (method === 'POST' && path === '/api/v1/admin/posts') {
    const body = JSON.parse((await readBody(req)) || '{}');
    return json(res, 200, { data: { id: 3, slug: body.slug || 'post-20261104-abcdef' } });
  }

  if (method === 'GET' && path === '/api/v1/admin/comments') {
    return json(res, 200, { data: { comments: [], counts: { pending: 0, published: 0, hidden: 0 } } });
  }

  if (method === 'GET' && path === '/api/v1/admin/tags') return json(res, 200, { data: [tag] });

  return json(res, 404, { error: { code: 'not_found', message: `mock 未实现：${method} ${path}` } });
});

server.listen(PORT, '127.0.0.1', () => {
  console.log(`[mock-api] listening on http://127.0.0.1:${PORT}`);
});