/**
 * 文件用途：用于验证种子场景自动化生命周期 API 测试。
 * 核心逻辑：使用确定性本地夹具执行 API 场景，断言响应、状态变化、负向分支和清理结果。
 * 关键注意事项：只有在本地账号、种子数据和清理步骤都成功时，才可作为对应流程的业务闭环证据。
 * 重构建议：继续把数据准备、断言 oracle 和清理逻辑拆清楚，便于补充故障注入或变异验证。
 */

const { expect } = require("chai");
const apiClient = require("../lib/api_client");
const seedData = require("../lib/seed_data");
const {
  expectBlockedOrSeeded,
  expectBusinessError,
  expectCreatedId,
  expectPagedListContains,
  expectSuccess,
} = require("../lib/response_assertions");

const ZERO_UUID = "00000000-0000-0000-0000-000000000000";

function sceneAutomationId(row) {
  return (
    row &&
    (row.id || row.ID || row.scene_automation_id || row.SceneAutomationID)
  );
}

function expectDryRunExplanation(data) {
  expect(data).to.be.an("object");
  expect(data)
    .to.have.property("summary")
    .that.is.a("string")
    .and.not.equal("");
  expect(data).to.have.property("dry_run").that.is.an("object");
  expect(data).to.have.property("reference_counts").that.is.an("object");
  expect(data).to.have.property("diagnostics").that.is.an("array").and.not
    .empty;
  expect(data).to.have.property("next_steps").that.is.an("array").and.not.empty;
}

function collectDryRunExplanationText(data) {
  return [
    ...(Array.isArray(data.errors) ? data.errors : []),
    ...(Array.isArray(data.diagnostics)
      ? data.diagnostics.map((item) => item && item.message)
      : []),
  ]
    .filter(Boolean)
    .join(" ");
}

