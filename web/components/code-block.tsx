'use client';

import { useRef, useState, type ReactNode } from 'react';

/**
 * 代码块外壳：语言标签 + 复制按钮。
 * 必须放在客户端，因为要用到 navigator.clipboard。
 */
export function CodeBlock({ lang, children }: { lang: string; children: ReactNode }) {
  const preRef = useRef<HTMLPreElement>(null);
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    const text = preRef.current?.textContent ?? '';
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      // navigator.clipboard 只在 HTTPS / localhost 下可用，失败时静默处理
    }
  }

  return (
    <div className="code-block">
      <div className="code-block__head">
        <span className="code-block__lang">{lang || 'text'}</span>
        <button
          type="button"
          className="code-block__copy"
          data-copied={copied}
          onClick={handleCopy}
          aria-label="复制代码"
        >
          {copied ? '已复制' : '复制'}
        </button>
      </div>
      <pre ref={preRef}>{children}</pre>
    </div>
  );
}
