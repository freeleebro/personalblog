import type { Metadata } from 'next';
import Link from 'next/link';

import { Markdown } from '@/components/markdown';
import { getPage } from '@/lib/api';
import { formatDate } from '@/lib/format';

export const metadata: Metadata = {
  title: 'Now',
  description: '此刻在做的事、在学的东西、在读的书。',
};

/**
 * Now 页：一张「最近在忙什么」的快照，不是关于页。
 *
 * 内容存在 pages 表里（slug = now），和「关于」一样在后台编辑，
 * 不新开一套数据结构——它本质就是一个独立页面。
 */
export default async function NowPage() {
  let page = null;
  try {
    page = await getPage('now');
  } catch {
    page = null;
  }

  return (
    <div className="shell page">
      <div className="page__head">
        <h1 className="page__title">{page?.title ?? 'Now'}</h1>
        <p className="page__desc">
          {page
            ? `最近更新于 ${formatDate(page.updated_at)} · 这一页记录此刻，不是履历。`
            : '这一页记录此刻，不是履历。'}
        </p>
      </div>

      {page ? (
        <Markdown>{page.body_md}</Markdown>
      ) : (
        <div className="panel" style={{ textAlign: 'center', padding: '40px 20px' }}>
          <p style={{ margin: 0, fontWeight: 600 }}>还没有写 Now</p>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--fg-muted)' }}>
            在后台「页面」里新建一条 slug 为 <code>now</code> 的记录，这里就会显示内容。
          </p>
          <Link href="/" className="btn btn--ghost" style={{ marginTop: 16 }}>
            回到首页
          </Link>
        </div>
      )}
    </div>
  );
}
