/**
 * 文件用途：ROADMAP TP-4「设备诊断 / GMQTT 管理界面 / Topic 映射页」的浏览器运行证据。
 *
 * 为什么要有这个文件：
 *   路线图把 TP-4 记为"已接线（四个组件均有挂载点）"，但挂载点只是源码里的 import +
 *   template 引用。这个仓库刚暴露过一整类"路由表有条目 / 组件有挂载点，但用户在浏览器里
 *   看不到"的问题（全路由白屏、菜单无行 → 守卫跳 403、v-if 前置条件不满足 → 组件静默不渲染）。
 *   因此本文件不复用"代码里有挂载点"这个结论，而是真开浏览器逐个断言四个组件的可见标记。
 *
 * 覆盖对象与可达路径：
 *   1. DeviceAccessGuide        设备创建向导第 2 步（/device/manage → 手动添加设备 → 下一步）
 *   2. ConnectionProofSteps     同上，嵌在 DeviceAccessGuide 内
 *   3. DeviceMqttDebugWorkbench DeviceAccessGuide.vue:439，v-if="deviceId && endpointKind==='mqtt'"；
 *      两条路径都已取证：设备详情页「接入」tab，以及设备创建向导 step2
 *      （step2 原先恒不渲染，见该用例上方注释；已随 TP-4 死代码修复一并修好）
 *   4. DeviceOperationsWorkbench 设备详情页（/device/details?d_id=<id>），v-if="canUseOwnerDetailActions"
 *
 * 断言口径：
 *   - 一律指向具体可见标记（data-testid 或按钮/标题/表单文案），
 *     不使用"页面上存在某个 div"这类恒真断言。
 *   - 中英双语正则，避免受默认语言影响。
 *   - 拿不到证据的组件用 test.fixme 保留并写清根因，不删除、不弱化。
 */

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');

/** 四个组件各自的可见标记。 */
const MARKER = {
  // DeviceAccessGuide 根容器 + 快速开始小标题
  accessGuideTestId: 'device-access-guide',
  accessGuideTitle: /复制即用快速开始|Copy-and-run quickstart/i,
  accessGuideIntro: /复制参数、运行测试上报|Connect this device by copying/i,
  // ConnectionProofSteps 根容器 + 证据区标签
  proofStepsTestId: 'device-access-guide-quickstart-steps',
  proofStepTitle: /使用这些凭证|Use these credentials/i,
  proofEvidenceLabel: /调试证据|Debug evidence/i,
  // DeviceMqttDebugWorkbench 根容器 + 卡片标题 + 主按钮
  mqttWorkbenchTestId: 'device-mqtt-debug-workbench',
  mqttWorkbenchTitle: /MQTT 调试工作台|MQTT debug workbench/i,
  mqttWorkbenchOpen: /开启调试会话|Open debug session/i,
  // 「上报主题」指标由 DeviceAccessGuide.vue:255 的 v-if="endpointKind === 'mqtt'" 控制，
  // 用来单独钉住 DeviceMqttDebugWorkbench 挂载条件的右半边
  accessGuideReportTopic: /上报主题|Report topic/i,
  // DeviceOperationsWorkbench 引导文案 + 首张卡片标题
  workbenchIntro: /设备运维工作台|device operations workbench/i,
  workbenchReadyCard: /接入就绪检查|Access readiness check/i,
  workbenchTelemetryCard: /实时数据|Realtime data/i
};

const WIZARD = {
  // 主按钮文案是 `custom.devicePage.addDevice`（"+Add Device"），不是 generate.manually-add-device
  addDevice: /添加设备|Add Device/i,
  // 下拉项文案是 `custom.devicePage.manualAdd`
  manualAdd: /^手动添加$|^Manual Add$/,
  // step1 提交按钮文案是 `custom.devicePage.saveAndNext`
  saveAndNext: /保存并下一步|Save and Next/i,
  deviceName: /设备名称|Device Name/i,
  pid: /PID/,
  step2Title: /第 2 步|Step 2/i
};

/** 12 位大写字母数字 PID，满足 step1 的 /^[A-Za-z0-9]{12}$/ 校验。 */
function randomPid() {
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ0123456789';
  let out = '';
  for (let i = 0; i < 12; i += 1) {
    out += alphabet[Math.floor(Math.random() * alphabet.length)];
  }
  return out;
}

/**
 * 删掉向导刚建出来的那台设备。
 * 必须清理的原因：step1 的 PID 是 12 位字母数字，正好命中
 * device-tab-plan.ts:38 的 RDI 判定，这些设备留在列表里会让别的 spec
 * 的 ensureDevice 选到 RDI 设备（详情页 tab 被裁掉 4 个），属于脏数据外溢。
 */
async function deleteWizardDevice(api, pid) {
  const listResp = await api.get('/device', { page: 1, page_size: 100 }, 'tenant_admin');
  const rows = seedData.listFromResponse(listResp);
  const row = rows.find(item => String(item.device_number || '') === pid);
  const id = row && seedData.pickId(row);
  if (id) await api.delete('/device/' + id, {}, 'tenant_admin');
}

