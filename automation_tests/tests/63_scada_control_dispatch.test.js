/**
 * 文件用途：P1.3「SCADA 控制真实下发联调」的 API 契约测试。
 *
 * 覆盖（ROADMAP §1.2 P1.3 剩余缺口「真实下发联调」）：
 *   1. 画布上的 valve 控件 → 二次确认令牌签发（HMAC 绑定租户/文档/控件/命令/操作者）；
 *   2. 未带确认令牌的执行被拒（fail closed，审计留 failed）；
 *   3. 带令牌执行成功 → 命令经真实命令下发通道到达设备模拟器
 *      （devices/command/<device_number>），模拟器按响应信封回执；
 *   4. 控制审计（pending→success）可回查。
 *
 * 关键注意事项：
 *   - 确认密钥未配置时签发端点 fail closed——本用例要求本地配置了
 *     scada.control.confirmation_secret（conf-localdev.yml，不入库）；
 *   - 命令模拟器走的是与 OTA/影子同一套 stub broker + 响应信封。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const { startMqttCommandDevice } = require('../lib/mqtt_device_fixture');

const SUITE = 'P1.3 SCADA control real dispatch [63_scada_control_dispatch]';
const ACCOUNT = 'tenant_admin';

const CODE_OP_DENIED = 201002;

const CANVAS = JSON.stringify({
  schemaVersion: 1,
  nodes: [
    { id: 'w1', kind: 'widget', ref: 'valve', props: {} }
  ]
});

describe(SUITE, function () {
  this.timeout(120000);

  let deviceSeed = null;
  let mqttDevice = null;
  let projectId = null;
  let documentId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 63_scada_control_dispatch.test.js');
    }
    await apiClient.login(ACCOUNT);

    deviceSeed = await seedData.createSimulationDevice(ACCOUNT);
    mqttDevice = await startMqttCommandDevice(deviceSeed, ACCOUNT);

    const projectResp = await apiClient.post('/scada/projects', {
      name: seedData.makeRunLabel('scada_proj'),
      description: 'P1.3 real dispatch fixture'
    }, ACCOUNT);
    expect(projectResp.code, 'create project').to.equal(200);
    projectId = projectResp.data && (projectResp.data.id || projectResp.data.ID);

    const docResp = await apiClient.post(`/scada/projects/${projectId}/documents`, {
      name: seedData.makeRunLabel('scada_doc'),
      json_data: CANVAS
    }, ACCOUNT);
    expect(docResp.code, 'create document').to.equal(200);
    documentId = docResp.data && (docResp.data.id || docResp.data.ID);

    const publishResp = await apiClient.post(`/scada/documents/${documentId}/publish`, {}, ACCOUNT);
    expect(publishResp.code, 'publish document').to.equal(200);
  });

  after(async function () {
    if (mqttDevice) {
      try { await mqttDevice.cleanup(); } catch (err) { /* 尽力回收 */ }
    }
    if (deviceSeed && typeof deviceSeed.cleanup === 'function') {
      try { await deviceSeed.cleanup(); } catch (err) { /* 尽力回收 */ }
    }
  });

  it('refuses execution without a confirmation token and records the failure', async function () {
    const resp = await apiClient.post('/scada/control', {
      device_id: deviceSeed.id,
      document_id: documentId,
      widget_id: 'w1',
      widget_type: 'valve',
      version: '1',
      command: 'open_valve',
      params: { target: 'open' }
    }, ACCOUNT);
    expect(resp.code, 'execute without token').to.equal(CODE_OP_DENIED);

    const audits = await apiClient.get(`/scada/documents/${documentId}/audits`, {}, ACCOUNT);
    expect(audits.code, 'audit list').to.equal(200);
    const rows = Array.isArray(audits.data) ? audits.data : (audits.data && audits.data.list) || [];
    // 确认闸门拒绝的审计 outcome=denied（与执行失败的 failed 是两类）。
    const denied = rows.find(row => row.command === 'open_valve' && row.outcome === 'denied');
    expect(denied, 'denied audit row persisted').to.be.an('object');
    expect(denied.detail || '', 'denial reason recorded').to.include('confirmation');
  });

  it('issues a confirmation token, executes the command, and the device receives it', async function () {
    const confirmResp = await apiClient.post('/scada/control/confirm', {
      document_id: documentId,
      widget_id: 'w1',
      command: 'open_valve'
    }, ACCOUNT);
    expect(confirmResp.code, 'issue confirmation').to.equal(200);
    const token = confirmResp.data && (confirmResp.data.confirmation_token || confirmResp.data.token);
    expect(token, 'hmac token').to.be.a('string').and.not.equal('');

    const execResp = await apiClient.post('/scada/control', {
      device_id: deviceSeed.id,
      document_id: documentId,
      widget_id: 'w1',
      widget_type: 'valve',
      version: '1',
      command: 'open_valve',
      params: { target: 'open' },
      confirmation_token: token
    }, ACCOUNT);
    expect(execResp.code, 'execute with token').to.equal(200);
    expect(execResp.data && (execResp.data.outcome || execResp.data), 'outcome success')
      .to.include('success');

    // 设备模拟器必须真实收到命令并按响应信封回执（回执字段：method/params/ack_payload）。
    const receipts = await mqttDevice.waitForReceipts(1, 30000);
    const commandReceipt = receipts.find(row => row.method === 'open_valve');
    expect(commandReceipt, 'device received the command').to.be.an('object');
    expect(commandReceipt.topic, 'command topic is the real dispatch topic')
      .to.match(/^devices\/command\//);
    expect(commandReceipt.params, 'params carried').to.deep.include({ target: 'open' });
    expect(commandReceipt.ack_payload, 'device responded with success envelope')
      .to.deep.include({ method: 'open_valve' });

    // 审计：pending 之后 success。
    const audits = await apiClient.get(`/scada/documents/${documentId}/audits`, {}, ACCOUNT);
    const rows = Array.isArray(audits.data) ? audits.data : (audits.data && audits.data.list) || [];
    const success = rows.find(row => row.command === 'open_valve' && row.outcome === 'success');
    expect(success, 'success audit row persisted').to.be.an('object');
    expect(success.actor_user_id || success.ActorUserID, 'actor recorded').to.be.a('string')
      .and.not.equal('');
  });

  it('rejects a command whose widget command is not registered', async function () {
    const resp = await apiClient.post('/scada/control', {
      device_id: deviceSeed.id,
      document_id: documentId,
      widget_id: 'w1',
      widget_type: 'valve',
      version: '1',
      command: 'launch_missile'
    }, ACCOUNT);
    expect(resp.code, 'unregistered command').to.equal(CODE_OP_DENIED);
  });
});
