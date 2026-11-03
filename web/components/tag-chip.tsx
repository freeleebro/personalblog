import Link from 'next/link';
import type { CSSProperties } from 'react';

import type { Tag } from '@/lib/types';

/** 标签胶囊。颜色来自后端（按标签名稳定生成），这里只负责呈现。 */
export function TagChip({
  tag,
  active = false,
  showCount = false,
}: {
  tag: Pick<Tag, 'slug' | 'name' | 'color'> & { post_count?: number };
  active?: boolean;
  showCount?: boolean;
}) {
  return (
    <Link
      href={`/tags/${encodeURIComponent(tag.slug)}`}
      className={`chip${active ? ' chip--active' : ''}`}
      style={{ '--chip': tag.color } as CSSProperties}
    >
      {tag.name}
      {showCount && tag.post_count !== undefined ? (
        <span className="chip__count">{tag.post_count}</span>
      ) : null}
    </Link>
  );
}
