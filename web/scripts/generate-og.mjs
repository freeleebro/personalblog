// 生成分享卡片图（Open Graph）：1200×630 的 JPEG。
//
// 为什么不在运行时用 next/og 动态生成：satori 渲染中文需要额外塞一份 CJK 字体
// （动辄十几 MB），对一个个人博客来说不划算。这里改成「一张固定的品牌图 +
// 每篇文章用封面图」，既没有运行时开销，微信 / Twitter 也都能正常显示。
//
// 用法：cd web && npm run og-image
// 换图：替换 src 里的底图，或调整下面的 sharp 参数后重新执行。

import { mkdir, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import sharp from 'sharp';

const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, '..');

const src = process.argv[2] ?? path.join(webRoot, 'public', 'anime-night-1200.webp');
const out = path.join(webRoot, 'public', 'og-image.jpg');

/** OG 图的推荐尺寸，Twitter / 微信 / Slack 都用这个比例。 */
const WIDTH = 1200;
const HEIGHT = 630;

await mkdir(path.dirname(out), { recursive: true });

await sharp(src)
  .resize(WIDTH, HEIGHT, { fit: 'cover', position: 'attention' })
  // 轻微压暗边缘，白字放上去也压得住（页面上的标题本身不烤进图片）。
  .modulate({ brightness: 0.96 })
  .jpeg({ quality: 82, mozjpeg: true, progressive: true })
  .toFile(out);

const info = await stat(out);
console.log(`已生成 ${path.relative(webRoot, out)}（${WIDTH}×${HEIGHT}，${Math.round(info.size / 1024)} KB）`);