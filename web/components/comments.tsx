'use client';

import { useCallback, useEffect, useState } from 'react';

import { formatDate } from '@/lib/format';
import type { Comment } from '@/lib/types';

type ListPayload = {
  enabled: boolean;
  comments: Comment[];
  total: number;
};

const EMPTY_FORM = { author: '', email: '', body: '' };

/**
 * 文章页评论区。
 *
 * 数据从同源代理 /api-proxy 拉取，避免把后端地址暴露给浏览器；
 * 提交后默认进入审核队列，只有站长在后台点「通过」才会公开。
 */
export function Comments({ slug }: { slug: string }) {
  const [list, setList] = useState<ListPayload>({ enabled: true, comments: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState(EMPTY_FORM);
  // 蜜罐字段：视觉上完全隐藏，真人不会填，机器人通常会把所有 input 填满。
  const [honeypot, setHoneypot] = useState('');
  const [sending, setSending] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const res = await fetch(`/api-proxy/api/v1/posts/${encodeURIComponent(slug)}/comments`, {
        cache: 'no-store',
      });
      if (!res.ok) throw new Error(`加载评论失败（${res.status}）`);
      const payload = (await res.json()) as { data: ListPayload };
      setList(payload.data);
    } catch {
      // 评论加载失败不该影响阅读正文，静默降级成「暂无评论」。
      setList((prev) => ({ ...prev, enabled: true }));
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    void load();
  }, [load]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setNotice(null);

    if (form.author.trim() === '') {
      setError('请填写昵称');
      return;
    }
    if (form.body.trim().length < 2) {
      setError('评论内容太短');
      return;
    }

    setSending(true);
    try {
      const res = await fetch(`/api-proxy/api/v1/posts/${encodeURIComponent(slug)}/comments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...form, website: honeypot }),
      });
      const payload = (await res.json().catch(() => null)) as
        | { data?: { status?: string }; error?: { message?: string } }
        | null;

      if (!res.ok) {
        setError(payload?.error?.message ?? `提交失败（${res.status}）`);
        return;
      }

      setForm(EMPTY_FORM);
      setHoneypot('');
      if (payload?.data?.status === 'published') {
        setNotice('评论已发布');
        await load();
      } else {
        setNotice('评论已提交，站长审核通过后会显示在这里');
      }
    } catch {
      setError('网络异常，请稍后再试');
    } finally {
      setSending(false);
    }
  }

  if (!list.enabled) return null;

  return (
    <section className="comments" aria-label="评论">
      <div className="comments__head">
        <h2 className="comments__title">评论</h2>
        <span className="comments__count">{list.total}</span>
      </div>

      {loading ? (
        <p className="comments__empty">正在加载评论…</p>
      ) : list.comments.length === 0 ? (
        <p className="comments__empty">还没有评论，来说点什么吧。</p>
      ) : (
        <ul className="comments__list">
          {list.comments.map((comment) => (
            <li key={comment.id} className="comment">
              <div className="comment__meta">
                <strong className="comment__author">{comment.author}</strong>
                <time className="comment__time" dateTime={comment.created_at}>
                  {formatDate(comment.created_at)}
                </time>
              </div>
              <p className="comment__body">{comment.body}</p>
            </li>
          ))}
        </ul>
      )}

      <form className="comment-form" onSubmit={submit}>
        <div className="comment-form__row">
          <label className="comment-form__field">
            <span>昵称</span>
            <input
              className="comment-form__input"
              value={form.author}
              onChange={(event) => setForm({ ...form, author: event.target.value })}
              maxLength={40}
              required
            />
          </label>
          <label className="comment-form__field">
            <span>邮箱（可选，不会公开）</span>
            <input
              className="comment-form__input"
              type="email"
              value={form.email}
              onChange={(event) => setForm({ ...form, email: event.target.value })}
              maxLength={120}
            />
          </label>
        </div>

        <label className="comment-form__field">
          <span>评论</span>
          <textarea
            className="comment-form__input comment-form__textarea"
            value={form.body}
            onChange={(event) => setForm({ ...form, body: event.target.value })}
            rows={5}
            maxLength={2000}
            required
          />
        </label>

        {/* 蜜罐：对真人隐藏，对自动填表脚本可见 */}
        <div className="comment-form__hp" aria-hidden="true">
          <label>
            <span>网站</span>
            <input
              tabIndex={-1}
              autoComplete="off"
              value={honeypot}
              onChange={(event) => setHoneypot(event.target.value)}
            />
          </label>
        </div>

        {error ? <p className="comment-form__error">{error}</p> : null}
        {notice ? <p className="comment-form__notice">{notice}</p> : null}

        <div className="comment-form__actions">
          <button className="btn btn--primary" type="submit" disabled={sending}>
            {sending ? '提交中…' : '发表评论'}
          </button>
          <span className="comment-form__hint">提交后需站长审核，广告链接会被隐藏。</span>
        </div>
      </form>
    </section>
  );
}