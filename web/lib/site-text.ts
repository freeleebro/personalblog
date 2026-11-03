import type { Site } from './types';

/**
 * 页面上那些「编辑会想改」的短文案都放在站点设置里，键名集中在这里，
 * 前后端各认这一份，新增一句文案不用动数据库。
 *
 * 约定：值留空就表示「用代码里的中性兜底，或干脆不渲染这一段」。
 */
export const SITE_TEXT_KEYS = {
  /** 首页大图上的标题，留空则不显示 */
  heroVisualTitle: 'home.visual_title',
  /** 首页大图右下角的小字，例如「雨夜 · 03:17」 */
  heroVisualMeta: 'home.visual_meta',
  /** 首页图集标题 */
  notesTitle: 'home.notes_title',
  /** 首页图集说明 */
  notesLead: 'home.notes_lead',
  /** '1' 才显示首页图集；默认不显示，避免空标题的空区块 */
  notesEnabled: 'home.notes_enabled',
} as const;

/** 取一条站点文案；没有就返回兜底值。 */
export function siteText(site: Site | null | undefined, key: string, fallback = ''): string {
  const value = site?.texts?.[key];
  return typeof value === 'string' && value.trim() !== '' ? value : fallback;
}

/** 取一个布尔开关，约定用 '1' / '0' 存。 */
export function siteFlag(site: Site | null | undefined, key: string): boolean {
  return site?.texts?.[key] === '1';
}