describe("Seeded scene automation business coverage [24_seeded_scene_automations]", function () {
  this.timeout(45000);

  before(async function () {
    await apiClient.login("tenant_admin");
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it("creates, updates, switches, filters, and deletes a seeded scene automation", async function () {
    const seed = await seedData.ensureSceneAutomation("tenant_admin");
    let createdId = "";
    try {
      expectBlockedOrSeeded(seed, "scene automation seed");
      createdId = seed.id;

      const detailResp = await apiClient.get(
        "/scene_automations/detail/" + createdId,
        {},
        "tenant_admin",
      );
      expectSuccess(detailResp);
      expect(detailResp.data).to.include({
        id: createdId,
        name: seed.name,
        enabled: "N",
      });
      expect(detailResp.data.trigger_condition_groups).to.be.an("array").and.not
        .be.empty;
      expect(detailResp.data.actions).to.be.an("array").and.not.be.empty;
      expect(detailResp.data.actions[0]).to.include({
        action_type: "30",
        action_target: seed.alarmConfigId,
      });

      const listResp = await apiClient.get(
        "/scene_automations/list",
        {
          page: 1,
          page_size: 20,
          name: seed.name,
        },
        "tenant_admin",
      );
      expectSuccess(listResp);
      expectPagedListContains(
        listResp.data,
        (row) => sceneAutomationId(row) === createdId,
        "seeded automation row",
      );

      const alarmFilterResp = await apiClient.get(
        "/scene_automations/alarm",
        {
          page: 1,
          page_size: 20,
          device_id: seed.deviceId,
        },
        "tenant_admin",
      );
      expectSuccess(alarmFilterResp);
      expectPagedListContains(
        alarmFilterResp.data,
        (row) => sceneAutomationId(row) === createdId,
        "seeded alarm automation row",
      );

      // Keep the generated fixture inside the API's max=36 contract.
      const updatedName = seed.name.slice(0, 31) + "_upd";
      const updateResp = await apiClient.put(
        "/scene_automations",
        {
          ...seed.payload,
          id: createdId,
          name: updatedName,
          enabled: "Y",
        },
        "tenant_admin",
      );
      expectSuccess(updateResp);
      expect(updateResp.data).to.have.property(
        "scene_automation_id",
        createdId,
      );

      const updatedResp = await apiClient.get(
        "/scene_automations/detail/" + createdId,
        {},
        "tenant_admin",
      );
      expectSuccess(updatedResp);
      expect(updatedResp.data).to.include({
        id: createdId,
        name: updatedName,
        enabled: "Y",
      });

      expectSuccess(
        await apiClient.post(
          "/scene_automations/switch/" + createdId,
          {},
          "tenant_admin",
        ),
      );
      const switchedResp = await apiClient.get(
        "/scene_automations/detail/" + createdId,
        {},
        "tenant_admin",
      );
      expectSuccess(switchedResp);
      expect(switchedResp.data.enabled).to.equal("N");

      expectSuccess(
        await apiClient.delete(
          "/scene_automations/" + createdId,
          {},
          "tenant_admin",
        ),
      );
      const deletedId = createdId;
      createdId = "";
      expectBusinessError(
        await apiClient.get(
          "/scene_automations/detail/" + deletedId,
          {},
          "tenant_admin",
        ),
        101001,
      );
    } finally {
      if (createdId) {
        await seed.cleanup();
      }
    }
  });

  it("rejects non-existent scene automation ids with explicit product errors", async function () {
    expectBusinessError(
      await apiClient.get(
        "/scene_automations/detail/" + ZERO_UUID,
        {},
        "tenant_admin",
      ),
      101001,
    );
    expectBusinessError(
      await apiClient.delete(
        "/scene_automations/" + ZERO_UUID,
        {},
        "tenant_admin",
      ),
      101001,
    );
    expectBusinessError(
      await apiClient.post(
        "/scene_automations/switch/" + ZERO_UUID,
        {},
        "tenant_admin",
      ),
      101001,
    );
  });

  it("keeps create response id shape explicit for seeded automation helpers", async function () {
    const seed = await seedData.ensureSceneAutomation("tenant_admin");
    try {
      expectBlockedOrSeeded(seed, "scene automation seed");
      expectCreatedId(
        { code: 200, message: "ok", data: seed.row },
        "scene_automation_id",
      );
    } finally {
      await seed.cleanup();
    }
  });

  it("dry-runs a seeded scene automation payload without saving or executing it", async function () {
    const seed = await seedData.ensureSceneAutomation("tenant_admin");
    try {
      expectBlockedOrSeeded(seed, "scene automation seed");

      const dryRunResp = await apiClient.post(
        "/scene_automations/dry-run",
        seed.payload,
        "tenant_admin",
      );
      expectSuccess(dryRunResp);
      expectDryRunExplanation(dryRunResp.data);
      expect(dryRunResp.data.valid).to.equal(true);
      expect(dryRunResp.data.can_save).to.equal(true);
      expect(dryRunResp.data.errors).to.be.an("array").and.empty;
      expect(dryRunResp.data.blocking_errors).to.be.an("array").and.empty;
      expect(dryRunResp.data.dry_run).to.include({
        condition_group_count: 1,
        condition_count: 1,
        action_count: 1,
      });
      expect(dryRunResp.data.reference_counts).to.include({
        device: 1,
        alarm: 1,
      });
      expect(
        dryRunResp.data.diagnostics.some(
          (item) => item && item.severity === "success",
        ),
      ).to.equal(true);
    } finally {
      await seed.cleanup();
    }
  });

  it("dry-run explains invalid action references without accepting the payload", async function () {
    const seed = await seedData.ensureSceneAutomation("tenant_admin");
    try {
      expectBlockedOrSeeded(seed, "scene automation seed");

      const badPayload = {
        ...seed.payload,
        actions: seed.payload.actions.map((action, index) => ({
          ...action,
          action_target: index === 0 ? ZERO_UUID : action.action_target,
        })),
      };
      const dryRunResp = await apiClient.post(
        "/scene_automations/dry-run",
        badPayload,
        "tenant_admin",
      );

      expectSuccess(dryRunResp);
      expectDryRunExplanation(dryRunResp.data);
      expect(dryRunResp.data.valid).to.equal(false);
      expect(dryRunResp.data.can_save).to.equal(false);
      expect(dryRunResp.data.errors).to.be.an("array").and.not.empty;
      expect(dryRunResp.data.blocking_errors).to.be.an("array").and.not.empty;
      expect(
        dryRunResp.data.diagnostics.some(
          (item) => item && item.severity === "error",
        ),
      ).to.equal(true);
      expect(
        collectDryRunExplanationText(dryRunResp.data).toLowerCase(),
      ).to.match(
        /reference|action|alarm|target|invalid|missing|not found|record/,
      );
    } finally {
      await seed.cleanup();
    }
  });
});
