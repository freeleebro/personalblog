'use client';

import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react';

import { adminApi } from '@/lib/admin-api';
import { SITE_TEXT_KEYS } from '@/lib/site-text';
import type { Comment, CommentCounts, Post, Tag } from '@/lib/types';
import { MarkdownPreview } from './markdown-preview';

type Tab = 'posts' | 'comments' | 'tags' | 'about' | 'settings';

// 编辑中的文章草稿
type Draft = {
  id?: number;
  slug: string;
  title: string;
  summary: string;
  body_md: string;
  cover_color: string;
  cover_url: string;
  status: string;
  featured: boolean;
  tag_ids: number[];
  published_at: string | null;
};

const EMPTY_DRAFT: Draft = {
  slug: '',
  title: '',
  summary: '',
  body_md: '',
  cover_color: '#8b5cf6',
  cover_url: '',
  status: 'draft',
  featured: false,
  tag_ids: [],
  published_at: null,
};

const STATUS_LABEL: Record<string, string> = {
  draft: '草稿',
  published: '已发布',
  archived: '已归档',
};

const PALETTE = [
  '#ff5d9e', '#8b5cf6', '#22d3ee', '#a78bfa', '#ff9bc9', '#5eead4',
  '#2563eb', '#0d9488', '#ca8a04', '#ea580c', '#dc2626', '#4f46e5',
];

type MarkdownFrontMatter = Record<string, string | string[]>;

