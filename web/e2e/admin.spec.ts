import { expect, test } from '@playwright/test';

/** 后台登录 → 工作台。token 存 localStorage，刷新后仍保持登录。 */
test('后台登录后进入工作台，刷新保持登录', async ({ page }) => {
  await page.goto('/admin');
  await expect(page.getByRole('heading', { name: '博客管理后台' })).toBeVisible();

  await page.getByLabel('用户名').fill('admin');
  await page.getByLabel('密码').fill('test-password');
  await page.getByRole('button', { name: '登录' }).click();

  await expect(page.getByRole('button', { name: '新建文章' })).toBeVisible();
  await expect(page.getByText('演示文章：用于端到端测试').first()).toBeVisible();

  await page.reload();
  await expect(page.getByRole('button', { name: '新建文章' })).toBeVisible();
});

test('密码错误时停留在登录页并显示错误', async ({ page }) => {
  await page.goto('/admin');
  await page.getByLabel('用户名').fill('admin');
  await page.getByLabel('密码').fill('wrong-password');
  await page.getByRole('button', { name: '登录' }).click();

  await expect(page.getByText('用户名或密码不正确')).toBeVisible();
  await expect(page.getByRole('heading', { name: '博客管理后台' })).toBeVisible();
});

/** 发文主路径：新建 → 填标题和正文 → 保存 → 后端收到 POST。 */
test('新建文章可以保存', async ({ page }) => {
  const saves: string[] = [];
  page.on('request', (req) => {
    if (req.method() === 'POST' && req.url().endsWith('/api/v1/admin/posts')) saves.push(req.url());
  });

  await page.goto('/admin');
  await page.getByLabel('用户名').fill('admin');
  await page.getByLabel('密码').fill('test-password');
  await page.getByRole('button', { name: '登录' }).click();
  await page.getByRole('button', { name: '新建文章' }).click();

  await page.getByLabel('标题').fill('端到端测试文章');
  await page.locator('.admin-textarea--md').fill('# 端到端测试\n\n这是一次保存测试。');
  await page.getByRole('button', { name: '保存' }).click();

  await expect(page.locator('.admin-notice').first()).toContainText('已保存');
  expect(saves).toHaveLength(1);
});