/**
 * 走完「/device/manage → +添加设备 → 手动添加设备 → step1 填表 → 下一步」，
 * 停在设备创建向导第 2 步。返回 { page, pid }，pid 供调用方清理。
 */
async function openDeviceWizardStep2(page) {
  await page.goto('/device/manage', { waitUntil: 'domcontentloaded', timeout: 30000 });
  await expect(page.getByText(/403|404|Not Found|页面不存在/i)).toHaveCount(0);

  await page.getByRole('button', { name: WIZARD.addDevice }).first().click();
  await page.getByText(WIZARD.manualAdd).first().click();

  // step1：设备名称 + PID（12 位）→ 保存并下一步
  const drawer = page.locator('.n-drawer');
  await expect(drawer).toBeVisible({ timeout: 20000 });
  await drawer.locator('.n-form-item').filter({ hasText: WIZARD.deviceName }).locator('input').first()
    .fill('tp4-evidence-' + Date.now());
  const pid = randomPid();
  await drawer.locator('.n-form-item').filter({ hasText: WIZARD.pid }).locator('input').first().fill(pid);

  await drawer.getByRole('button', { name: WIZARD.saveAndNext }).first().click();

  // 等到第 2 步容器真的出现再返回，避免调用方在第 1 步上做断言
  await expect(page.getByTestId(MARKER.accessGuideTestId)).toBeVisible({ timeout: 30000 });
  return { page, pid };
}

