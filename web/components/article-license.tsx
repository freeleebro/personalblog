import { formatDate } from '@/lib/format';
import type { Post, Site } from '@/lib/types';

const LICENSE_URL = 'https://creativecommons.org/licenses/by-nc-sa/4.0/deed.zh-hans';
const LICENSE_NAME = '署名-非商业性使用-相同方式共享 4.0 国际';

/**
 * 文末版权声明。
 *
 * 不开 CC 的「保留所有权利」对个人博客没什么意义，反而挡住正常的转载；
 * 所以这里默认写死 BY-NC-SA：署名、非商用、以相同协议共享。
 * 作者名优先取文章作者，其次站点作者，最后退回站点名。
 */
export function ArticleLicense({ post, site }: { post: Post; site: Site | null }) {
  const author = post.author?.name || site?.author?.name || site?.name || '站长';
  const siteName = site?.name ?? '';
  const base = (site?.url ?? '').replace(/\/+$/, '');
  const path = `/posts/${encodeURIComponent(post.slug)}`;
  // 站点 URL 没配时，站内相对路径仍然能说明「原文在哪」，不至于渲染成空。
  const canonical = base ? `${base}${path}` : path;

  return (
    <aside className="article-license" aria-label="版权声明">
      <h2 className="article-license__title">版权声明</h2>
      <p className="article-license__text">
        本文由 {author} 创作，采用{' '}
        <a href={LICENSE_URL} target="_blank" rel="noopener noreferrer license">
          CC {LICENSE_NAME} 许可协议
        </a>{' '}
        授权。转载请注明作者与原文链接，且不得用于商业用途。
      </p>
      <ul className="article-license__list">
        <li>
          <span className="article-license__key">作者</span>
          <span>{author}</span>
        </li>
        <li>
          <span className="article-license__key">原文</span>
          {base ? (
            <a href={canonical} rel="bookmark">
              {canonical}
            </a>
          ) : (
            <span>{canonical}</span>
          )}
        </li>
        <li>
          <span className="article-license__key">最后更新</span>
          <time dateTime={post.updated_at}>{formatDate(post.updated_at)}</time>
        </li>
        {siteName ? (
          <li>
            <span className="article-license__key">站点</span>
            <span>{siteName}</span>
          </li>
        ) : null}
      </ul>
    </aside>
  );
}
