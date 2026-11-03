import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  // 产出精简的可部署产物，Docker 镜像里只需要 .next/standalone + static
  output: 'standalone',

  // 安全响应头。
  // HSTS 交给 TLS 终结层（Caddy）设置——只有 https 响应上的 HSTS 才有效。
  // 静态资源的长缓存也由反代负责：Next 对 public/ 下的文件固定返回 max-age=0。
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
          { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
          { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
        ],
      },
    ];
  },
};

export default nextConfig;
