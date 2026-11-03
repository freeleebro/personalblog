'use client';

import { useState } from 'react';

import { adminApi, setToken } from '@/lib/admin-api';

/** 登录表单。 */
export function AdminLogin({ onAuthed }: { onAuthed: () => void }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const res = await adminApi.login(username, password);
      setToken(res.token);
      onAuthed();
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="admin-shell admin-login">
      <form className="admin-login__card" onSubmit={submit}>
        <h1 className="admin-login__title">博客管理后台</h1>
        <p className="admin-login__sub">登录以管理你的文章与站点</p>

        <label className="admin-field">
          <span className="admin-field__label">用户名</span>
          <input
            className="admin-input"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            autoFocus
          />
        </label>

        <label className="admin-field">
          <span className="admin-field__label">密码</span>
          <input
            className="admin-input"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
          />
        </label>

        {error ? <p className="admin-error">{error}</p> : null}

        <button className="btn btn--primary admin-login__submit" type="submit" disabled={loading}>
          {loading ? '登录中…' : '登录'}
        </button>
      </form>
    </div>
  );
}
