import type { Metadata } from 'next';
import type { ReactNode } from 'react';

import './admin.css';

export const metadata: Metadata = {
  title: '管理后台',
  robots: { index: false, follow: false },
};

/**
 * 管理后台的独立布局：不渲染博客的头部/粒子/页脚，
 * 只保留一个干净的后台工作区。
 */
export default function AdminLayout({ children }: { children: ReactNode }) {
  return <>{children}</>;
}
