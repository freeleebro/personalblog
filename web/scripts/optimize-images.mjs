// 一次性把 web/public 里的原始插图压成按显示尺寸裁好的 WebP。
// 用法（在 web/ 下）：node scripts/optimize-images.mjs
//
// 原图分辨率远超实际显示尺寸（例如 2266×1488 只用在 400px 宽的卡片上），
// 直接用会让首屏多下载 1MB 以上。这里按显示宽度出图，文件名里带宽度，
// 反代就能对它们发 immutable 长缓存。
import { existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import sharp from 'sharp';

const publicDir = join(dirname(fileURLToPath(import.meta.url)), '..', 'public');

/** @type {{src: string, out: string, width: number, quality: number}[]} */
const jobs = [
  // hero 区在 600~640px 宽显示，2x 屏给 1200；上面压了一层深色渐变，画质可以压低
  { src: 'anime-night.jpg', out: 'anime-night-1200.webp', width: 1200, quality: 66 },
  // 精选卡右侧 + 列表卡片缩略图，实际显示 ≤640px，同样有遮罩
  { src: 'anime-night.jpg', out: 'anime-night-640.webp', width: 640, quality: 70 },
  { src: 'anime-crossing.jpg', out: 'anime-crossing-800.webp', width: 800, quality: 74 },
  { src: 'anime-underground.jpg', out: 'anime-underground-800.webp', width: 800, quality: 74 },
];

let total = 0;
for (const job of jobs) {
  const src = join(publicDir, job.src);
  if (!existsSync(src)) {
    console.warn(`[skip] 找不到 ${job.src}`);
    continue;
  }
  const out = join(publicDir, job.out);
  const info = await sharp(src)
    .resize({ width: job.width, withoutEnlargement: true })
    .webp({ quality: job.quality, effort: 6 })
    .toFile(out);
  total += info.size;
  console.log(
    `${job.out.padEnd(30)} ${String(info.width).padStart(4)}×${String(info.height).padEnd(5)} ${(info.size / 1024).toFixed(1)} KB`,
  );
}
console.log(`\n合计 ${(total / 1024).toFixed(1)} KB`);
