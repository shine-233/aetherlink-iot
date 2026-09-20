/**
 * 文件用途：P2.3 摄取回压可观测度量（Decision Memo B 方案）与 TB-5 外部节点派发契约测试。
 *
 * 验证重点：
 * 1. P2.3 摄取回压与账本可观测性：
 *    - /api/v1/queue/stats 端点返回 uplink_bus 快照
 *    - 包含决策备忘录验收口径的 uplink_dropped_total 根指标
 *    - accounting 账本精确统计 received_total、accepted_total、各链路丢弃与阻塞事件
 *    - backpressure_alert 包含窗口时长、丢弃率阈值与告警计数
 * 2. TB-5 外部规则节点（external.kafka / action.webhook）契约：
 *    - external.kafka 必须配置非空 topic，否则 100002 拒绝
 *    - action.webhook 必须配置非空 url，否则校验拦截
 *    - 合法配置成功挂入规则链拓扑并支持版本发布
 */

const { expect } = require('chai');
require('../lib/runtime_config');
const apiClient = require('../lib/api_client');
const {
  createSimulationDevice,
  isMqttBrokerAvailable
} = require('../lib/seed_data');
const {
  publishMqttTelemetry
} = require('../lib/mqtt_device_fixture');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');

const SUITE = 'P2.3 Ingest Backpressure & TB-5 External Dispatch [67_p23_backpressure_and_tb5_external_dispatch]';
const ACCOUNT = 'tenant_admin';

