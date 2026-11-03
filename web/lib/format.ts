/** 展示层格式化工具。全部按中文习惯输出。 */

const MONTHS = ['一月', '二月', '三月', '四月', '五月', '六月', '七月', '八月', '九月', '十月', '十一月', '十二月'];

/** 2026-08-30 → 2026 年 8 月 30 日 */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return `${d.getFullYear()} 年 ${d.getMonth() + 1} 月 ${d.getDate()} 日`;
}

/** 2026-08-30 → 08-30 */
export function formatDateShort(iso: string | null | undefined): string {
  if (!iso) return '--';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '--';
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** 用于 <time dateTime> 属性 */
export function isoDate(iso: string | null | undefined): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function formatMonth(month: number): string {
  return MONTHS[month - 1] ?? `${month} 月`;
}

/** 1234 → 1.2k */
export function compactNumber(n: number): string {
  if (n < 1000) return String(n);
  if (n < 10000) return `${(n / 1000).toFixed(1).replace(/\.0$/, '')}k`;
  return `${Math.round(n / 1000)}k`;
}

/** 字数：按「万」为单位更适合中文阅读 */
export function formatChars(n: number): string {
  if (n < 10000) return `${n} 字`;
  return `${(n / 10000).toFixed(1).replace(/\.0$/, '')} 万字`;
}

export function readingLabel(minutes: number): string {
  return `${minutes} 分钟阅读`;
}

/** 按标签名生成稳定的柔和色，用于标签默认配色 */
export function fallbackColor(seed: string): string {
  let hash = 0;
  for (let i = 0; i < seed.length; i += 1) {
    hash = (hash * 31 + seed.charCodeAt(i)) >>> 0;
  }
  const palette = ['#2563eb', '#0891b2', '#0d9488', '#65a30d', '#ca8a04', '#ea580c', '#dc2626', '#db2777', '#7c3aed', '#4f46e5'];
  return palette[hash % palette.length];
}

function pad(n: number): string {
  return String(n).padStart(2, '0');
}
