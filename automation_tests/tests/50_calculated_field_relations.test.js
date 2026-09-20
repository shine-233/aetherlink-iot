/**
 * 文件用途：TB-2 计算字段关联实体聚合（Calculated Fields with Entity Relations & Hierarchies）契约测试。
 *
 * 对标 ThingsBoard 4.3 LTS 最新计算字段能力：
 *   1. 参数与边界校验：related_agg 与 propagation 配置校验，非法函数、方向或缺失源键/目标拒绝；
 *   2. 显式 device_ids 聚合：多关联设备最新数值聚合（sum, avg, min, max, count）；
 *   3. 实体关系图谱动态发现：基于 entity_relations (Contains, etc.) 自动发现下游设备并聚合遥测；
 *   4. 网关-子设备层级发现：基于 devices.parent_id 自动发现子设备并进行下行汇总；
 *   5. 实体间遥测传播（propagation）：主设备派生遥测自动同步广播至关联目标设备；
 *   6. 租户边界与隔离：跨租户不可见、不可改、不可查，关系图谱解析严格限制在各自租户内。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');

const SUITE = 'Calculated Fields with Entity Relations [50_calculated_field_relations]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const CALCULATED_FIELDS_PATH = '/calculated_fields';
const ENTITY_RELATIONS_PATH = '/entity-relations';

function calcFieldPath(id) {
  return `${CALCULATED_FIELDS_PATH}/${id}`;
}

describe(SUITE, function () {
  this.timeout(180000);

  const cleanups = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 50_calculated_field_relations.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);
  });

  after(async function () {
    for (let i = cleanups.length - 1; i >= 0; i--) {
      try {
        await cleanups[i]();
      } catch (err) {
        console.warn('Cleanup step error:', err.message);
      }
    }
    apiClient.clearAllTokens();
  });

  // Helper: 创建独立测试模板
  async function createTestTemplate(accountKey = ACCOUNT) {
    const name = seedData.makeRunLabel('tb2_calc_template');
    const resp = await apiClient.post('/device/template', {
      name,
      author: 'automation',
      version: '1.0.0',
      description: 'TB-2 test template'
    }, accountKey);
    expectSuccess(resp);
    const templateId = seedData.pickId(resp.data);
    cleanups.push(async () => {
      await apiClient.delete('/device/template/' + templateId, {}, accountKey);
    });
    return templateId;
  }

  // Helper: 创建设备配置并绑定模板
  async function createTestConfig(templateId, accountKey = ACCOUNT) {
    const name = seedData.makeRunLabel('tb2_calc_config');
    const resp = await apiClient.post('/device_config', {
      name,
      device_template_id: templateId,
      device_type: '1',
      protocol_type: 'MQTT',
      voucher_type: 'BASIC',
      device_conn_type: 'A',
      protocol_config: '{}'
    }, accountKey);
    expectSuccess(resp);
    const configId = seedData.pickId(resp.data);
    cleanups.push(async () => {
      await apiClient.delete('/device_config/' + configId, {}, accountKey);
    });
    return configId;
  }

  // Helper: 创建仿真设备并绑定配置
  async function createConfiguredDevice(configId, accountKey = ACCOUNT, parentId = '') {
    const deviceSeed = await seedData.createSimulationDevice(accountKey);
    cleanups.push(async () => {
      await deviceSeed.cleanup();
    });

    if (configId) {
      const bindResp = await apiClient.put('/device/update/config', {
        device_id: deviceSeed.id,
        device_config_id: configId
      }, accountKey);
      expectSuccess(bindResp);
    }

    if (parentId) {
      const updateResp = await apiClient.put('/device', {
        id: deviceSeed.id,
        name: deviceSeed.row.name || 'sub-dev',
        parent_id: parentId
      }, accountKey);
      expectSuccess(updateResp);
    }

    return deviceSeed;
  }

  describe('1. API Validation & Advanced Config Schema', function () {
    let templateId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
    });

    it('rejects related_agg without source_key or func', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-agg-1',
        device_template_id: templateId,
        output_key: 'agg_out',
        type: 'related_agg',
        config: { func: 'sum', device_ids: ['d1'] }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('source_key');
    });

    it('rejects related_agg with unsupported aggregate function', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-agg-2',
        device_template_id: templateId,
        output_key: 'agg_out',
        type: 'related_agg',
        config: { source_key: 'power', func: 'median', device_ids: ['d1'] }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('func');
    });

    it('rejects related_agg without target devices or relation config', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-agg-3',
        device_template_id: templateId,
        output_key: 'agg_out',
        type: 'related_agg',
        config: { source_key: 'power', func: 'sum' }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('device_ids or relation_type');
    });

    it('rejects propagation with invalid direction', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-prop-1',
        device_template_id: templateId,
        output_key: 'prop_out',
        type: 'propagation',
        config: { source_key: 'power', device_ids: ['d1'], direction: 'sideways' }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('direction');
    });
  });

  describe('2. Advanced Calculated Field CRUD & Toggle', function () {
    let templateId = null;
    let fieldId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
    });

    it('creates a related_agg calculated field successfully', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'valid-agg-crud',
        device_template_id: templateId,
        output_key: 'sum_power',
        type: 'related_agg',
        config: {
          source_key: 'power',
          func: 'sum',
          relation_type: 'Contains',
          use_relation: true
        },
        enabled: false,
        remark: 'testing related_agg'
      }, ACCOUNT);
      expectSuccess(resp);
      expect(resp.data).to.be.an('object');
      fieldId = resp.data.id;
      expect(fieldId).to.be.a('string').and.not.equal('');
      expect(resp.data.type).to.equal('related_agg');
      expect(resp.data.output_key).to.equal('sum_power');
      expect(resp.data.enabled).to.equal(false);
    });

    it('updates the related_agg calculated field config', async function () {
      const updateResp = await apiClient.put(calcFieldPath(fieldId), {
        name: 'valid-agg-crud-updated',
        device_template_id: templateId,
        output_key: 'avg_power',
        type: 'related_agg',
        config: {
          source_key: 'power',
          func: 'avg',
          relation_type: 'Contains',
          use_relation: true
        },
        remark: 'updated to avg'
      }, ACCOUNT);
      expectSuccess(updateResp);
      expect(updateResp.data.output_key).to.equal('avg_power');
      expect(updateResp.data.type).to.equal('related_agg');
    });

    it('toggles the enabled state of the calculated field', async function () {
      const toggleResp = await apiClient.put(`${calcFieldPath(fieldId)}/toggle`, {
        enabled: true
      }, ACCOUNT);
      expectSuccess(toggleResp);
      expect(toggleResp.data.enabled).to.equal(true);
    });

    it('reads the updated calculated field via list & detail', async function () {
      const detailResp = await apiClient.get(calcFieldPath(fieldId), {}, ACCOUNT);
      expectSuccess(detailResp);
      expect(detailResp.data.id).to.equal(fieldId);
      expect(detailResp.data.output_key).to.equal('avg_power');
      expect(detailResp.data.type).to.equal('related_agg');
      expect(detailResp.data.enabled).to.equal(true);

      const listResp = await apiClient.get(CALCULATED_FIELDS_PATH, {
        page: 1,
        page_size: 10,
        device_template_id: templateId
      }, ACCOUNT);
      expectSuccess(listResp);
      const found = listResp.data.list.find(item => item.id === fieldId);
      expect(found, 'created field in list').to.be.an('object');
      expect(found.type).to.equal('related_agg');
    });

    it('deletes the calculated field', async function () {
      const delResp = await apiClient.delete(calcFieldPath(fieldId), {}, ACCOUNT);
      expectSuccess(delResp);

      const getResp = await apiClient.get(calcFieldPath(fieldId), {}, ACCOUNT);
      expectBusinessError(getResp, 100404);
    });
  });

  describe('3. Explicit device_ids Telemetry Aggregation Execution', function () {
    let templateId = null;
    let configId = null;
    let devMain = null;
    let devB = null;
    let devC = null;
    let fieldId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
      configId = await createTestConfig(templateId, ACCOUNT);
      devMain = await createConfiguredDevice(configId, ACCOUNT);
      devB = await createConfiguredDevice('', ACCOUNT);
      devC = await createConfiguredDevice('', ACCOUNT);

      // 先为关联设备 devB、devC 注入遥测值
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        devB.id,
        { power_metric: 40.0 },
        ACCOUNT
      );
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        devC.id,
        { power_metric: 60.0 },
        ACCOUNT
      );

      // 创建并启用聚合字段（sum 40 + 60 = 100）
      const createResp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'explicit-sum-power',
        device_template_id: templateId,
        output_key: 'total_cluster_power',
        type: 'related_agg',
        config: {
          source_key: 'power_metric',
          func: 'sum',
          device_ids: [devB.id, devC.id]
        },
        enabled: true
      }, ACCOUNT);
      expectSuccess(createResp);
      fieldId = createResp.data.id;
    });

    after(async function () {
      if (fieldId) {
        try { await apiClient.delete(calcFieldPath(fieldId), {}, ACCOUNT); } catch (_) {}
      }
    });

    it('aggregates telemetry from explicitly declared device_ids upon trigger uplink', async function () {
      // 主设备上报一次触发遥测
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        devMain.id,
        { trigger_pulse: 1.0 },
        ACCOUNT
      );

      // 轮询主设备最新遥测，验证 total_cluster_power 派生成功且值为 100
      let verified = false;
      for (let i = 0; i < 15; i++) {
        const curResp = await apiClient.get('/telemetry/datas/current/' + devMain.id, {}, ACCOUNT);
        if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
          const item = curResp.data.find(r => r.key === 'total_cluster_power');
          if (item && Number(item.value) === 100) {
            verified = true;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }
      expect(verified, 'total_cluster_power should be derived as 100 on devMain').to.be.true;
    });
  });

  describe('4. Dynamic Discovery via entity_relations Graph', function () {
    let templateId = null;
    let configId = null;
    let devParent = null;
    let devChild = null;
    let relationId = null;
    let fieldId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
      configId = await createTestConfig(templateId, ACCOUNT);
      devParent = await createConfiguredDevice(configId, ACCOUNT);
      devChild = await createConfiguredDevice('', ACCOUNT);

      // 建立 entity_relation: devParent -> devChild (Contains)
      const relResp = await apiClient.post(ENTITY_RELATIONS_PATH, {
        from_id: devParent.id,
        from_type: 'device',
        to_id: devChild.id,
        to_type: 'device',
        relation_type: 'Contains'
      }, ACCOUNT);
      expectSuccess(relResp);
      relationId = relResp.data.id;
      cleanups.push(async () => {
        if (relationId) await apiClient.delete(ENTITY_RELATIONS_PATH + '/' + relationId, {}, ACCOUNT);
      });

      // 子设备上报数值遥测
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        devChild.id,
        { battery_level: 88.0 },
        ACCOUNT
      );

      // 在父设备模板上配置基于 Contains 关系的计算字段 (avg)
      const createResp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'dynamic-child-battery',
        device_template_id: templateId,
        output_key: 'avg_child_battery',
        type: 'related_agg',
        config: {
          source_key: 'battery_level',
          func: 'avg',
          relation_type: 'Contains',
          use_relation: true,
          direction: 'to'
        },
        enabled: true
      }, ACCOUNT);
      expectSuccess(createResp);
      fieldId = createResp.data.id;
    });

    after(async function () {
      if (fieldId) {
        try { await apiClient.delete(calcFieldPath(fieldId), {}, ACCOUNT); } catch (_) {}
      }
    });

    it('dynamically resolves related devices from entity_relations and computes telemetry', async function () {
      // 父设备触发上报
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        devParent.id,
        { ping: 1.0 },
        ACCOUNT
      );

      // 轮询父设备当前遥测，验证 avg_child_battery = 88.0
      let verified = false;
      for (let i = 0; i < 15; i++) {
        const curResp = await apiClient.get('/telemetry/datas/current/' + devParent.id, {}, ACCOUNT);
        if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
          const item = curResp.data.find(r => r.key === 'avg_child_battery');
          if (item && Number(item.value) === 88.0) {
            verified = true;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }
      expect(verified, 'avg_child_battery should be derived as 88 on devParent').to.be.true;
    });
  });

  describe('5. Gateway / Sub-Device Hierarchy Discovery (devices.parent_id)', function () {
    let templateId = null;
    let configId = null;
    let devGateway = null;
    let devSub = null;
    let fieldId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
      configId = await createTestConfig(templateId, ACCOUNT);
      devGateway = await createConfiguredDevice(configId, ACCOUNT);
      // devSub 以 devGateway.id 为 parent_id
      devSub = await createConfiguredDevice('', ACCOUNT, devGateway.id);

      // 子设备上报遥测
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        devSub.id,
        { sensor_temp: 36.5 },
        ACCOUNT
      );

      // 网关配置下行汇聚计算字段 (max)
      const createResp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'gateway-subdev-max-temp',
        device_template_id: templateId,
        output_key: 'max_subdev_temp',
        type: 'related_agg',
        config: {
          source_key: 'sensor_temp',
          func: 'max',
          use_relation: true,
          direction: 'down'
        },
        enabled: true
      }, ACCOUNT);
      expectSuccess(createResp);
      fieldId = createResp.data.id;
    });

    after(async function () {
      if (fieldId) {
        try { await apiClient.delete(calcFieldPath(fieldId), {}, ACCOUNT); } catch (_) {}
      }
    });

    it('aggregates sub-device telemetry to gateway via parent_id hierarchy', async function () {
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        devGateway.id,
        { gw_heartbeat: 1.0 },
        ACCOUNT
      );

      let verified = false;
      for (let i = 0; i < 15; i++) {
        const curResp = await apiClient.get('/telemetry/datas/current/' + devGateway.id, {}, ACCOUNT);
        if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
          const item = curResp.data.find(r => r.key === 'max_subdev_temp');
          if (item && Number(item.value) === 36.5) {
            verified = true;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }
      expect(verified, 'max_subdev_temp should be derived as 36.5 on devGateway').to.be.true;
    });
  });

  describe('6. Telemetry Propagation (propagation) Across Entities', function () {
    let templateId = null;
    let configId = null;
    let devSource = null;
    let devTarget = null;
    let relationId = null;
    let fieldId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
      configId = await createTestConfig(templateId, ACCOUNT);
      devSource = await createConfiguredDevice(configId, ACCOUNT);
      devTarget = await createConfiguredDevice('', ACCOUNT);

      // 建立 entity_relation: devSource -> devTarget (Manages)
      const relResp = await apiClient.post(ENTITY_RELATIONS_PATH, {
        from_id: devSource.id,
        from_type: 'device',
        to_id: devTarget.id,
        to_type: 'device',
        relation_type: 'Manages'
      }, ACCOUNT);
      expectSuccess(relResp);
      relationId = relResp.data.id;
      cleanups.push(async () => {
        if (relationId) await apiClient.delete(ENTITY_RELATIONS_PATH + '/' + relationId, {}, ACCOUNT);
      });

      // 源设备配置 propagation 计算字段：上报 target_mode 时自动传播到目标设备
      const createResp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'propagate-mode-setting',
        device_template_id: templateId,
        output_key: 'target_mode',
        type: 'propagation',
        config: {
          source_key: 'input_mode',
          relation_type: 'Manages',
          use_relation: true,
          direction: 'to'
        },
        enabled: true
      }, ACCOUNT);
      expectSuccess(createResp);
      fieldId = createResp.data.id;
    });

    after(async function () {
      if (fieldId) {
        try { await apiClient.delete(calcFieldPath(fieldId), {}, ACCOUNT); } catch (_) {}
      }
    });

    it('propagates telemetry from source device to related target device', async function () {
      // 源设备上报 input_mode = 3.0
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        devSource.id,
        { input_mode: 3.0 },
        ACCOUNT
      );

      // 验证目标设备 devTarget 接收到派生遥测 target_mode = 3.0
      let verified = false;
      for (let i = 0; i < 15; i++) {
        const curResp = await apiClient.get('/telemetry/datas/current/' + devTarget.id, {}, ACCOUNT);
        if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
          const item = curResp.data.find(r => r.key === 'target_mode');
          if (item && Number(item.value) === 3.0) {
            verified = true;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }
      expect(verified, 'target_mode should be propagated to devTarget with value 3').to.be.true;
    });
  });

  describe('7. Multi-Tenant Isolation', function () {
    let templateIdA = null;
    let fieldIdA = null;

    before(async function () {
      templateIdA = await createTestTemplate(ACCOUNT);
      const createResp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'tenant-a-private-field',
        device_template_id: templateIdA,
        output_key: 'priv_val',
        type: 'related_agg',
        config: {
          source_key: 'raw',
          func: 'sum',
          device_ids: ['some-dummy-id']
        },
        enabled: true
      }, ACCOUNT);
      expectSuccess(createResp);
      fieldIdA = createResp.data.id;
    });

    after(async function () {
      if (fieldIdA) {
        try { await apiClient.delete(calcFieldPath(fieldIdA), {}, ACCOUNT); } catch (_) {}
      }
    });

    it('prevents tenant B from reading or manipulating tenant A calculated field', async function () {
      const getResp = await apiClient.get(calcFieldPath(fieldIdA), {}, OTHER_ACCOUNT);
      expectBusinessError(getResp, 100404);

      const updateResp = await apiClient.put(calcFieldPath(fieldIdA), {
        name: 'hacked',
        device_template_id: templateIdA,
        output_key: 'hacked_key',
        type: 'related_agg',
        config: { source_key: 'raw', func: 'sum', device_ids: ['d1'] }
      }, OTHER_ACCOUNT);
      expectBusinessError(updateResp, 100404);

      const delResp = await apiClient.delete(calcFieldPath(fieldIdA), {}, OTHER_ACCOUNT);
      expectBusinessError(delResp, 100404);
    });
  });
});
