import { isValidElement, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeSlug from 'rehype-slug';
import rehypeHighlight from 'rehype-highlight';

import { CodeBlock } from './code-block';

/** 从 <pre> 的子元素上读出 rehype-highlight 写入的 language-xxx */
function extractLang(children: ReactNode): string {
  const first = Array.isArray(children) ? children[0] : children;
  if (isValidElement<{ className?: string }>(first)) {
    const match = /language-([\w+#.-]+)/.exec(first.props.className ?? '');
    if (match) return match[1];
  }
  return '';
}

/**
 * Markdown 渲染器（服务端组件）。
 *
 * 渲染发生在服务端：首屏就是完整 HTML，不需要在客户端下载解析器，
 * 搜索引擎也能直接拿到内容。这里唯一需要客户端的部分是代码块的复制按钮。
 */
export function Markdown({ children }: { children: string }) {
  return (
    <div className="prose">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeSlug, [rehypeHighlight, { detect: false, ignoreMissing: true }]]}
        components={{
          pre: ({ children: inner }) => <CodeBlock lang={extractLang(inner)}>{inner}</CodeBlock>,
          a: ({ href, children: label }) => {
            const external = !!href && /^https?:\/\//.test(href);
            return (
              <a
                href={href}
                {...(external ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
              >
                {label}
              </a>
            );
          },
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
