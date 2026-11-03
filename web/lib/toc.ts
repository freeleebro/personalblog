import GithubSlugger from 'github-slugger';

/**
 * 从 Markdown 源码中提取目录。
 *
 * 这里刻意不自己实现 slug 算法，而是直接用 `github-slugger`——因为正文渲染
 * 走的 rehype-slug 用的就是它。任何"看起来差不多"的重实现都会在 Unicode
 * 标点上翻车：例如 `一、SQL 比我想象的更能扛事`，
 * github-slugger v2 会去掉顿号得到 `一sql-比我想象的更能扛事`，
 * 而按 v1 规则手写的实现会保留顿号，导致目录点不过去。
 */

export type TocItem = {
  id: string;
  text: string;
  level: number;
};

const FENCE = /^\s*(```|~~~)/;
const HEADING = /^(#{1,6})\s+(.+?)\s*#*\s*$/;

/** 去掉标题里的行内 Markdown 语法，还原出渲染后的可见文字 */
function stripInline(text: string): string {
  return text
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/`([^`]*)`/g, '$1')
    .replace(/(\*\*|__)(.*?)\1/g, '$2')
    .replace(/(\*|_)(.*?)\1/g, '$2')
    .replace(/~~(.*?)~~/g, '$1')
    .replace(/<[^>]+>/g, '')
    .trim();
}

/**
 * @param minLevel 收录的最小层级，默认从 h2 开始（h1 是文章标题）
 * @param maxLevel 收录的最大层级
 */
export function buildToc(markdown: string, minLevel = 2, maxLevel = 3): TocItem[] {
  const lines = markdown.replace(/\r\n?/g, '\n').split('\n');
  const slugger = new GithubSlugger();
  const items: TocItem[] = [];
  let inFence = false;

  for (const line of lines) {
    if (FENCE.test(line)) {
      inFence = !inFence;
      continue;
    }
    if (inFence) continue;

    const match = HEADING.exec(line);
    if (!match) continue;

    const level = match[1].length;
    const text = stripInline(match[2]);

    // 注意：必须对所有层级的标题都调用 slug()，包括被过滤掉的 h1/h4+。
    // 重名计数是按整篇文档的顺序累加的，跳过任何一级都会让后续 id 错位。
    const id = slugger.slug(text);

    if (level >= minLevel && level <= maxLevel) {
      items.push({ id, text, level });
    }
  }

  return items;
}
