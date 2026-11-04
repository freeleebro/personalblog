import { expect, test } from '@playwright/test';

/** 访客在文章页提交评论：前端应提示「待审核」，并且真的向接口发了 POST。 */
test('访客提交评论后提示待审核', async ({ page }) => {
  const posts: string[] = [];
  page.on('request', (req) => {
    if (req.method() === 'POST' && req.url().includes('/comments')) posts.push(req.url());
  });

  await page.goto('/posts/demo-post');

  const form = page.locator('.comment-form');
  await expect(form).toBeVisible();

  await form.getByLabel('昵称').fill('测试访客');
  await form.getByLabel('评论').fill('这是一条端到端测试评论。');
  await form.getByRole('button', { name: '发表评论' }).click();

  await expect(form.getByText('评论已提交，站长审核通过后会显示在这里')).toBeVisible();
  await expect(form.getByLabel('昵称')).toHaveValue('');
  expect(posts).toHaveLength(1);
});

/** 空表单由浏览器原生校验拦住，不应该发请求。 */
test('空表单不会提交到后端', async ({ page }) => {
  const posts: string[] = [];
  page.on('request', (req) => {
    if (req.method() === 'POST' && req.url().includes('/comments')) posts.push(req.url());
  });

  await page.goto('/posts/demo-post');
  const form = page.locator('.comment-form');
  await form.getByRole('button', { name: '发表评论' }).click();

  await expect(form.getByLabel('昵称')).toHaveJSProperty('validity.valid', false);
  await expect(form.getByText('评论已提交，站长审核通过后会显示在这里')).toHaveCount(0);
  expect(posts).toHaveLength(0);
});