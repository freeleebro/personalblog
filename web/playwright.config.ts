import { defineConfig, devices } from '@playwright/test';

const PORT = Number(process.env.E2E_WEB_PORT ?? 3100);
// 必须用 localhost：Next dev 会把 127.0.0.1 当作跨域来源，
// 拦掉 /_next/hmr 等开发资源，页面就永远不会 hydrate。
const BASE_URL = `http://localhost:${PORT}`;

/**
 * e2e 用 `next dev` 起真实页面，后端换成 e2e/mock-api.mjs。
 *
 * 不跑 `next build`：dev 已经能验证 SSR + 客户端交互，而且 CI 里省一次
 * 完整的生产构建；生产构建本身由 `npm run build` 在同一个 CI job 里覆盖。
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: 'node e2e/mock-api.mjs',
      port: 8080,
      reuseExistingServer: !process.env.CI,
      env: { E2E_WEB_PORT: String(PORT) },
    },
    {
      command: `npm run dev -- --port ${PORT}`,
      url: BASE_URL,
      reuseExistingServer: !process.env.CI,
      timeout: 180_000,
      env: { API_BASE_URL: 'http://127.0.0.1:8080', NEXT_TELEMETRY_DISABLED: '1' },
    },
  ],
});