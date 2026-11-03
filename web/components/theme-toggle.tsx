'use client';

import { useEffect, useState } from 'react';

import { IconMoon, IconSun } from './icons';

type Theme = 'light' | 'dark';

/**
 * 主题切换。
 * 初值由 layout 里的内联脚本提前写到 <html data-theme>，避免首屏闪烁，
 * 所以这里直接读取现有属性，不再依赖 props。
 */
export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>('light');
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    const current = document.documentElement.getAttribute('data-theme');
    setTheme(current === 'dark' ? 'dark' : 'light');
    setMounted(true);
  }, []);

  function toggle() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark';
    setTheme(next);
    document.documentElement.setAttribute('data-theme', next);
    try {
      localStorage.setItem('theme', next);
    } catch {
      // 忽略隐私模式下的写入失败
    }
  }

  return (
    <button
      type="button"
      className="icon-btn"
      onClick={toggle}
      aria-label={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'}
      title={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'}
    >
      {mounted && theme === 'dark' ? <IconMoon width={17} height={17} /> : <IconSun width={17} height={17} />}
    </button>
  );
}
