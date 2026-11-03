import Link from 'next/link';

export default function NotFound() {
  return (
    <div className="shell">
      <div className="state">
        <div className="state__code">404</div>
        <h1 className="state__title">这一页不在了</h1>
        <p className="state__desc">
          可能是链接写错了，也可能这篇文章被我删掉了。不妨回首页看看别的。
        </p>
        <div className="state__actions">
          <Link href="/" className="btn btn--primary">
            回到首页
          </Link>
          <Link href="/posts" className="btn btn--ghost">
            浏览全部文章
          </Link>
        </div>
      </div>
    </div>
  );
}
