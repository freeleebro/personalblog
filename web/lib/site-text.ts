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

  /** 文章列表页大标题（按标签筛选时会被「标签：xxx」覆盖） */
  postsTitle: 'posts.title',
  /** 文章列表页标题下的一句话 */
  postsLead: 'posts.lead',
  /** 文章列表页空状态文案 */
  postsEmpty: 'posts.empty',
  /** 归档页大标题 */
  archiveTitle: 'archive.title',
  /** 归档页标题下的一句话 */
  archiveLead: 'archive.lead',
  /** 标签页大标题 */
  tagsTitle: 'tags.title',
  /** 标签页标题下的一句话，留空则显示「N 个标签，共 M 次引用」 */
  tagsLead: 'tags.lead',
  /** 搜索页大标题 */
  searchTitle: 'search.title',
  /** 搜索框为空时的提示语 */
  searchLead: 'search.lead',
  /** 404 页标题 */
  notFoundTitle: 'notfound.title',
  /** 404 页说明 */
  notFoundDesc: 'notfound.desc',
} as const;

/**
 * 代码里的中性兜底。没有在后台改过的键就用这里的值，
 * 所以「克隆下来直接跑」也不会出现空标题。
 */
export const SITE_TEXT_DEFAULTS: Record<string, string> = {
  [SITE_TEXT_KEYS.postsTitle]: '全部文章',
  [SITE_TEXT_KEYS.postsLead]: '按发布时间倒序，点标题读全文。',
  [SITE_TEXT_KEYS.postsEmpty]: '这里还没有文章，去后台上传一篇吧。',
  [SITE_TEXT_KEYS.archiveTitle]: '归档',
  [SITE_TEXT_KEYS.archiveLead]: '按年份浏览全部文章。',
  [SITE_TEXT_KEYS.tagsTitle]: '标签',
  [SITE_TEXT_KEYS.tagsLead]: '',
  [SITE_TEXT_KEYS.searchTitle]: '搜索',
  [SITE_TEXT_KEYS.searchLead]: '输入关键词，在标题、摘要与正文中查找。',
  [SITE_TEXT_KEYS.notFoundTitle]: '这一页不在了',
  [SITE_TEXT_KEYS.notFoundDesc]: '可能是链接写错了，也可能这篇文章被删掉了。不妨回首页看看别的。',
};

/** 取一条站点文案；没有就返回兜底值（再没有就返回 key 对应的默认文案）。 */
export function siteText(site: Site | null | undefined, key: string, fallback?: string): string {
  const value = site?.texts?.[key];
  if (typeof value === 'string' && value.trim() !== '') return value;
  if (fallback !== undefined) return fallback;
  return SITE_TEXT_DEFAULTS[key] ?? '';
}

/** 取一个布尔开关，约定用 '1' / '0' 存。 */
export function siteFlag(site: Site | null | undefined, key: string): boolean {
  return site?.texts?.[key] === '1';
}
