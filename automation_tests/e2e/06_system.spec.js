/**
 * System-setting browser closure. The test edits one existing data-retention
 * policy, proves the persisted API state and refreshed table state, then
 * restores the original policy in cleanup.
 */

const { test, expect } = require('./fixtures');

function isDataPolicyResponse(response, method) {
  try {
    const url = new URL(response.url());
    return response.request().method() === method
      && url.pathname.endsWith('/datapolicy');
  } catch {
    return false;
  }
}

test.describe('system module', () => {
  test.describe.configure({ timeout: 45000 });
  test.use({ role: 'super_admin' });

  test('data cleanup policy edited in the browser persists through the API and refreshed table', async ({ rolePage, api }) => {
    await api.login('super_admin');
    const listBefore = await api.get('/datapolicy', { page: 1, page_size: 10 }, 'super_admin');
    expect(listBefore.code).toBe(200);
    expect(listBefore.data).toEqual(expect.objectContaining({
      total: expect.any(Number),
      list: expect.any(Array)
    }));
    const original = listBefore.data.list.find(row => row && row.id);
    expect(original).toEqual(expect.objectContaining({
      id: expect.stringMatching(/\S/),
      retention_days: expect.any(Number),
      enabled: expect.stringMatching(/^[12]$/)
    }));

    const originalRemark = original.remark == null ? '' : String(original.remark);
    const updatedRemark = 'browser-policy-edit-' + Date.now();

    try {
      const initialListPromise = rolePage.waitForResponse(
        response => isDataPolicyResponse(response, 'GET'),
        { timeout: 20000 }
      );
      await rolePage.goto('/management/setting', { waitUntil: 'domcontentloaded' });
      const initialListResponse = await initialListPromise;
      expect(initialListResponse.status()).toBe(200);
      const initialListBody = await initialListResponse.json();
      expect(initialListBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          list: expect.arrayContaining([
            expect.objectContaining({
              id: original.id,
              retention_days: original.retention_days,
              enabled: original.enabled,
              remark: original.remark
            })
          ])
        })
      }));

      const firstRow = rolePage.locator('.n-data-table-base-table-body .n-data-table-tr').filter({
        // Seed policy IDs are the short values "a"/"b"; substring matching
        // also matches words such as "Data" in both rows. Bind to the exact
        // first-cell text instead.
        has: rolePage.getByText(original.id, { exact: true })
      });
      await expect(firstRow).toHaveCount(1);
      await expect(firstRow).toBeVisible();
      await firstRow.getByRole('button', { name: /Edit|编辑/i }).click();

      const modal = rolePage.locator('.n-modal').filter({ hasText: /Edit|编辑/i });
      await expect(modal).toBeVisible();
      const retentionInput = modal.locator('.n-input-number input');
      await expect(retentionInput).toHaveValue(String(original.retention_days));
      const remarkInput = modal.locator('textarea');
      await remarkInput.fill(updatedRemark);

      const updatePromise = rolePage.waitForResponse(
        response => isDataPolicyResponse(response, 'PUT'),
        { timeout: 20000 }
      );
      const refreshPromise = rolePage.waitForResponse(
        response => isDataPolicyResponse(response, 'GET'),
        { timeout: 20000 }
      );
      await modal.getByRole('button', { name: /Edit|编辑/i }).click();
      const updateResponse = await updatePromise;
      expect(updateResponse.status()).toBe(200);
      const updateBody = await updateResponse.json();
      expect(updateBody.code).toBe(200);
      const refreshResponse = await refreshPromise;
      expect(refreshResponse.status()).toBe(200);

      const listAfter = await api.get('/datapolicy', { page: 1, page_size: 10 }, 'super_admin');
      expect(listAfter.code).toBe(200);
      expect(listAfter.data.list).toEqual(expect.arrayContaining([
        expect.objectContaining({ id: original.id, remark: updatedRemark })
      ]));
      await expect(rolePage.getByText(updatedRemark, { exact: true })).toBeVisible();
    } finally {
      const restoreResponse = await api.put('/datapolicy', {
        id: original.id,
        retention_days: original.retention_days,
        enabled: original.enabled,
        remark: originalRemark
      }, 'super_admin');
      expect(restoreResponse.code).toBe(200);
      const restoredList = await api.get('/datapolicy', { page: 1, page_size: 10 }, 'super_admin');
      expect(restoredList.code).toBe(200);
      expect(restoredList.data.list).toEqual(expect.arrayContaining([
        expect.objectContaining({
          id: original.id,
          retention_days: original.retention_days,
          enabled: original.enabled,
          remark: originalRemark
        })
      ]));
    }
  });
});
