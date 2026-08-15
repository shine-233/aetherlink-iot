/**
 * Real exception-route interaction coverage.
 *
 * These pages are boundary evidence rather than customer business closure,
 * but they still exercise the rendered error state and its recovery action.
 */

const { test, expect } = require('./fixtures');

const exceptionRoutes = [
  { route: '/403', code: '403' },
  { route: '/404', code: '404' },
  { route: '/500', code: '500' }
];

for (const { route, code } of exceptionRoutes) {
  test(`exception page ${code} renders its recovery action and returns home`, async ({ rolePage }) => {
    await rolePage.goto(route, { waitUntil: 'domcontentloaded' });
    await expect(rolePage).toHaveURL(new RegExp(`${route.replace('/', '\\/')}$`));

    const illustration = rolePage.locator('img').first();
    await expect(illustration).toBeVisible({ timeout: 15000 });
    await expect(illustration).toHaveAttribute('src', /.+/);

    const backHome = rolePage.getByRole('button', { name: /Back to Home|返回首页/i });
    await expect(backHome).toBeVisible();
    await backHome.click();
    await expect(rolePage).not.toHaveURL(new RegExp(`${route.replace('/', '\\/')}$`), { timeout: 15000 });
    await expect(rolePage).toHaveURL(/\/(?:home)?(?:\?.*)?$/);
  });
}
