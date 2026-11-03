'use client';

// 注意：window 上的 keydown 监听要用 DOM 的 KeyboardEvent，
// 而 input 的 onKeyDown 要用 React 的泛型版本，因此这里把 React 的改名引入。
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from 'react';
import { useRouter } from 'next/navigation';

import { IconClose, IconSearch } from './icons';
import { formatDate } from '@/lib/format';
import type { Post } from '@/lib/types';

type SearchResponse = { data: Post[] };

/**
 * 搜索面板。
 * 走同源 /api/search 代理到 Go 后端，避免浏览器直连 API 产生跨域与
 * 「后端地址暴露在客户端」两个问题。
 */
export function SearchDialog() {
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [results, setResults] = useState<Post[]>([]);
  const [loading, setLoading] = useState(false);
  const [cursor, setCursor] = useState(0);

  const close = useCallback(() => {
    setOpen(false);
    setKeyword('');
    setResults([]);
    setCursor(0);
  }, []);

  // 快捷键：/ 或 Ctrl/Cmd+K 打开，Esc 关闭
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const target = event.target as HTMLElement | null;
      const typing =
        target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable;

      if ((event.key === 'k' && (event.metaKey || event.ctrlKey)) || (event.key === '/' && !typing)) {
        event.preventDefault();
        setOpen(true);
        return;
      }
      if (event.key === 'Escape') close();
    }

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [close]);

  useEffect(() => {
    if (open) {
      // 等面板挂载后再聚焦，否则 ref 还是空的
      const id = window.setTimeout(() => inputRef.current?.focus(), 20);
      return () => window.clearTimeout(id);
    }
  }, [open]);

  // 输入防抖
  useEffect(() => {
    if (!open) return;
    const trimmed = keyword.trim();
    if (trimmed === '') {
      setResults([]);
      setLoading(false);
      return;
    }

    setLoading(true);
    const controller = new AbortController();
    const id = window.setTimeout(async () => {
      try {
        const res = await fetch(`/api/search?q=${encodeURIComponent(trimmed)}`, {
          signal: controller.signal,
        });
        const json = (await res.json()) as SearchResponse;
        setResults(json.data ?? []);
        setCursor(0);
      } catch {
        /* 取消或网络失败，静默处理 */
      } finally {
        setLoading(false);
      }
    }, 220);

    return () => {
      window.clearTimeout(id);
      controller.abort();
    };
  }, [keyword, open]);

  function go(post: Post) {
    close();
    router.push(`/posts/${encodeURIComponent(post.slug)}`);
  }

  function onInputKeyDown(event: ReactKeyboardEvent<HTMLInputElement>) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setCursor((c) => Math.min(c + 1, Math.max(0, results.length - 1)));
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setCursor((c) => Math.max(0, c - 1));
    } else if (event.key === 'Enter') {
      event.preventDefault();
      if (results[cursor]) {
        go(results[cursor]);
      } else if (keyword.trim()) {
        close();
        router.push(`/search?q=${encodeURIComponent(keyword.trim())}`);
      }
    }
  }

  return (
    <>
      <button type="button" className="icon-btn" onClick={() => setOpen(true)} aria-label="搜索文章">
        <IconSearch width={17} height={17} />
        <span className="icon-btn__hint">搜索</span>
      </button>

      {open ? (
        <div className="search-overlay" onClick={close} role="presentation">
          <div
            className="search-panel"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-label="搜索文章"
          >
            <div className="search-panel__form">
              <IconSearch width={18} height={18} style={{ color: 'var(--fg-subtle)' }} />
              <input
                ref={inputRef}
                className="search-panel__input"
                value={keyword}
                placeholder="搜索标题、摘要或正文…"
                onChange={(event) => setKeyword(event.target.value)}
                onKeyDown={onInputKeyDown}
              />
              <button type="button" className="icon-btn" onClick={close} aria-label="关闭搜索">
                <IconClose width={16} height={16} />
              </button>
            </div>

            <div className="search-results">
              {results.length > 0 ? (
                results.map((post, index) => (
                  <button
                    key={post.id}
                    type="button"
                    className="search-result"
                    data-active={index === cursor}
                    onClick={() => go(post)}
                    onMouseEnter={() => setCursor(index)}
                    style={{ width: '100%', textAlign: 'left', background: 'none', border: 0, cursor: 'pointer' }}
                  >
                    <div className="search-result__title">{post.title}</div>
                    <div className="search-result__meta">
                      {formatDate(post.published_at)} · {post.tags.map((t) => t.name).join(' / ')}
                    </div>
                  </button>
                ))
              ) : (
                <div className="search-empty">
                  {loading ? '搜索中…' : keyword.trim() ? '没有找到匹配的文章' : '输入关键词开始搜索'}
                </div>
              )}
            </div>

            <div className="search-panel__foot">
              <span>
                <kbd>↑</kbd> <kbd>↓</kbd> 选择
              </span>
              <span>
                <kbd>Enter</kbd> 打开
              </span>
              <span>
                <kbd>Esc</kbd> 关闭
              </span>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
