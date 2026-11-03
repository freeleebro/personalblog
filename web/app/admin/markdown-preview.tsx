'use client';

import { isValidElement, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeSlug from 'rehype-slug';
import rehypeHighlight from 'rehype-highlight';

import { CodeBlock } from '@/components/code-block';

function extractLang(children: ReactNode): string {
  const first = Array.isArray(children) ? children[0] : children;
  if (isValidElement<{ className?: string }>(first)) {
    const match = /language-([\w+#.-]+)/.exec(first.props.className ?? '');
    if (match) return match[1];
  }
  return '';
}

/**
 * 客户端 Markdown 预览（供管理后台编辑器实时预览用）。
 * 与服务端 Markdown 组件保持相同的渲染配置。
 */
export function MarkdownPreview({ children }: { children: string }) {
  return (
    <div className="prose">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeSlug, [rehypeHighlight, { detect: false, ignoreMissing: true }]]}
        components={{
          pre: ({ children: inner }) => <CodeBlock lang={extractLang(inner)}>{inner}</CodeBlock>,
          a: ({ href, children: label }) => (
            <a href={href} {...(/^https?:\/\//.test(href ?? '') ? { target: '_blank', rel: 'noopener noreferrer' } : {})}>
              {label}
            </a>
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