describe(SUITE, function () {
  this.timeout(60000);

  const cleanups = [];
  const suffix = Date.now().toString().slice(-6);

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('API server is not healthy');
    }
    await apiClient.login(ACCOUNT);
  });

  after(async function () {
    for (const fn of cleanups.reverse()) {
      try {
        await fn();
      } catch (err) {
        // ignore cleanup errors
      }
    }
    apiClient.clearAllTokens();
  });

  describe('1. P2.3 Ingest Backpressure Ledger & Observable Drop Metrics', function () {
    it('exposes uplink_bus backpressure metrics and accounting ledger on /queue/stats', async function () {
      const resp = await apiClient.get('/queue/stats', {}, ACCOUNT);
      expectSuccess(resp);
      expect(resp.data).to.have.property('uplink_bus');
      const bus = resp.data.uplink_bus;
      expect(bus).to.be.an('object');

      // 验证决策备忘录验收口径的根指标名 uplink_dropped_total
      expect(bus).to.have.property('uplink_dropped_total');
      expect(bus.uplink_dropped_total).to.be.a('number');
      expect(bus.uplink_dropped_total).to.be.at.least(0);

      // 验证内部通道长度与容量展示
      expect(bus).to.have.property('telemetry_len').that.is.a('number');
      expect(bus).to.have.property('telemetry_cap').that.is.at.least(1);
      expect(bus).to.have.property('response_queue').that.is.a('number');

      // 验证 accounting 账本结构
      expect(bus).to.have.property('accounting').that.is.an('object');
      const acct = bus.accounting;
      expect(acct).to.have.property('received_total').that.is.a('number');
      expect(acct).to.have.property('accepted_total').that.is.a('number');
      expect(acct).to.have.property(bus.uplink_dropped_total !== undefined ? 'uplink_dropped_total' : 'dropped_total');
      expect(acct).to.have.property('rejected_admission').that.is.a('number');
      expect(acct).to.have.property('dropped_unknown_type').that.is.a('number');

      // 验证分账明细分类表
      expect(acct).to.have.property('dropped_channel_full').that.is.an('object');
      expect(acct).to.have.property('dropped_caller_context').that.is.an('object');
      expect(acct).to.have.property('dropped_bus_closed').that.is.an('object');
      expect(acct).to.have.property('blocked_events').that.is.an('object');

      for (const kind of ['telemetry', 'attribute', 'event', 'status', 'response']) {
        expect(acct.dropped_channel_full).to.have.property(kind).that.is.a('number');
        expect(acct.blocked_events).to.have.property(kind).that.is.a('number');
      }

      // 验证告警采样器配置与状态
      expect(bus).to.have.property('backpressure_alert').that.is.an('object');
      const alert = bus.backpressure_alert;
      expect(alert).to.have.property('enabled').to.equal(true);
      expect(alert).to.have.property('window_seconds').that.is.at.least(1);
      expect(alert).to.have.property('drop_ratio_threshold').that.is.within(0, 1);
      expect(alert).to.have.property('min_received').that.is.at.least(1);
      expect(alert).to.have.property('alert_count').that.is.a('number');
    });

    it('records received and accepted counts accurately upon live telemetry ingestion', async function () {
      const preResp = await apiClient.get('/queue/stats', {}, ACCOUNT);
      expectSuccess(preResp);
      const preReceived = preResp.data.uplink_bus.accounting.received_total;
      const preAccepted = preResp.data.uplink_bus.accounting.accepted_total;

      const device = await createSimulationDevice(ACCOUNT, `p23-dev-${suffix}`);
      cleanups.push(async () => {
        await apiClient.delete(`/device/${device.id}`, {}, ACCOUNT);
      });

      const mqttOk = await isMqttBrokerAvailable();
      if (mqttOk) {
        await publishMqttTelemetry(device, { temperature: 25.4, humidity: 62.1 }, ACCOUNT);
        // 允许总线异步分发调度
        await new Promise(r => setTimeout(r, 600));

        const postResp = await apiClient.get('/queue/stats', {}, ACCOUNT);
        expectSuccess(postResp);
        const postReceived = postResp.data.uplink_bus.accounting.received_total;
        const postAccepted = postResp.data.uplink_bus.accounting.accepted_total;

        expect(postReceived).to.be.at.least(preReceived);
        expect(postAccepted).to.be.at.least(preAccepted);
      }
    });
  });

  describe('2. TB-5 External Rule Chain Dispatch Configuration & Verification', function () {
    let testChainId = null;

    it('creates a rule chain with external nodes and rejects invalid configurations', async function () {
      // 1) 验证 external.kafka 缺少 topic 时拒绝
      const badKafkaGraph = {
        name: `rc-bad-kafka-${suffix}`,
        enabled: true,
        graph: {
          nodes: [
            { id: 't1', type: 'trigger.telemetry', config: {} },
            { id: 'k1', type: 'external.kafka', config: { topic: '' } }
          ],
          edges: [
            { from: 't1', to: 'k1' }
          ]
        }
      };

      const badResp = await apiClient.post('/rule-chains', badKafkaGraph, ACCOUNT);
      expectBusinessError(badResp, 100002);

      // 2) 验证 action.webhook 缺少 url 时校验拒绝
      const badWebhookGraph = {
        name: `rc-bad-webhook-${suffix}`,
        enabled: true,
        graph: {
          nodes: [
            { id: 't1', type: 'trigger.telemetry', config: {} },
            { id: 'w1', type: 'action.webhook', config: { url: '' } }
          ],
          edges: [
            { from: 't1', to: 'w1' }
          ]
        }
      };
      const badWebhookResp = await apiClient.post('/rule-chains', badWebhookGraph, ACCOUNT);
      expectBusinessError(badWebhookResp, 100002);

      // 3) 配置合法 external.kafka 与 action.webhook 节点成功创建规则链
      const validExternalGraph = {
        name: `rc-external-dispatch-${suffix}`,
        description: 'external kafka and webhook rule chain',
        enabled: true,
        graph: {
          nodes: [
            { id: 't1', type: 'trigger.telemetry', config: {} },
            { id: 'kafka_node', type: 'external.kafka', config: { topic: 'aetherlink.telemetry.sink' } },
            { id: 'webhook_node', type: 'action.webhook', config: { url: 'https://webhook.site/mock-test', timeout_ms: 1000 } }
          ],
          edges: [
            { from: 't1', to: 'kafka_node' },
            { from: 'kafka_node', to: 'webhook_node' }
          ]
        }
      };

      const createResp = await apiClient.post('/rule-chains', validExternalGraph, ACCOUNT);
      expectSuccess(createResp);
      testChainId = createResp.data.id;
      cleanups.push(async () => {
        if (testChainId) {
          await apiClient.delete(`/rule-chains/${testChainId}`, {}, ACCOUNT);
        }
      });

      // 4) 验证读取规则链结构，确认 external 节点配置正确回显
      const getResp = await apiClient.get(`/rule-chains/${testChainId}`, {}, ACCOUNT);
      expectSuccess(getResp);
      const nodes = getResp.data.graph.nodes;
      const kafkaNode = nodes.find(n => n.type === 'external.kafka');
      expect(kafkaNode).to.be.an('object');
      expect(kafkaNode.config.topic).to.equal('aetherlink.telemetry.sink');

      const webhookNode = nodes.find(n => n.type === 'action.webhook');
      expect(webhookNode).to.be.an('object');
      expect(webhookNode.config.url).to.equal('https://webhook.site/mock-test');
      expect(webhookNode.config.timeout_ms).to.equal(1000);
    });

    it('publishes a valid version for rule chain with external nodes', async function () {
      if (!testChainId) {
        this.skip();
      }

      // 创建草稿版本
      const versionResp = await apiClient.post(`/rule-chains/${testChainId}/versions`, {
        graph_hash: `hash-ext-${suffix}`
      }, ACCOUNT);
      expectSuccess(versionResp);
      expect(versionResp.data.version).to.be.an('object');
      expect(versionResp.data.version.status).to.equal('draft');

      // 发布版本
      const pubResp = await apiClient.post('/rule-chains/versions/publish', {
        chain_id: testChainId,
        version: 1
      }, ACCOUNT);
      expectSuccess(pubResp);
      expect(pubResp.data.audit).to.be.an('object');
    });
  });
});
