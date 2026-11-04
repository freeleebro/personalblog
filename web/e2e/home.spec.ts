import { expect, test } from '@playwright/test';

/** 首页走真实 SSR：站点名、文章卡片都应来自后端数据。 */
test('首页渲染站点信息与文章列表', async ({ page }) => {
  await page.goto('/');

  await expect(page).toHaveTitle(/测试博客/);
  await expect(page.getByRole('link', { name: /测试博客/ }).first()).toBeVisible();
  await expect(page.getByRole('heading', { name: '记录，然后分享' })).toBeVisible();
  await expect(page.getByText('演示文章：用于端到端测试').first()).toBeVisible();
});

test('文章详情页渲染正文与评论区', async ({ page }) => {
  await page.goto('/posts/demo-post');

  await expect(page.getByRole('heading', { name: '演示文章：用于端到端测试' }).first()).toBeVisible();
  await expect(page.getByText('正文第一段。')).toBeVisible();
  await expect(page.locator('.comments')).toBeVisible();
  await expect(page.getByRole('button', { name: '发表评论' })).toBeVisible();
});