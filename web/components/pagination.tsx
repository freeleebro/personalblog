import Link from 'next/link';

/** 分页控件。用链接而不是按钮，方便爬虫跟进和中间键新开。 */
export function Pagination({
  page,
  totalPages,
  buildHref,
}: {
  page: number;
  totalPages: number;
  buildHref: (page: number) => string;
}) {
  if (totalPages <= 1) return null;

  const pages = pageWindow(page, totalPages);

  return (
    <nav className="pagination" aria-label="分页">
      <Link
        href={buildHref(Math.max(1, page - 1))}
        className="pagination__link"
        data-disabled={page <= 1}
        aria-label="上一页"
      >
        上一页
      </Link>

      {pages.map((p, i) =>
        p === null ? (
          <span key={`gap-${i}`} className="pagination__link" style={{ border: 0, background: 'none' }}>
            …
          </span>
        ) : (
          <Link key={p} href={buildHref(p)} className="pagination__link" data-active={p === page}>
            {p}
          </Link>
        ),
      )}

      <Link
        href={buildHref(Math.min(totalPages, page + 1))}
        className="pagination__link"
        data-disabled={page >= totalPages}
        aria-label="下一页"
      >
        下一页
      </Link>
    </nav>
  );
}

/** 生成形如 [1, null, 4, 5, 6, null, 12] 的页码窗口 */
function pageWindow(current: number, total: number): (number | null)[] {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1);
  }

  const out: (number | null)[] = [1];
  const start = Math.max(2, current - 1);
  const end = Math.min(total - 1, current + 1);

  if (start > 2) out.push(null);
  for (let p = start; p <= end; p += 1) out.push(p);
  if (end < total - 1) out.push(null);
  out.push(total);

  return out;
}
