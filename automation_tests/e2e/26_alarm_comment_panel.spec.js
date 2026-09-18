/**
 * 文件用途：告警评论面板的浏览器运行期证据（ROADMAP TB-1 第一片）。
 *
 * 为什么需要这一条：面板此前是**死文件**——组件已写、后端五层齐备、tests/43 的
 * 10 条 API 契约用例全绿，但全仓 0 处 import，用户在界面上根本碰不到它。
 * 单测（`AlarmCommentPanel.test.ts` 10 条）只能证明组件逻辑对，证明不了
 * "用户能在详情页里真的用它发一条评论"——按 ROADMAP §4，UI 行为必须有运行证据。
 *
 * 关键注意事项：
 *   1. 必须先 seed 一条告警历史，否则列表为空、没有「详情」按钮可点。
 *      沿用 04_alarm.spec.js 的 ensureSceneAlarmHistory，并在 finally 里 cleanup。
 *   2. 面板挂在详情弹窗内（n-modal 懒渲染），所以必须先点开「详情」再断言可见。
 *   3. 断言走双语正则（发表/Post、删除/Delete），避免受默认语言切换影响。
 *   4. 用例结束前把自己发的评论删掉，不在共享验证库里留垃圾。
 *   5. 前端 preview 未启动时本用例会失败——这是预期的：没有运行期证据就不算闭环，
 *      不要改成跳过。
 */

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');

const ALARM_ROUTE = '/alarm/warning-message';

function isApiResponseFor(response, path, method = 'GET') {
  try {
    const url = new URL(response.url());
    // Dev 走 Vite /proxy-default 前缀，preview 走 /api/v1；
    // 后端资源后缀在两种传输下是稳定的，所以只比对后缀。
    return response.request().method() === method && url.pathname.endsWith(path);
  } catch {
    return false;
  }
}

test.describe('Alarm comment panel [26_alarm_comment_panel]', () => {
  test.use({ role: 'tenant_admin' });

  test('detail modal exposes the comment panel and round-trips a comment through the real API', async ({
    rolePage,
    api
  }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    if (seed.blocked) {
      throw new Error('scene alarm history seed is blocked: ' + (seed.reason || 'missing deterministic seed prerequisite'));
    }

    // seed + 首次冷启动的 SPA 加载会超出默认 30s，这里放宽到 120s。
    test.setTimeout(120000);

    let marker = '';
    try {
      const listPromise = rolePage.waitForResponse(response => isApiResponseFor(response, '/alarm/info/history'), {
        timeout: 60000
      });
      await rolePage.goto(ALARM_ROUTE, { waitUntil: 'domcontentloaded' });
      await listPromise;

      // 1) 打开种子告警的详情弹窗
      const seededRow = rolePage.locator('.n-data-table-tr').filter({ hasText: seed.alarmConfigName });
      await expect(seededRow).toHaveCount(1);
      await seededRow.getByTestId('alarm-details').click();

      // 2) 面板真的挂载出来了（此前它是谁都碰不到的死文件）
      const panel = rolePage.getByTestId('alarm-comment-panel');
      await expect(panel).toBeVisible({ timeout: 20000 });
      await expect(rolePage.getByText(/告警评论|Alarm comments/i).first()).toBeVisible();

      // 3) 发一条评论，并断言请求真的打到了后端的 comment 端点
      marker = `e2e-comment-${Date.now()}`;
      const postPromise = rolePage.waitForResponse(r => isApiResponseFor(r, '/comment', 'POST'), { timeout: 20000 });
      await panel.locator('textarea').fill(marker);
      await panel.getByRole('button', { name: /发表|Post/i }).click();

      const postResponse = await postPromise;
      expect(postResponse.status()).toBe(200);
      const postBody = await postResponse.json();
      expect(postBody.code).toBe(200);

      // 4) 评论回到列表里（提交后会重新拉取，这一步同时证明重拉生效）
      await expect(panel.getByText(marker)).toBeVisible({ timeout: 20000 });

      // 5) 删掉自己发的评论，保持共享验证库干净；顺带覆盖删除路径
      await panel.getByRole('button', { name: /删除|Delete/i }).first().click();
      await expect(panel.getByText(marker)).toHaveCount(0, { timeout: 20000 });
    } finally {
      await seed.cleanup();
    }
  });
});
