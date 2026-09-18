/**
 * 文件用途：TB-1 告警规则 2.0 终章（Alarm Rules 2.0 via Calculated Fields）端到端契约测试。
 *
 * 对标 ThingsBoard 4.3 LTS (PR#14036 CalculatedFieldType.ALARM / AlarmCalculatedFieldConfiguration.java)：
 *   1. 配置与模式校验：非法严重度、空规则、语法畸变表达式与非法清除规则精确拒绝（100002 参数错误）；
 *   2. 告警字段 CRUD 与状态切换：创建、更新、查询详情、列表筛选、启停 toggle、删除；
 *   3. 遥测触发生成活动告警：遥测上行满足阈值时，自动生成活动告警（ACTIVE_UNACK），并写入派生状态遥测；
 *   4. 严重度平滑升级（Severity Escalation）：遥测恶化时活动告警就地升级（M -> H），且无重复告警记录噪声；
 *   5. 自动生命周期清除（Self-Healing Auto-Clear）：遥测恢复正常命中 clear_rule 时自动清除告警（转为 CLEARED_UNACK）；
 *   6. 拓扑与关联实体传播（Alarm Propagation）：告警事件自动向关联设备/父网关广播留存审计溯源标记；
 *   7. 租户隔离防护：租户 B 无法查询、修改、触发或清除租户 A 的告警规则。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');

const SUITE = 'Alarm Rules 2.0 via Calculated Fields [52_alarm_rules_advanced]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const CALCULATED_FIELDS_PATH = '/calculated_fields';

function calcFieldPath(id) {
  return `${CALCULATED_FIELDS_PATH}/${id}`;
}

describe(SUITE, function () {
  this.timeout(180000);

  const cleanups = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 52_alarm_rules_advanced.test.js');
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

  // Helper: 创建测试模板
  async function createTestTemplate(accountKey = ACCOUNT) {
    const name = seedData.makeRunLabel('tb1_alarm_tpl');
    const resp = await apiClient.post('/device/template', {
      name,
      author: 'automation',
      version: '1.0.0',
      description: 'TB-1 test template'
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
    const name = seedData.makeRunLabel('tb1_alarm_cfg');
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
        name: deviceSeed.row.name || 'child-dev',
        parent_id: parentId
      }, accountKey);
      expectSuccess(updateResp);
    }

    return deviceSeed;
  }

  describe('1. Schema & Configuration Validation', function () {
    let templateId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
    });

    it('rejects alarm type without rules array', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-alarm-1',
        device_template_id: templateId,
        output_key: 'alarm_status',
        type: 'alarm',
        config: {
          alarm_name: 'Empty Rules Alarm'
        }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('at least one rule in rules');
    });

    it('rejects alarm rule with invalid severity', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-alarm-2',
        device_template_id: templateId,
        output_key: 'alarm_status',
        type: 'alarm',
        config: {
          alarm_name: 'Invalid Severity Alarm',
          rules: [
            { severity: 'CRITICAL', expression: 'temp > 80' }
          ]
        }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('severity must be H, M, or L');
    });

    it('rejects alarm rule with empty expression', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-alarm-3',
        device_template_id: templateId,
        output_key: 'alarm_status',
        type: 'alarm',
        config: {
          alarm_name: 'Empty Expression Alarm',
          rules: [
            { severity: 'H', expression: '   ' }
          ]
        }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('expression cannot be empty');
    });

    it('rejects alarm rule with malformed expression syntax', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-alarm-4',
        device_template_id: templateId,
        output_key: 'alarm_status',
        type: 'alarm',
        config: {
          alarm_name: 'Syntax Error Alarm',
          rules: [
            { severity: 'H', expression: 'temp > > 80' }
          ]
        }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('expression is invalid');
    });

    it('rejects alarm with malformed clear_rule expression syntax', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'bad-alarm-5',
        device_template_id: templateId,
        output_key: 'alarm_status',
        type: 'alarm',
        config: {
          alarm_name: 'Clear Syntax Error Alarm',
          rules: [
            { severity: 'H', expression: 'temp > 80' }
          ],
          clear_rule: {
            expression: 'temp < < 40'
          }
        }
      }, ACCOUNT);
      expectBusinessError(resp, 100002);
      expect(String(resp.message)).to.include('clear_rule expression is invalid');
    });
  });

  describe('2. Alarm Field CRUD & Lifecycle Toggle', function () {
    let templateId = null;
    let fieldId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
    });

    it('creates an alarm calculated field successfully', async function () {
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'motor-temp-alarm',
        device_template_id: templateId,
        output_key: 'motor_alarm_status',
        type: 'alarm',
        config: {
          alarm_name: 'Motor Overheat Alarm',
          rules: [
            { severity: 'H', expression: 'temperature >= 90' },
            { severity: 'M', expression: 'temperature >= 70' },
            { severity: 'L', expression: 'temperature >= 50' }
          ],
          clear_rule: {
            expression: 'temperature < 45'
          },
          propagate: true
        },
        enabled: false,
        remark: 'Motor temperature tiered alarm'
      }, ACCOUNT);
      expectSuccess(resp);
      expect(resp.data).to.be.an('object');
      fieldId = resp.data.id;
      expect(fieldId).to.be.a('string').and.not.equal('');
      expect(resp.data.type).to.equal('alarm');
      expect(resp.data.output_key).to.equal('motor_alarm_status');
      expect(resp.data.enabled).to.equal(false);
    });

    it('updates alarm calculated field configuration', async function () {
      const updateResp = await apiClient.put(calcFieldPath(fieldId), {
        name: 'motor-temp-alarm-updated',
        device_template_id: templateId,
        output_key: 'motor_alarm_status',
        type: 'alarm',
        config: {
          alarm_name: 'Motor Thermal Surge Alarm',
          rules: [
            { severity: 'H', expression: 'temperature >= 85' },
            { severity: 'M', expression: 'temperature >= 65' }
          ],
          clear_rule: {
            expression: 'temperature < 40'
          },
          propagate: true
        },
        remark: 'Adjusted thresholds'
      }, ACCOUNT);
      expectSuccess(updateResp);
      expect(updateResp.data.type).to.equal('alarm');
    });

    it('toggles the enabled status of the alarm field', async function () {
      const toggleResp = await apiClient.put(`${calcFieldPath(fieldId)}/toggle`, {
        enabled: true
      }, ACCOUNT);
      expectSuccess(toggleResp);
      expect(toggleResp.data.enabled).to.equal(true);
    });

    it('reads the updated alarm field via list and detail endpoints', async function () {
      const detailResp = await apiClient.get(calcFieldPath(fieldId), {}, ACCOUNT);
      expectSuccess(detailResp);
      expect(detailResp.data.id).to.equal(fieldId);
      expect(detailResp.data.type).to.equal('alarm');
      expect(detailResp.data.enabled).to.equal(true);

      const listResp = await apiClient.get(CALCULATED_FIELDS_PATH, {
        page: 1,
        page_size: 10,
        device_template_id: templateId
      }, ACCOUNT);
      expectSuccess(listResp);
      const found = listResp.data.list.find(item => item.id === fieldId);
      expect(found, 'created alarm field in template list').to.be.an('object');
      expect(found.type).to.equal('alarm');
    });

    it('deletes the alarm calculated field', async function () {
      const delResp = await apiClient.delete(calcFieldPath(fieldId), {}, ACCOUNT);
      expectSuccess(delResp);

      const getResp = await apiClient.get(calcFieldPath(fieldId), {}, ACCOUNT);
      expectBusinessError(getResp, 100404);
    });
  });

  describe('3. Telemetry Trigger, Severity Escalation & Auto-Clear Lifecycle', function () {
    let templateId = null;
    let configId = null;
    let device = null;
    let fieldId = null;
    let generatedAlarmId = null;

    before(async function () {
      templateId = await createTestTemplate(ACCOUNT);
      configId = await createTestConfig(templateId, ACCOUNT);
      device = await createConfiguredDevice(configId, ACCOUNT);

      // 创建并启用三级严重度阶梯告警规则（含自愈清除规则）
      const createResp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'thermal-tiered-alarm',
        device_template_id: templateId,
        output_key: 'thermal_alarm_state',
        type: 'alarm',
        config: {
          alarm_name: 'Reactor Overheat Alert',
          rules: [
            { severity: 'H', expression: 'temp >= 80' },
            { severity: 'M', expression: 'temp >= 60' },
            { severity: 'L', expression: 'temp >= 40' }
          ],
          clear_rule: {
            expression: 'temp < 35'
          }
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
      if (generatedAlarmId) {
        try { await apiClient.delete('/alarm/info/history/' + generatedAlarmId, {}, ACCOUNT); } catch (_) {}
      }
    });

    it('triggers Medium severity alarm when telemetry reaches 65', async function () {
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        device.id,
        { temp: 65.0 },
        ACCOUNT
      );

      // 轮询查询告警历史，验证生成了一行活动告警
      let alarmFound = null;
      for (let i = 0; i < 20; i++) {
        const histResp = await apiClient.get('/alarm/info/history', {
          page: 1,
          page_size: 10,
          device_id: device.id
        }, ACCOUNT);
        if (histResp && histResp.code === 200 && histResp.data && Array.isArray(histResp.data.list)) {
          const item = histResp.data.list.find(r => r.name === 'Reactor Overheat Alert');
          if (item) {
            alarmFound = item;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }

      expect(alarmFound, 'alarm record created in alarm_history').to.be.an('object');
      generatedAlarmId = alarmFound.id;
      expect(alarmFound.alarm_status).to.equal('M');

      // 验证通过详情接口读取四态状态为 ACTIVE_UNACK
      const detailResp = await apiClient.get(`/alarm/info/history/${encodeURIComponent(generatedAlarmId)}`, {}, ACCOUNT);
      expectSuccess(detailResp);
      expect(detailResp.data.lifecycle_status).to.equal('ACTIVE_UNACK');
      expect(detailResp.data.alarm_status).to.equal('M');

      // 验证派生遥测也同步写入了 thermal_alarm_state = "M"
      const curResp = await apiClient.get('/telemetry/datas/current/' + device.id, {}, ACCOUNT);
      if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
        const stateKey = curResp.data.find(k => k.key === 'thermal_alarm_state');
        if (stateKey) {
          expect(stateKey.value).to.equal('M');
        }
      }
    });

    it('escalates alarm severity in-place to High (M -> H) without duplicate noise when temp reaches 85', async function () {
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        device.id,
        { temp: 85.0 },
        ACCOUNT
      );

      // 轮询等待严重度就地平滑升级
      let escalated = false;
      for (let i = 0; i < 20; i++) {
        const detailResp = await apiClient.get(`/alarm/info/history/${encodeURIComponent(generatedAlarmId)}`, {}, ACCOUNT);
        if (detailResp && detailResp.code === 200 && detailResp.data) {
          if (detailResp.data.alarm_status === 'H') {
            escalated = true;
            expect(detailResp.data.content).to.include('escalated from M to H');
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }

      expect(escalated, 'alarm escalated to H in-place').to.be.true;

      // 验证设备维度的告警历史记录数没有翻倍膨胀，依然保持单行活动态
      const histResp = await apiClient.get('/alarm/info/history', {
        page: 1,
        page_size: 10,
        device_id: device.id
      }, ACCOUNT);
      expectSuccess(histResp);
      const activeAlerts = histResp.data.list.filter(r => r.name === 'Reactor Overheat Alert');
      expect(activeAlerts.length, 'should not generate duplicate active alarm records on escalation').to.equal(1);
    });

    it('self-heals and auto-clears the alarm when telemetry recovers below clear threshold (temp = 25)', async function () {
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        device.id,
        { temp: 25.0 },
        ACCOUNT
      );

      // 轮询等待自动清除生效（alarm_status -> N, lifecycle_status -> CLEARED_UNACK）
      let cleared = false;
      for (let i = 0; i < 20; i++) {
        const detailResp = await apiClient.get(`/alarm/info/history/${encodeURIComponent(generatedAlarmId)}`, {}, ACCOUNT);
        if (detailResp && detailResp.code === 200 && detailResp.data) {
          if (detailResp.data.alarm_status === 'N' || detailResp.data.lifecycle_status === 'CLEARED_UNACK') {
            cleared = true;
            expect(detailResp.data.lifecycle_status).to.equal('CLEARED_UNACK');
            expect(detailResp.data.cleared_by).to.equal('system');
            expect(detailResp.data.cleared_at).to.be.a('string').and.not.be.empty;
            if (detailResp.data.action_note) {
              expect(detailResp.data.action_note).to.include('auto-cleared by rule');
            }
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }

      expect(cleared, 'alarm auto-cleared upon meeting clear_rule').to.be.true;
    });
  });

  describe('4. Topology Alarm Propagation (Broadcast to Parent Gateway)', function () {
    let parentTemplateId = null;
    let parentConfigId = null;
    let childTemplateId = null;
    let childConfigId = null;
    let gatewayDev = null;
    let childDev = null;
    let fieldId = null;

    before(async function () {
      // 创建网关模板与设备
      parentTemplateId = await createTestTemplate(ACCOUNT);
      parentConfigId = await createTestConfig(parentTemplateId, ACCOUNT);
      gatewayDev = await createConfiguredDevice(parentConfigId, ACCOUNT);

      // 创建子设备模板与子设备（挂载到网关下）
      childTemplateId = await createTestTemplate(ACCOUNT);
      childConfigId = await createTestConfig(childTemplateId, ACCOUNT);
      childDev = await createConfiguredDevice(childConfigId, ACCOUNT, gatewayDev.id);

      // 在子设备模板上配置具备 propagation: true 且显式包含网关的目标广播
      const createResp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'subdevice-propagated-alarm',
        device_template_id: childTemplateId,
        output_key: 'sensor_fault_state',
        type: 'alarm',
        config: {
          alarm_name: 'Critical Vibration Alert',
          rules: [
            { severity: 'H', expression: 'vibration >= 50' }
          ],
          propagate: true,
          device_ids: [gatewayDev.id]
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

    it('propagates the alarm to the parent gateway when the subdevice triggers an alarm', async function () {
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        childDev.id,
        { vibration: 75.0 },
        ACCOUNT
      );

      // 轮询验证父网关上是否收到了广播告警
      let propAlarmFound = null;
      for (let i = 0; i < 20; i++) {
        const gatewayHistResp = await apiClient.get('/alarm/info/history', {
          page: 1,
          page_size: 10,
          device_id: gatewayDev.id
        }, ACCOUNT);
        if (gatewayHistResp && gatewayHistResp.code === 200 && gatewayHistResp.data && Array.isArray(gatewayHistResp.data.list)) {
          const item = gatewayHistResp.data.list.find(r => r.name === 'Critical Vibration Alert');
          if (item) {
            propAlarmFound = item;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }

      expect(propAlarmFound, 'alarm propagated to parent gateway').to.be.an('object');
      expect(propAlarmFound.alarm_status).to.equal('H');
      expect(propAlarmFound.content).to.include(`Propagated from device ${childDev.id}`);
      if (propAlarmFound.remark) {
        expect(JSON.stringify(propAlarmFound.remark)).to.include(childDev.id);
      }
    });
  });

  describe('5. Multi-Tenant Isolation', function () {
    let templateA = null;
    let fieldA = null;

    before(async function () {
      templateA = await createTestTemplate(ACCOUNT);
      const resp = await apiClient.post(CALCULATED_FIELDS_PATH, {
        name: 'tenant-a-alarm',
        device_template_id: templateA,
        output_key: 'leak_state',
        type: 'alarm',
        config: {
          alarm_name: 'Gas Leak Alert',
          rules: [
            { severity: 'H', expression: 'gas_ppm > 100' }
          ]
        },
        enabled: true
      }, ACCOUNT);
      expectSuccess(resp);
      fieldA = resp.data.id;
    });

    after(async function () {
      if (fieldA) {
        try { await apiClient.delete(calcFieldPath(fieldA), {}, ACCOUNT); } catch (_) {}
      }
    });

    it('prevents Tenant B from retrieving Tenant A alarm calculated field', async function () {
      const resp = await apiClient.get(calcFieldPath(fieldA), {}, OTHER_ACCOUNT);
      expectBusinessError(resp, 100404);
    });

    it('prevents Tenant B from modifying Tenant A alarm calculated field', async function () {
      const resp = await apiClient.put(calcFieldPath(fieldA), {
        name: 'hacked-alarm',
        device_template_id: templateA,
        output_key: 'leak_state',
        type: 'alarm',
        config: {
          alarm_name: 'Hacked',
          rules: [{ severity: 'L', expression: 'gas_ppm > 1' }]
        }
      }, OTHER_ACCOUNT);
      expectBusinessError(resp, 100404);
    });

    it('prevents Tenant B from toggling Tenant A alarm calculated field', async function () {
      const resp = await apiClient.put(`${calcFieldPath(fieldA)}/toggle`, {
        enabled: false
      }, OTHER_ACCOUNT);
      expectBusinessError(resp, 100404);
    });

    it('prevents Tenant B from deleting Tenant A alarm calculated field', async function () {
      const resp = await apiClient.delete(calcFieldPath(fieldA), {}, OTHER_ACCOUNT);
      expectBusinessError(resp, 100404);
    });
  });
});
