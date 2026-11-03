'use client';

import { useEffect, useState } from 'react';

import { adminApi, getToken, setToken, AdminError } from '@/lib/admin-api';
import { AdminEditor } from './editor';
import { AdminLogin } from './login';

/**
 * 管理后台入口。
 * 纯客户端：登录后把 token 存 localStorage，之后所有请求带 token。
 */
export default function AdminPage() {
  const [authed, setAuthed] = useState<boolean | null>(null); // null=检查中

  useEffect(() => {
    const t = getToken();
    if (!t) {
      setAuthed(false);
      return;
    }
    adminApi
      .whoami()
      .then(() => setAuthed(true))
      .catch((e: unknown) => {
        if (e instanceof AdminError && e.status === 401) setToken(null);
        setAuthed(false);
      });
  }, []);

  if (authed === null) {
    return (
      <div className="admin-shell">
        <p className="admin-loading">正在检查登录状态…</p>
      </div>
    );
  }

  if (!authed) {
    return <AdminLogin onAuthed={() => setAuthed(true)} />;
  }

  return <AdminEditor onLogout={() => { setToken(null); setAuthed(false); }} />;
}