function parseMarkdownFile(text: string, fileName: string): {
  body: string;
  meta: MarkdownFrontMatter;
  title: string;
  summary: string;
  slug: string;
} {
  const frontMatterMatch = text.match(/^---\s*\r?\n([\s\S]*?)\r?\n---\s*\r?\n?/);
  const meta: MarkdownFrontMatter = {};
  let body = text;

  if (frontMatterMatch) {
    body = text.slice(frontMatterMatch[0].length);
    for (const line of frontMatterMatch[1].split(/\r?\n/)) {
      const match = line.match(/^([\w-]+):\s*(.*)$/);
      if (!match) continue;
      const [, key, raw] = match;
      const value = raw.trim().replace(/^['"]|['"]$/g, '');
      if (key === 'tags' || key === 'categories') {
        meta[key] = value
          .replace(/^\[|\]$/g, '')
          .split(',')
          .map((item) => item.trim().replace(/^['"]|['"]$/g, ''))
          .filter(Boolean);
      } else {
        meta[key] = value;
      }
    }
  }

  const cleanBody = body.trim();
  const heading = cleanBody.match(/^#\s+(.+)$/m)?.[1]?.trim() ?? '';
  const paragraph = cleanBody
    .split(/\r?\n\s*\r?\n/)
    .map((part) => part.trim())
    .find((part) => part && !part.startsWith('#') && !part.startsWith('```'))
    ?.replace(/\s+/g, ' ')
    .slice(0, 180) ?? '';
  const rawSlug = String(meta.slug ?? fileName.replace(/\.(markdown?|md)$/i, ''));
  const slug = rawSlug
    .trim()
    .replace(/[^\w\u4e00-\u9fff]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .toLowerCase() || 'new-note';

  return {
    body: cleanBody,
    meta,
    title: String(meta.title ?? heading ?? fileName.replace(/\.(markdown?|md)$/i, '')),
    summary: String(meta.summary ?? paragraph),
    slug,
  };
}

function parsePublishedAt(value: string | string[] | undefined): string | null {
  if (!value || Array.isArray(value)) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date.toISOString();
}

// ---------------------------------------------------------------- 时间与图片工具

const pad2 = (n: number) => String(n).padStart(2, '0');

/** ISO 时间 → <input type="datetime-local"> 需要的本地时间字符串。 */
function toLocalInput(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
}

/** datetime-local 的值按浏览器本地时区解析，再统一存成 ISO。 */
function fromLocalInput(value: string): string | null {
  if (!value) return null;
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

/** 明天 09:00（本地时区），用于「排到明天早上」。 */
function tomorrowMorning(): string {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  d.setHours(9, 0, 0, 0);
  return d.toISOString();
}

/** 已发布但时间还没到，就是「待发布」。 */
function isScheduled(status: string, publishedAt: string | null): boolean {
  if (status !== 'published' || !publishedAt) return false;
  const t = new Date(publishedAt).getTime();
  return !Number.isNaN(t) && t > Date.now();
}

/**
 * 上传前在浏览器里压到长边 1600px、quality 0.82。
 * 原图常有 3~5MB，压完通常 100~300KB；GIF 不动，避免把动图压成静图。
 */
async function compressImage(file: File): Promise<Blob> {
  if (file.type === 'image/gif' || file.size < 200 * 1024) return file;

  try {
    const bitmap = await createImageBitmap(file);
    const maxEdge = 1600;
    const scale = Math.min(1, maxEdge / Math.max(bitmap.width, bitmap.height));
    const width = Math.max(1, Math.round(bitmap.width * scale));
    const height = Math.max(1, Math.round(bitmap.height * scale));

    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      bitmap.close();
      return file;
    }
    ctx.drawImage(bitmap, 0, 0, width, height);
    bitmap.close();

    const blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, 'image/webp', 0.82),
    );
    return blob && blob.size < file.size ? blob : file;
  } catch {
    return file;
  }
}

export function AdminEditor({ onLogout }: { onLogout: () => void }) {
  const [tab, setTab] = useState<Tab>('posts');
  const [posts, setPosts] = useState<Post[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // 编辑器状态
  const [draft, setDraft] = useState<Draft | null>(null);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [p, t] = await Promise.all([adminApi.listPosts(), adminApi.listTags()]);
      setPosts(p);
      setTags(t);
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function handleSave() {
    if (!draft) return;
    if (!draft.title.trim()) {
      setNotice('标题不能为空');
      return;
    }
    if (draft.id && !draft.slug.trim()) {
      setNotice('修改已有文章时 slug 不能为空');
      return;
    }
    setSaving(true);
    setNotice('');
    try {
      const saved = await adminApi.savePost({
        slug: draft.slug,
        title: draft.title,
        summary: draft.summary,
        body_md: draft.body_md,
        cover_color: draft.cover_color,
        cover_url: draft.cover_url,
        status: draft.status,
        featured: draft.featured,
        tag_ids: draft.tag_ids,
        published_at: draft.published_at,
      });
      if (!draft.slug.trim()) {
        // 新文章留空 slug：由服务端按标题生成（中文标题会得到 post-日期-哈希）。
        setDraft({ ...draft, id: saved.id, slug: saved.slug });
        setNotice(`已保存 ✓ 自动生成的 URL：/posts/${saved.slug}`);
      } else {
        setNotice('已保存 ✓');
      }
      await load();
    } catch (e) {
      setNotice(e instanceof Error ? e.message : '保存失败');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(slug: string) {
    if (!confirm(`确认删除文章「${slug}」？此操作不可恢复。`)) return;
    try {
      await adminApi.deletePost(slug);
      setNotice('已删除');
      if (draft?.slug === slug) setDraft(null);
      await load();
    } catch (e) {
      setNotice(e instanceof Error ? e.message : '删除失败');
    }
  }

  function startNew() {
    setDraft({ ...EMPTY_DRAFT });
  }

  async function editPost(slug: string) {
    try {
      const p = await adminApi.getPost(slug);
      setDraft({
        id: p.id,
        slug: p.slug,
        title: p.title,
        summary: p.summary,
        body_md: p.body_md ?? '',
        cover_color: p.cover_color,
        cover_url: p.cover_url ?? '',
        status: p.status,
        featured: p.featured,
        tag_ids: p.tags.map((t) => t.id),
        published_at: p.published_at,
      });
    } catch (e) {
      setNotice(e instanceof Error ? e.message : '加载文章失败');
    }
  }

  return (
    <div className="admin-shell">
      <header className="admin-topbar">
        <div className="admin-topbar__brand">
          <span className="admin-topbar__logo">B</span>
          <div>
            <strong>博客后台</strong>
            <span className="admin-topbar__sub">个人博客后台</span>
          </div>
        </div>
        <nav className="admin-tabs">
          <button className={`admin-tab${tab === 'posts' ? ' admin-tab--active' : ''}`} onClick={() => setTab('posts')}>
            文章
          </button>
          <button className={`admin-tab${tab === 'comments' ? ' admin-tab--active' : ''}`} onClick={() => setTab('comments')}>
            评论
          </button>
          <button className={`admin-tab${tab === 'tags' ? ' admin-tab--active' : ''}`} onClick={() => setTab('tags')}>
            分类 / 标签
          </button>
          <button className={`admin-tab${tab === 'about' ? ' admin-tab--active' : ''}`} onClick={() => setTab('about')}>
            关于页面
          </button>
          <button className={`admin-tab${tab === 'settings' ? ' admin-tab--active' : ''}`} onClick={() => setTab('settings')}>
            站点设置
          </button>
        </nav>
        <div className="admin-topbar__actions">
          <a className="admin-link" href="/" target="_blank">看前台 ↗</a>
          <button className="admin-btn-ghost" onClick={onLogout}>退出</button>
        </div>
      </header>

      {error ? <p className="admin-error">{error}</p> : null}
      {notice ? <p className="admin-notice">{notice}</p> : null}

      {loading ? (
        <p className="admin-loading">加载中…</p>
      ) : tab === 'posts' ? (
        <>
          <div className="admin-page-intro">
            <div>
              <p className="admin-kicker">CONTENT / WORKSPACE</p>
              <h1>文章工作台</h1>
              <p>导入 Markdown，整理标签，预览后发布你的文章。</p>
            </div>
            <div className="admin-page-stats">
              <strong>{posts.length}</strong><span>篇文章</span>
              <strong>{tags.length}</strong><span>个分类</span>
            </div>
          </div>
          <div className="admin-posts">
          <div className="admin-posts__list">
            <div className="admin-posts__head">
              <div>
                <h2 className="admin-section-title">全部文章</h2>
                <p className="admin-section-hint">按最近更新时间排列</p>
              </div>
              <button className="btn btn--primary" onClick={startNew}>新建文章</button>
            </div>

            {posts.length === 0 ? (
              <p className="admin-empty">还没有文章，点「新建文章」开始写作。</p>
            ) : (
              <ul className="admin-post-list">
                {posts.map((p) => (
                  <li key={p.id} className="admin-post-item">
                    <button className="admin-post-item__main" onClick={() => editPost(p.slug)}>
                      {isScheduled(p.status, p.published_at) ? (
                        <span className="admin-status admin-status--scheduled">待发布</span>
                      ) : (
                        <span className={`admin-status admin-status--${p.status}`}>
                          {STATUS_LABEL[p.status] ?? p.status}
                        </span>
                      )}
                      <span className="admin-post-item__title">{p.title}</span>
                      <span className="admin-post-item__slug">{p.slug}</span>
                      <span className="admin-post-item__tags">
                        {p.tags.slice(0, 2).map((tag) => tag.name).join(' · ') || '未分类'}
                      </span>
                    </button>
                    <button className="admin-post-item__del" onClick={() => handleDelete(p.slug)} title="删除">
                      ✕
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {draft ? (
            <PostEditor
              draft={draft}
              tags={tags}
              onChange={setDraft}
              onSave={handleSave}
              saving={saving}
              onCancel={() => setDraft(null)}
            />
          ) : (
            <div className="admin-editor-empty">
              <span className="admin-editor-empty__mark">⌁</span>
              <strong>选择一篇文章</strong>
              <p>从左侧打开文章，或导入本地 Markdown 文件开始整理。</p>
            </div>
          )}
          </div>
        </>
      ) : tab === 'comments' ? (
        <CommentsPanel />
      ) : tab === 'tags' ? (
        <TagsPanel tags={tags} onChanged={load} />
      ) : tab === 'about' ? (
        <AboutPanel />
      ) : (
        <SettingsPanel />
      )}
    </div>
  );
}

// ---------------------------------------------------------------- 文章编辑器

function PostEditor({
  draft,
  tags,
  onChange,
  onSave,
  saving,
  onCancel,
}: {
  draft: Draft;
  tags: Tag[];
  onChange: (d: Draft) => void;
  onSave: () => void;
  saving: boolean;
  onCancel: () => void;
}) {
  const [preview, setPreview] = useState(false);
  const [importName, setImportName] = useState('');
  const [importNotice, setImportNotice] = useState('');
  const [uploading, setUploading] = useState(false);
  const [uploadNotice, setUploadNotice] = useState('');
  const [linkNotice, setLinkNotice] = useState('');
  const bodyRef = useRef<HTMLTextAreaElement>(null);

  function toggleTag(id: number) {
    const has = draft.tag_ids.includes(id);
    onChange({
      ...draft,
      tag_ids: has ? draft.tag_ids.filter((t) => t !== id) : [...draft.tag_ids, id],
    });
  }

  /** 把一段 Markdown 插到光标处（没有光标就追加到末尾）。 */
  function insertAtCursor(snippet: string) {
    const el = bodyRef.current;
    const body = draft.body_md;
    if (!el) {
      onChange({ ...draft, body_md: `${body}\n\n${snippet}\n` });
      return;
    }
    const start = el.selectionStart ?? body.length;
    const end = el.selectionEnd ?? start;
    const next = `${body.slice(0, start)}${snippet}${body.slice(end)}`;
    onChange({ ...draft, body_md: next });
    requestAnimationFrame(() => {
      el.focus();
      const pos = start + snippet.length;
      el.setSelectionRange(pos, pos);
    });
  }

  /** 压缩 + 上传，成功后插入 ![](url)。 */
  async function uploadIntoBody(file: File, alt = '') {
    setUploading(true);
    setUploadNotice('上传中…');
    try {
      const blob = await compressImage(file);
      const ext = blob.type === 'image/webp' ? 'webp' : (file.name.split('.').pop() ?? 'jpg');
      const { url } = await adminApi.uploadImage(blob, `upload.${ext}`);
      insertAtCursor(`![${alt}](${url})`);
      const kb = Math.round(blob.size / 1024);
      setUploadNotice(`已插入图片（${kb} KB）`);
    } catch (e) {
      setUploadNotice(e instanceof Error ? e.message : '上传失败，请重试');
    } finally {
      setUploading(false);
    }
  }

  async function uploadAsCover(file: File) {
    setUploading(true);
    setUploadNotice('上传封面中…');
    try {
      const blob = await compressImage(file);
      const ext = blob.type === 'image/webp' ? 'webp' : (file.name.split('.').pop() ?? 'jpg');
      const { url } = await adminApi.uploadImage(blob, `cover.${ext}`);
      onChange({ ...draft, cover_url: url });
      setUploadNotice('封面已更新');
    } catch (e) {
      setUploadNotice(e instanceof Error ? e.message : '上传封面失败');
    } finally {
      setUploading(false);
    }
  }

  async function copyPreviewLink() {
    if (!draft.slug) {
      setLinkNotice('先保存文章才能生成预览链接');
      return;
    }
    try {
      const { url } = await adminApi.previewLink(draft.slug);
      const absolute = `${window.location.origin}${url}`;
      window.open(absolute, '_blank', 'noopener');
      try {
        await navigator.clipboard.writeText(absolute);
        setLinkNotice('预览链接已复制（1 小时内有效）');
      } catch {
        setLinkNotice('预览链接已打开（复制失败，请手动复制地址栏）');
      }
    } catch (e) {
      setLinkNotice(e instanceof Error ? e.message : '生成预览链接失败');
    }
  }

  async function importMarkdown(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    if (!/\.(md|markdown)$/i.test(file.name)) {
      setImportNotice('请选择 .md 或 .markdown 文件');
      return;
    }

    try {
      const parsed = parseMarkdownFile(await file.text(), file.name);
      const frontMatterTags = [parsed.meta.tags, parsed.meta.categories]
        .flatMap((value) => (Array.isArray(value) ? value : value ? [value] : []))
        .map((value) => value.toLowerCase());
      const importedTagIds = tags
        .filter((tag) => frontMatterTags.includes(tag.name.toLowerCase()) || frontMatterTags.includes(tag.slug.toLowerCase()))
        .map((tag) => tag.id);

      onChange({
        ...draft,
        title: parsed.title,
        slug: String(parsed.meta.slug ?? parsed.slug),
        summary: parsed.summary,
        body_md: parsed.body,
        tag_ids: importedTagIds.length > 0 ? importedTagIds : draft.tag_ids,
        status: String(parsed.meta.status ?? draft.status),
        published_at: parsePublishedAt(parsed.meta.published_at ?? parsed.meta.date) ?? draft.published_at,
      });
      setImportName(file.name);
      setImportNotice(`已导入 ${file.name}，可继续编辑后保存`);
      setPreview(true);
    } catch {
      setImportNotice('读取 Markdown 文件失败，请重试');
    }
  }

  return (
    <div className="admin-editor">
      <div className="admin-editor__head">
        <button className="admin-btn-ghost" onClick={onCancel}>← 返回</button>
        <div className="admin-editor__mode">
          <button className={`admin-tab${!preview ? ' admin-tab--active' : ''}`} onClick={() => setPreview(false)}>
            编辑
          </button>
          <button className={`admin-tab${preview ? ' admin-tab--active' : ''}`} onClick={() => setPreview(true)}>
            预览
          </button>
        </div>
        <button className="admin-btn-ghost" onClick={copyPreviewLink} title="生成 1 小时有效的草稿预览链接">
          预览链接
        </button>
        <button className="btn btn--primary" onClick={onSave} disabled={saving}>
          {saving ? '保存中…' : '保存'}
        </button>
      </div>

      {linkNotice ? <p className="admin-field__hint">{linkNotice}</p> : null}

      {preview ? (
        <div className="admin-editor__preview">
          <h1 className="admin-preview-title">{draft.title || '（无标题）'}</h1>
          <MarkdownPreview>{draft.body_md}</MarkdownPreview>
        </div>
      ) : (
        <div className="admin-editor__form">
          <div className="admin-editor__row">
            <label className="admin-field admin-field--grow">
              <span className="admin-field__label">标题</span>
              <input
                className="admin-input"
                value={draft.title}
                onChange={(e) => onChange({ ...draft, title: e.target.value })}
              />
            </label>
            <label className="admin-field">
              <span className="admin-field__label">slug（URL 标识，留空自动生成）</span>
              <input
                className="admin-input"
                value={draft.slug}
                onChange={(e) => onChange({ ...draft, slug: e.target.value })}
                placeholder="留空则按标题生成；中文标题得到 post-日期-随机串"
              />
            </label>
          </div>

          <label className="admin-field">
            <span className="admin-field__label">摘要</span>
            <textarea
              className="admin-input admin-textarea"
              rows={2}
              value={draft.summary}
              onChange={(e) => onChange({ ...draft, summary: e.target.value })}
            />
          </label>

          <div className="admin-editor__row">
            <label className="admin-field">
              <span className="admin-field__label">状态</span>
              <select
                className="admin-input"
                value={draft.status}
                onChange={(e) => onChange({ ...draft, status: e.target.value })}
              >
                <option value="draft">草稿</option>
                <option value="published">已发布</option>
                <option value="archived">已归档</option>
              </select>
            </label>
            <label className="admin-field admin-field--grow">
              <span className="admin-field__label">发布时间</span>
              <input
                className="admin-input"
                type="datetime-local"
                value={toLocalInput(draft.published_at)}
                onChange={(e) => onChange({ ...draft, published_at: fromLocalInput(e.target.value) })}
              />
              <div className="admin-quick-row">
                <button
                  type="button"
                  className="admin-btn-ghost"
                  onClick={() => onChange({ ...draft, status: 'published', published_at: new Date().toISOString() })}
                >
                  立即发布
                </button>
                <button
                  type="button"
                  className="admin-btn-ghost"
                  onClick={() => onChange({ ...draft, status: 'published', published_at: tomorrowMorning() })}
                >
                  排到明天 09:00
                </button>
                {draft.published_at ? (
                  <button
                    type="button"
                    className="admin-btn-ghost"
                    onClick={() => onChange({ ...draft, published_at: null })}
                  >
                    清空
                  </button>
                ) : null}
              </div>
              <span className="admin-field__hint">
                {isScheduled(draft.status, draft.published_at)
                  ? '时间到之前，前台、RSS、sitemap 都不会出现这篇文章。'
                  : '留空则用保存时间；填未来时间就是定时发布。'}
              </span>
            </label>
            <label className="admin-field">
              <span className="admin-field__label">封面颜色</span>
              <div className="admin-color-row">
                {PALETTE.map((c) => (
                  <button
                    key={c}
                    type="button"
                    className={`admin-color-swatch${draft.cover_color === c ? ' admin-color-swatch--active' : ''}`}
                    style={{ background: c }}
                    onClick={() => onChange({ ...draft, cover_color: c })}
                    aria-label={c}
                  />
                ))}
              </div>
            </label>
          </div>

          <div className="admin-field">
            <span className="admin-field__label">封面图（可选）</span>
            <span className="admin-field__hint">不填就走上面的色块；列表卡片会用这张图，正文里的图仍按原样显示。</span>
            <div className="admin-cover-row">
              <div className={`admin-cover-preview${draft.cover_url ? ' admin-cover-preview--set' : ''}`}>
                {draft.cover_url ? <img src={draft.cover_url} alt="" /> : <span>色块</span>}
              </div>
              <input
                className="admin-input"
                value={draft.cover_url}
                onChange={(e) => onChange({ ...draft, cover_url: e.target.value })}
                placeholder="/media/2026/11/xxx.webp"
              />
              <label className="admin-import-button">
                <input
                  type="file"
                  accept="image/*"
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    e.target.value = '';
                    if (file) uploadAsCover(file);
                  }}
                />
                上传封面
              </label>
              {draft.cover_url ? (
                <button
                  type="button"
                  className="admin-btn-ghost"
                  onClick={() => onChange({ ...draft, cover_url: '' })}
                >
                  清除
                </button>
              ) : null}
            </div>
          </div>

          <label className="admin-field admin-field--check">
            <input
              type="checkbox"
              checked={draft.featured}
              onChange={(e) => onChange({ ...draft, featured: e.target.checked })}
            />
            <span>设为精选</span>
          </label>

          <div className="admin-field">
            <span className="admin-field__label">分类 / 标签</span>
            <span className="admin-field__hint">用标签整理主题，例如：技术、阅读、生活随笔。</span>
            <div className="admin-tag-picker">
              {tags.map((t) => (
                <button
                  key={t.id}
                  type="button"
                  className={`chip${draft.tag_ids.includes(t.id) ? ' chip--active' : ''}`}
                  style={{ '--chip': t.color } as React.CSSProperties}
                  onClick={() => toggleTag(t.id)}
                >
                  {t.name}
                </button>
              ))}
            </div>
          </div>

          <div className="admin-field">
            <span className="admin-field__label">正文（Markdown）</span>
            <div className="admin-import-row">
              <label className="admin-import-button">
                <input type="file" accept=".md,.markdown,text/markdown" onChange={importMarkdown} />
                导入 Markdown 文件
              </label>
              <label className="admin-import-button">
                <input
                  type="file"
                  accept="image/*"
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    e.target.value = '';
                    if (file) uploadIntoBody(file, file.name.replace(/\.[^.]+$/, ''));
                  }}
                />
                {uploading ? '上传中…' : '插入图片'}
              </label>
              <span className="admin-import-name">
                {importName || '支持 front matter 自动读取；图片可直接拖进编辑框或 ⌘V 粘贴'}
              </span>
            </div>
            {importNotice ? <p className="admin-field__hint">{importNotice}</p> : null}
            {uploadNotice ? <p className="admin-field__hint">{uploadNotice}</p> : null}
            <textarea
              ref={bodyRef}
              className="admin-input admin-textarea admin-textarea--md"
              value={draft.body_md}
              onChange={(e) => onChange({ ...draft, body_md: e.target.value })}
              onPaste={(e) => {
                const file = Array.from(e.clipboardData.files).find((f) => f.type.startsWith('image/'));
                if (!file) return;
                e.preventDefault();
                uploadIntoBody(file);
              }}
              onDragOver={(e) => {
                if (e.dataTransfer.types.includes('Files')) e.preventDefault();
              }}
              onDrop={(e) => {
                const file = Array.from(e.dataTransfer.files).find((f) => f.type.startsWith('image/'));
                if (!file) return;
                e.preventDefault();
                uploadIntoBody(file, file.name.replace(/\.[^.]+$/, ''));
              }}
              placeholder="# 标题&#10;&#10;在这里写 Markdown…"
            />
          </div>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------- 标签面板

function TagsPanel({ tags, onChanged }: { tags: Tag[]; onChanged: () => void }) {
  const [name, setName] = useState('');
  const [color, setColor] = useState('#8b5cf6');
  const [desc, setDesc] = useState('');
  const [msg, setMsg] = useState('');

  async function add() {
    if (!name.trim()) return;
    try {
      await adminApi.saveTag({ slug: '', name: name.trim(), color, description: desc });
      setName('');
      setDesc('');
      setMsg('已保存');
      onChanged();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : '保存失败');
    }
  }

  async function del(slug: string) {
    if (!confirm(`删除标签「${slug}」？`)) return;
    try {
      await adminApi.deleteTag(slug);
      setMsg('已删除');
      onChanged();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : '删除失败');
    }
  }

  return (
    <div className="admin-panel">
      <h2 className="admin-section-title">分类 / 标签</h2>
      {msg ? <p className="admin-notice">{msg}</p> : null}
      <p className="admin-hint">建议为文章建立稳定的分类，例如“技术”“阅读”“生活”。一篇文章可以同时属于多个分类。</p>

      <div className="admin-tag-add">
        <input className="admin-input" placeholder="标签名" value={name} onChange={(e) => setName(e.target.value)} />
        <input className="admin-input admin-input--color" type="color" value={color} onChange={(e) => setColor(e.target.value)} />
        <input className="admin-input" placeholder="描述（可选）" value={desc} onChange={(e) => setDesc(e.target.value)} />
        <button className="btn btn--primary" onClick={add}>添加</button>
      </div>

      <ul className="admin-tag-list">
        {tags.map((t) => (
          <li key={t.id} className="admin-tag-item">
            <span className="admin-tag-item__dot" style={{ background: t.color }} />
            <span className="admin-tag-item__name">{t.name}</span>
            <span className="admin-tag-item__meta">{t.slug} · {t.post_count} 篇</span>
            <button className="admin-post-item__del" onClick={() => del(t.slug)}>✕</button>
          </li>
        ))}
      </ul>
    </div>
  );
}

// ---------------------------------------------------------------- 关于页面

function AboutPanel() {
  const [title, setTitle] = useState('关于');
  const [body, setBody] = useState('');
  const [preview, setPreview] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');

  useEffect(() => {
    (async () => {
      try {
        const page = await adminApi.getPage('about');
        setTitle(page.title);
        setBody(page.body_md);
      } catch (e) {
        setMsg(e instanceof Error ? e.message : '加载关于页面失败');
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  async function save() {
    setSaving(true);
    setMsg('');
    try {
      await adminApi.savePage({ slug: 'about', title: title.trim() || '关于', body_md: body });
      setMsg('关于页面已保存，前台会立即更新');
    } catch (e) {
      setMsg(e instanceof Error ? e.message : '保存关于页面失败');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="admin-panel admin-about-panel">
      <div className="admin-panel-heading">
        <div>
          <p className="admin-kicker">ABOUT / PROFILE</p>
          <h2 className="admin-section-title">关于页面</h2>
          <p className="admin-hint">介绍你自己、关注的方向和联系方式。正文支持完整 Markdown。</p>
        </div>
        <a className="admin-link" href="/about" target="_blank" rel="noreferrer">打开前台 ↗</a>
      </div>

      {msg ? <p className="admin-notice">{msg}</p> : null}

      {loading ? (
        <p className="admin-loading">加载中…</p>
      ) : (
        <>
          <label className="admin-field">
            <span className="admin-field__label">页面标题</span>
            <input className="admin-input" value={title} onChange={(e) => setTitle(e.target.value)} />
          </label>

          <div className="admin-editor__head admin-about-toolbar">
            <span className="admin-field__label">页面正文</span>
            <div className="admin-editor__mode">
              <button className={`admin-tab${!preview ? ' admin-tab--active' : ''}`} onClick={() => setPreview(false)}>
                编辑 Markdown
              </button>
              <button className={`admin-tab${preview ? ' admin-tab--active' : ''}`} onClick={() => setPreview(true)}>
                预览渲染
              </button>
            </div>
            <button className="btn btn--primary" onClick={save} disabled={saving}>
              {saving ? '保存中…' : '保存关于页'}
            </button>
          </div>

          {preview ? (
            <div className="admin-about-preview">
              <h1 className="admin-preview-title">{title || '关于'}</h1>
              <MarkdownPreview>{body}</MarkdownPreview>
            </div>
          ) : (
            <textarea
              className="admin-input admin-textarea admin-about-textarea"
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder={'# 关于我\n\n介绍一下你自己、正在做的事情，以及这个博客记录什么。'}
            />
          )}
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------- 设置面板

function SettingsPanel() {
  const [name, setName] = useState('');
  const [tagline, setTagline] = useState('');
  const [description, setDescription] = useState('');
  const [url, setUrl] = useState('');
  const [texts, setTexts] = useState<Record<string, string>>({});
  const [notesEnabled, setNotesEnabled] = useState(false);
  const [msg, setMsg] = useState('');
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const site = await adminApi.getSite();
        setName(site.name);
        setTagline(site.tagline);
        setDescription(site.description);
        setUrl(site.url);
        setTexts(site.texts ?? {});
        setNotesEnabled(site.texts?.[SITE_TEXT_KEYS.notesEnabled] === '1');
      } catch (e) {
        setMsg(e instanceof Error ? e.message : '加载站点设置失败');
      } finally {
        setLoaded(true);
      }
    })();
  }, []);

  function setText(key: string, value: string) {
    setTexts((prev) => ({ ...prev, [key]: value }));
  }

  async function save() {
    try {
      await adminApi.saveSite({
        name,
        tagline,
        description,
        url,
        socials: [],
        texts: { ...texts, [SITE_TEXT_KEYS.notesEnabled]: notesEnabled ? '1' : '0' },
      });
      setMsg('已保存 ✓');
    } catch (e) {
      setMsg(e instanceof Error ? e.message : '保存失败');
    }
  }

  return (
    <div className="admin-panel">
      <h2 className="admin-section-title">站点设置</h2>
      {msg ? <p className="admin-notice">{msg}</p> : null}
      <p className="admin-hint">
        站点名称、副标题、描述与网址。改动后前台会实时生效（前台强制动态渲染）。
        首次部署可以从这里开始：先给博客起个名字，再去「关于页面」写自我介绍。
      </p>

      <label className="admin-field">
        <span className="admin-field__label">站点名称</span>
        <input className="admin-input" value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <label className="admin-field">
        <span className="admin-field__label">副标题</span>
        <input className="admin-input" value={tagline} onChange={(e) => setTagline(e.target.value)} />
      </label>
      <label className="admin-field">
        <span className="admin-field__label">描述</span>
        <textarea className="admin-input admin-textarea" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      <label className="admin-field">
        <span className="admin-field__label">站点网址</span>
        <input className="admin-input" value={url} onChange={(e) => setUrl(e.target.value)} />
      </label>

      <h3 className="admin-section-title" style={{ marginTop: 28 }}>首页文案</h3>
      <p className="admin-hint">留空就整段不渲染，不会留下空区块。</p>

      <label className="admin-field">
        <span className="admin-field__label">大图标题</span>
        <input
          className="admin-input"
          value={texts[SITE_TEXT_KEYS.heroVisualTitle] ?? ''}
          onChange={(e) => setText(SITE_TEXT_KEYS.heroVisualTitle, e.target.value)}
        />
      </label>
      <label className="admin-field">
        <span className="admin-field__label">大图角标</span>
        <input
          className="admin-input"
          value={texts[SITE_TEXT_KEYS.heroVisualMeta] ?? ''}
          onChange={(e) => setText(SITE_TEXT_KEYS.heroVisualMeta, e.target.value)}
          placeholder="例如：雨夜 · 03:17"
        />
      </label>

      <label className="admin-field admin-field--check">
        <input type="checkbox" checked={notesEnabled} onChange={(e) => setNotesEnabled(e.target.checked)} />
        <span>显示首页图集</span>
      </label>

      <label className="admin-field">
        <span className="admin-field__label">图集标题</span>
        <input
          className="admin-input"
          value={texts[SITE_TEXT_KEYS.notesTitle] ?? ''}
          onChange={(e) => setText(SITE_TEXT_KEYS.notesTitle, e.target.value)}
          disabled={!notesEnabled}
        />
      </label>
      <label className="admin-field">
        <span className="admin-field__label">图集说明</span>
        <textarea
          className="admin-input admin-textarea"
          rows={2}
          value={texts[SITE_TEXT_KEYS.notesLead] ?? ''}
          onChange={(e) => setText(SITE_TEXT_KEYS.notesLead, e.target.value)}
          disabled={!notesEnabled}
        />
      </label>

      <h3 className="admin-section-title" style={{ marginTop: 28 }}>页面文案</h3>
      <p className="admin-hint">这些是各页面的标题与提示语，留空就用代码里的默认文案。</p>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: 14 }}>
        <label className="admin-field">
          <span className="admin-field__label">文章页标题</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.postsTitle] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.postsTitle, e.target.value)}
            placeholder="全部文章"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">文章页副标题</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.postsLead] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.postsLead, e.target.value)}
            placeholder="留空显示「共 N 篇…」"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">文章页空状态</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.postsEmpty] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.postsEmpty, e.target.value)}
            placeholder="这里还没有文章…"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">归档页标题</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.archiveTitle] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.archiveTitle, e.target.value)}
            placeholder="归档"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">归档页副标题</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.archiveLead] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.archiveLead, e.target.value)}
            placeholder="留空显示「共 N 篇…」"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">标签页标题</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.tagsTitle] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.tagsTitle, e.target.value)}
            placeholder="标签"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">标签页副标题</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.tagsLead] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.tagsLead, e.target.value)}
            placeholder="留空显示「N 个标签…」"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">搜索页标题</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.searchTitle] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.searchTitle, e.target.value)}
            placeholder="搜索"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">搜索页提示</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.searchLead] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.searchLead, e.target.value)}
            placeholder="输入关键词…"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">404 标题</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.notFoundTitle] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.notFoundTitle, e.target.value)}
            placeholder="这一页不在了"
          />
        </label>
        <label className="admin-field">
          <span className="admin-field__label">404 说明</span>
          <input
            className="admin-input"
            value={texts[SITE_TEXT_KEYS.notFoundDesc] ?? ''}
            onChange={(e) => setText(SITE_TEXT_KEYS.notFoundDesc, e.target.value)}
            placeholder="可能是链接写错了…"
          />
        </label>
      </div>

      <button className="btn btn--primary" onClick={save} disabled={!loaded}>保存设置</button>

      <BackupPanel />

      <PasswordPanel />
    </div>
  );
}

/**
 * 内容导出：一键把文章、页面、图片打包成 zip。
 * 不想用这套系统了，或者只是想留个本地备份，都能直接拿走。
 */
function BackupPanel() {
  const [msg, setMsg] = useState('');
  const [busy, setBusy] = useState(false);

  async function exportAll() {
    setBusy(true);
    setMsg('');
    try {
      const { blob, filename } = await adminApi.exportContent();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      // 给浏览器一点时间把 blob 交给下载器，再释放
      setTimeout(() => URL.revokeObjectURL(url), 10_000);
      setMsg(`已导出 ${filename}（${Math.max(1, Math.round(blob.size / 1024))} KB）`);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : '导出失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="admin-password">
      <h3 className="admin-section-title">导出全部内容</h3>
      <p className="admin-hint">
        打包成 zip：每篇文章一个带 front matter 的 .md、独立页面、上传过的图片，外加 posts.json。
        解压后可以直接用编辑器「导入 Markdown 文件」逐篇导回。
      </p>

      <button className="btn btn--ghost" onClick={exportAll} disabled={busy}>
        {busy ? '正在打包…' : '导出全部内容（zip）'}
      </button>
      {msg ? <p className="admin-notice">{msg}</p> : null}
    </section>
  );
}

/**
 * 后台改密码。单独成块：它不参与上面的「保存设置」，点了就即时生效。
 */
function PasswordPanel() {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [msg, setMsg] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (next !== confirm) {
      setMsg('两次输入的新密码不一致');
      return;
    }
    if (next.length < 8) {
      setMsg('新密码至少 8 个字符');
      return;
    }
    setBusy(true);
    setMsg('');
    try {
      await adminApi.changePassword(current, next);
      setCurrent('');
      setNext('');
      setConfirm('');
      setMsg('密码已修改 ✓ 下次登录请用新密码');
    } catch (e) {
      setMsg(e instanceof Error ? e.message : '修改失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="admin-password">
      <h3 className="admin-section-title">修改密码</h3>
      <p className="admin-hint">
        改完立刻生效，不用重启容器。已经登录的其他设备不会被踢下线——
        想把所有设备都下线，需要换掉 .env.server 里的 ADMIN_SECRET 再重启一次。
      </p>

      <label className="admin-field">
        <span className="admin-field__label">当前密码</span>
        <input
          className="admin-input"
          type="password"
          autoComplete="current-password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
        />
      </label>
      <label className="admin-field">
        <span className="admin-field__label">新密码（至少 8 位）</span>
        <input
          className="admin-input"
          type="password"
          autoComplete="new-password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
        />
      </label>
      <label className="admin-field">
        <span className="admin-field__label">确认新密码</span>
        <input
          className="admin-input"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
        />
      </label>

      <button
        className="btn btn--ghost"
        onClick={submit}
        disabled={busy || !current || !next || !confirm}
      >
        {busy ? '提交中…' : '修改密码'}
      </button>
      {msg ? <p className="admin-notice">{msg}</p> : null}
    </section>
  );
}

/**
 * 评论审核面板：待审核 → 通过 / 隐藏 / 删除。
 * 公开站点默认所有新评论都进待审队列，避免广告机器人直接挂到文章下面。
 */
function CommentsPanel() {
  const [filter, setFilter] = useState<'pending' | 'published' | 'hidden' | ''>('pending');
  const [comments, setComments] = useState<Comment[]>([]);
  const [counts, setCounts] = useState<CommentCounts>({ pending: 0, published: 0, hidden: 0 });
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState('');
  const [busyId, setBusyId] = useState<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await adminApi.listComments(filter);
      setComments(data.comments);
      setCounts(data.counts);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : '加载评论失败');
    } finally {
      setLoading(false);
    }
  }, [filter]);

  useEffect(() => {
    void load();
  }, [load]);

  async function act(id: number, status: 'pending' | 'published' | 'hidden' | 'delete') {
    setBusyId(id);
    setMsg('');
    try {
      if (status === 'delete') {
        await adminApi.deleteComment(id);
        setMsg('评论已删除');
      } else {
        await adminApi.updateComment(id, status);
        setMsg(
          status === 'published' ? '评论已公开' : status === 'hidden' ? '评论已隐藏' : '评论已退回待审',
        );
      }
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : '操作失败');
    } finally {
      setBusyId(null);
    }
  }

  const tabs: { value: typeof filter; label: string; count: number }[] = [
    { value: 'pending', label: '待审核', count: counts.pending },
    { value: 'published', label: '已公开', count: counts.published },
    { value: 'hidden', label: '已隐藏', count: counts.hidden },
    { value: '', label: '全部', count: counts.pending + counts.published + counts.hidden },
  ];

  return (
    <>
      <div className="admin-page-intro">
        <div>
          <p className="admin-kicker">COMMENTS / MODERATION</p>
          <h1>评论审核</h1>
          <p>新评论默认进待审队列，通过之后才会出现在文章页。</p>
        </div>
        <div className="admin-page-stats">
          <strong>{counts.pending}</strong><span>条待审</span>
          <strong>{counts.published}</strong><span>条已公开</span>
        </div>
      </div>

      <div className="admin-quick-row">
        {tabs.map((t) => (
          <button
            key={t.label}
            className={`btn ${filter === t.value ? 'btn--primary' : 'btn--ghost'}`}
            onClick={() => setFilter(t.value)}
          >
            {t.label}（{t.count}）
          </button>
        ))}
      </div>

      {msg ? <p className="admin-notice">{msg}</p> : null}

      {loading ? (
        <p className="admin-loading">加载中…</p>
      ) : comments.length === 0 ? (
        <p className="admin-empty">
          {filter === 'pending' ? '没有待审核的评论。' : '这里还没有评论。'}
        </p>
      ) : (
        <ul className="admin-comment-list">
          {comments.map((comment) => (
            <li key={comment.id} className="admin-comment">
              <div className="admin-comment__head">
                <strong>{comment.author}</strong>
                <span className="admin-comment__slug">/{comment.post_slug}</span>
                <span className={`admin-status admin-status--${comment.status}`}>
                  {comment.status === 'pending' ? '待审' : comment.status === 'published' ? '已公开' : '已隐藏'}
                </span>
                <time className="admin-comment__time">
                  {new Date(comment.created_at).toLocaleString('zh-CN')}
                </time>
                {comment.email ? <span className="admin-comment__email">{comment.email}</span> : null}
              </div>
              <p className="admin-comment__body">{comment.body}</p>
              <div className="admin-comment__actions">
                {comment.status !== 'published' ? (
                  <button
                    className="btn btn--primary"
                    disabled={busyId === comment.id}
                    onClick={() => act(comment.id, 'published')}
                  >
                    通过
                  </button>
                ) : null}
                {comment.status !== 'hidden' ? (
                  <button
                    className="btn btn--ghost"
                    disabled={busyId === comment.id}
                    onClick={() => act(comment.id, 'hidden')}
                  >
                    隐藏
                  </button>
                ) : null}
                {comment.status !== 'pending' ? (
                  <button
                    className="btn btn--ghost"
                    disabled={busyId === comment.id}
                    onClick={() => act(comment.id, 'pending')}
                  >
                    退回待审
                  </button>
                ) : null}
                <button
                  className="btn btn--ghost"
                  disabled={busyId === comment.id}
                  onClick={() => act(comment.id, 'delete')}
                >
                  删除
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