test.describe('TP-4 device diagnostics surfaces [25_tp4_device_diagnostics]', () => {
  test.describe('device creation wizard step 2', () => {
    test.use({ role: 'tenant_admin' });
    // 向导要走完 deviceAdd API + 抽屉重渲染，默认 30s 单测超时不够
    test.slow();

    test('DeviceAccessGuide renders in the creation wizard step 2', async ({ rolePage, api }) => {
      const { page, pid } = await openDeviceWizardStep2(rolePage);

      try {
        await expect(page.getByTestId(MARKER.accessGuideTestId)).toBeVisible();
        await expect(page.getByText(MARKER.accessGuideTitle).first()).toBeVisible({ timeout: 20000 });
        // 接入指南的引导文案本身可见，证明不是"空壳容器"
        await expect(page.getByText(MARKER.accessGuideIntro).first()).toBeVisible({ timeout: 20000 });
        // 向导步骤条停在第 2 步
        await expect(page.getByText(WIZARD.step2Title).first()).toBeVisible({ timeout: 20000 });
      } finally {
        await deleteWizardDevice(api, pid);
      }
    });

    test('ConnectionProofSteps renders inside the wizard access guide', async ({ rolePage, api }) => {
      const { page, pid } = await openDeviceWizardStep2(rolePage);

      try {
        await expect(page.getByTestId(MARKER.proofStepsTestId)).toBeVisible({ timeout: 20000 });
        await expect(page.getByText(MARKER.proofStepTitle).first()).toBeVisible({ timeout: 20000 });
        await expect(page.getByText(MARKER.proofEvidenceLabel).first()).toBeVisible({ timeout: 20000 });
      } finally {
        await deleteWizardDevice(api, pid);
      }
    });

    /**
     * 2026-09-15 已修复并取证。此处保留根因，说明为什么必须有这条用例：
     *
     * 原状：这条路径在真实浏览器里拿不到 DeviceMqttDebugWorkbench。
     *   DeviceAccessGuide.vue:439 的挂载条件为
     *     v-if="deviceId && accessGuide.endpointKind === 'mqtt'"
     *   而 add-devices-step2.vue 调用 DeviceAccessGuide 时只传了
     *   access-guide / connect-info / has-unsaved-credentials，没有传 device-id，
     *   尽管 step2 自身有 props.device_id（add-devices-step2.vue:43）。
     *   → wizard 路径下 deviceId 恒为 undefined，v-if 恒假，组件静默不渲染：
     *     已挂载但永远不可达，等于死代码。
     *
     * 已排除的其它可能（当时逐条验证过）：
     *   - 不是 pageerror / 模块加载失败：同页 DeviceAccessGuide 与
     *     ConnectionProofSteps 都正常渲染，上面两条用例已证明；
     *   - 不是 403：/device/manage 与抽屉都能打开，步骤条也走到第 2 步；
     *   - endpointKind 不是阻碍：`endpointKind = protocol === 'HTTP' ? 'http' : 'mqtt'`
     *     （device-access-guide-state.ts:706），MQTT 设备取 'mqtt'；真正缺的是 device-id。
     *
     * 修法：add-devices-step2.vue 补 `:device-id="device_id || undefined"`
     * （用 || undefined 而非空串：空串会让 prop 变成"传了但为空"，排查时更难分辨）。
     * 副作用是有意为之——向导里现在会出现「开启调试会话」入口，这是原本就该有的能力。
     */
    test(
      'DeviceMqttDebugWorkbench renders in the creation wizard step 2',
      async ({ rolePage, api }) => {
        const { page, pid } = await openDeviceWizardStep2(rolePage);

        try {
          // 先钉住 v-if 的右半边 endpointKind === 'mqtt'：该指标由
          // DeviceAccessGuide.vue:255 的同款 v-if 控制，可见即证明不是 endpointKind 挡的。
          await expect(page.getByText(MARKER.accessGuideReportTopic).first()).toBeVisible({ timeout: 20000 });
          // 再钉住左半边 deviceId 与合取结果：只有 device-id 真传进去了才成立。
          await expect(page.getByTestId(MARKER.mqttWorkbenchTestId)).toBeVisible({ timeout: 20000 });
          await expect(page.getByText(MARKER.mqttWorkbenchTitle).first()).toBeVisible({ timeout: 20000 });
          await expect(
            page.getByRole('button', { name: MARKER.mqttWorkbenchOpen }).first()
          ).toBeVisible({ timeout: 20000 });
        } finally {
          await deleteWizardDevice(api, pid);
        }
      }
    );
  });

  test.describe('device details page', () => {
    test.use({ role: 'tenant_admin' });
    test.slow();

    /**
     * 建一台全新的、确定非 RDI 的设备。
     *
     * 为什么不能用 ensureDevice：device-tab-plan.ts:38 把 device_number 为
     * 12 位字母数字的设备判为 RDI，RDI 设备的详情页 tab 被裁剪成
     * 「详细信息 / 历史数据 / 报警信息 / 当前参数设定」四个（applyRdiCustomerTabs），
     * **没有「接入」tab**，join.vue（以及它内部的 DeviceMqttDebugWorkbench）根本不渲染。
     * 而本文件上面的向导用例创建的正是 12 位 PID 设备，会污染列表、让 ensureDevice 时好时坏。
     * API 直接创建的设备 device_number 是 UUID，稳定落到非 RDI 分支。
     */
    async function createPlainDevice(api) {
      await api.login('tenant_admin');
      return seedData.createSimulationDevice('tenant_admin');
    }

    test('DeviceOperationsWorkbench renders on the owned device details page', async ({ rolePage, api }) => {
      const seed = await createPlainDevice(api);

      try {
        await rolePage.goto('/device/details?d_id=' + encodeURIComponent(seed.id), {
          waitUntil: 'domcontentloaded',
          timeout: 30000
        });
        await expect(rolePage.getByText(/403|404|Not Found|页面不存在/i)).toHaveCount(0);

        // workbench 由 defineAsyncComponent 懒加载，且 v-if="canUseOwnerDetailActions"
        // 要求设备详情已按 d_id 加载完成
        await expect(rolePage.getByText(MARKER.workbenchIntro).first()).toBeVisible({ timeout: 30000 });
        await expect(rolePage.getByText(MARKER.workbenchReadyCard).first()).toBeVisible({ timeout: 20000 });
        await expect(rolePage.getByText(MARKER.workbenchTelemetryCard).first()).toBeVisible({ timeout: 20000 });
        // 组件根容器 + 卡片都在（不是只有标题文本）
        await expect(rolePage.locator('.device-operations-workbench')).toBeVisible({ timeout: 20000 });
        await expect(rolePage.locator('.device-operation-card').first()).toBeVisible({ timeout: 20000 });
      } finally {
        if (seed && typeof seed.cleanup === 'function') await seed.cleanup();
      }
    });

    test('DeviceMqttDebugWorkbench renders on the connection tab of the device details page', async ({
      rolePage,
      api
    }) => {
      // 设备详情页的「接入」tab（join.vue:369）才是真正给 DeviceAccessGuide 传 device-id 的地方，
      // 因此 DeviceMqttDebugWorkbench 的 v-if 在这里才有机会成立。
      const seed = await createPlainDevice(api);

      try {
        await rolePage.goto('/device/details?d_id=' + encodeURIComponent(seed.id), {
          waitUntil: 'domcontentloaded',
          timeout: 30000
        });
        await expect(rolePage.getByText(/403|404|Not Found|页面不存在/i)).toHaveCount(0);
        await expect(rolePage.getByText(MARKER.workbenchIntro).first()).toBeVisible({ timeout: 30000 });

        const joinTab = rolePage.locator('.n-tabs-tab').filter({ hasText: /^接入$|^Connection$/ }).first();
        await joinTab.click();

        await expect(rolePage.getByTestId(MARKER.accessGuideTestId)).toBeVisible({ timeout: 30000 });
        await expect(rolePage.getByTestId(MARKER.proofStepsTestId)).toBeVisible({ timeout: 20000 });
        await expect(rolePage.getByTestId(MARKER.mqttWorkbenchTestId)).toBeVisible({ timeout: 30000 });
        await expect(rolePage.getByText(MARKER.mqttWorkbenchTitle).first()).toBeVisible({ timeout: 20000 });
        await expect(
          rolePage.getByRole('button', { name: MARKER.mqttWorkbenchOpen }).first()
        ).toBeVisible({ timeout: 20000 });
      } finally {
        if (seed && typeof seed.cleanup === 'function') await seed.cleanup();
      }
    });
  });
});
