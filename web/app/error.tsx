'use client';

import { useEffect } from 'react';

/** 全局错误边界。生产环境不展示堆栈，只给一条可操作的提示。 */
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('[app error]', error);
  }, [error]);

  return (
    <div className="shell">
      <div className="state">
        <div className="state__code">500</div>
        <h1 className="state__title">页面出错了</h1>
        <p className="state__desc">
          多数情况下是后端服务没连上。确认 <code>docker compose ps</code> 中的容器已经
          running 之后，再重试一次。
        </p>
        <div className="state__actions">
          <button type="button" className="btn btn--primary" onClick={reset}>
            重新加载
          </button>
          <a href="/" className="btn btn--ghost">
            回到首页
          </a>
        </div>
      </div>
    </div>
  );
}
