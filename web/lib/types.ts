// 与 Go 后端 JSON 结构一一对应的类型定义。
// 后端是唯一数据源，这里刻意保持「字段名 = JSON 字段名」，不做二次映射。

export type Author = {
  id: number;
  slug: string;
  name: string;
  avatar_url: string;
  bio: string;
};

export type Tag = {
  id: number;
  slug: string;
  name: string;
  color: string;
  description?: string;
  post_count: number;
};

export type Post = {
  id: number;
  slug: string;
  title: string;
  summary: string;
  /** 仅详情接口返回，列表接口为空 */
  body_md?: string;
  cover_color: string;
  /** 封面图，空字符串表示回退到色块 */
  cover_url: string;
  status: string;
  featured: boolean;
  reading_minutes: number;
  views: number;
  author?: Author;
  tags: Tag[];
  published_at: string | null;
  created_at: string;
  updated_at: string;
};

export type Page = {
  slug: string;
  title: string;
  body_md: string;
  updated_at: string;
};

export type Social = {
  label: string;
  url: string;
};

export type Stats = {
  posts: number;
  tags: number;
  chars: number;
  last_published_at: string | null;
};

export type Site = {
  name: string;
  tagline: string;
  description: string;
  url: string;
  author: Author | null;
  socials: Social[] | null;
  // 后台「站点设置」里可编辑的短文案，键名见 lib/site-text.ts。
  texts?: Record<string, string>;
  stats: Stats;
  tags: Tag[];
  featured?: Post | null;
  recent: Post[];
};

export type MonthCount = {
  year: number;
  month: number;
  count: number;
};

export type ArchiveYear = {
  year: number;
  count: number;
  months: MonthCount[];
};

export type Meta = {
  page?: number;
  per_page?: number;
  total?: number;
  total_pages?: number;
};

export type PostDetail = {
  post: Post;
  prev: Post | null;
  next: Post | null;
};

export type TagDetail = {
  tag: Tag;
  posts: Post[];
};
