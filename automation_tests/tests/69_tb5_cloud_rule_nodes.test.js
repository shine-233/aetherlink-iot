/**
 * 文件用途：TB-5 云连接规则节点（AWS SQS, AWS SNS, Azure IoT Hub）API 契约与生命周期测试。
 *
 * 覆盖：
 *   1. external.aws_sqs 节点配置校验（缺少 queue_url 或 region 均被拒绝）；
 *   2. external.aws_sns 节点配置校验（缺少 topic_arn 或 region 均被拒绝）；
 *   3. external.azure_iot_hub 节点配置校验（缺少 hub_name 被拒绝）；
 *   4. 云连接规则链创建——三类云节点与触发器串联，支持 ${secret.KEY} 动态凭证引用；
 *   5. 规则链图读取回验与节点规范一致性；
 *   6. 版本快照发布（POST /rule-chains/versions/publish）与不可变历史回查。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'TB-5 cloud rule nodes (AWS SQS, AWS SNS, Azure IoT Hub) [69_tb5_cloud_rule_nodes]';
const ACCOUNT = 'tenant_admin';

const CODE_PARAM_ERROR = 100002;

describe(SUITE, function () {
  this.timeout(120000);

  const cleanups = [];
  const suffix = Date.now();
  let testChainId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 69_tb5_cloud_rule_nodes.test.js');
    }
    await apiClient.login(ACCOUNT);
  });

  after(async function () {
    for (const cleanup of cleanups) {
      try {
        await cleanup();
      } catch (_) {}
    }
  });

  it('rejects external.aws_sqs node when queue_url or region is missing', async function () {
    // 1) 缺少 queue_url
    const badSQS1 = {
      name: `rc-bad-sqs1-${suffix}`,
      enabled: true,
      graph: {
        nodes: [
          { id: 't1', type: 'trigger.telemetry', config: {} },
          { id: 'sqs1', type: 'external.aws_sqs', config: { region: 'us-east-1' } }
        ],
        edges: [{ from: 't1', to: 'sqs1' }]
      }
    };
    const resp1 = await apiClient.post('/rule-chains', badSQS1, ACCOUNT);
    expect(resp1.code, 'missing queue_url rejected').to.equal(CODE_PARAM_ERROR);

    // 2) 缺少 region
    const badSQS2 = {
      name: `rc-bad-sqs2-${suffix}`,
      enabled: true,
      graph: {
        nodes: [
          { id: 't1', type: 'trigger.telemetry', config: {} },
          { id: 'sqs2', type: 'external.aws_sqs', config: { queue_url: 'https://sqs.us-east-1.amazonaws.com/123/q' } }
        ],
        edges: [{ from: 't1', to: 'sqs2' }]
      }
    };
    const resp2 = await apiClient.post('/rule-chains', badSQS2, ACCOUNT);
    expect(resp2.code, 'missing region rejected').to.equal(CODE_PARAM_ERROR);
  });

  it('rejects external.aws_sns node when topic_arn or region is missing', async function () {
    // 1) 缺少 topic_arn
    const badSNS1 = {
      name: `rc-bad-sns1-${suffix}`,
      enabled: true,
      graph: {
        nodes: [
          { id: 't1', type: 'trigger.telemetry', config: {} },
          { id: 'sns1', type: 'external.aws_sns', config: { region: 'eu-west-1' } }
        ],
        edges: [{ from: 't1', to: 'sns1' }]
      }
    };
    const resp1 = await apiClient.post('/rule-chains', badSNS1, ACCOUNT);
    expect(resp1.code, 'missing topic_arn rejected').to.equal(CODE_PARAM_ERROR);

    // 2) 缺少 region
    const badSNS2 = {
      name: `rc-bad-sns2-${suffix}`,
      enabled: true,
      graph: {
        nodes: [
          { id: 't1', type: 'trigger.telemetry', config: {} },
          { id: 'sns2', type: 'external.aws_sns', config: { topic_arn: 'arn:aws:sns:eu-west-1:123:my-topic' } }
        ],
        edges: [{ from: 't1', to: 'sns2' }]
      }
    };
    const resp2 = await apiClient.post('/rule-chains', badSNS2, ACCOUNT);
    expect(resp2.code, 'missing region rejected').to.equal(CODE_PARAM_ERROR);
  });

  it('rejects external.azure_iot_hub node when hub_name is missing', async function () {
    const badAzure = {
      name: `rc-bad-azure-${suffix}`,
      enabled: true,
      graph: {
        nodes: [
          { id: 't1', type: 'trigger.telemetry', config: {} },
          { id: 'az1', type: 'external.azure_iot_hub', config: { device_id: 'dev-1' } }
        ],
        edges: [{ from: 't1', to: 'az1' }]
      }
    };
    const resp = await apiClient.post('/rule-chains', badAzure, ACCOUNT);
    expect(resp.code, 'missing hub_name rejected').to.equal(CODE_PARAM_ERROR);
  });

  it('creates a rule chain with valid AWS SQS, AWS SNS, and Azure IoT Hub nodes', async function () {
    const validCloudGraph = {
      name: `rc-cloud-dispatch-${suffix}`,
      description: 'full cloud connector rule chain',
      enabled: true,
      graph: {
        nodes: [
          { id: 't1', type: 'trigger.telemetry', config: {} },
          {
            id: 'sqs_node',
            type: 'external.aws_sqs',
            config: {
              queue_url: 'https://sqs.us-east-1.amazonaws.com/123456789012/telemetry-queue',
              region: 'us-east-1',
              secret_key: '${secret.AWS_SQS_KEY}'
            }
          },
          {
            id: 'sns_node',
            type: 'external.aws_sns',
            config: {
              topic_arn: 'arn:aws:sns:us-east-1:123456789012:industrial-alerts',
              region: 'us-east-1'
            }
          },
          {
            id: 'azure_node',
            type: 'external.azure_iot_hub',
            config: {
              hub_name: 'aetherlink-production-hub',
              device_id: 'edge-gateway-01'
            }
          }
        ],
        edges: [
          { from: 't1', to: 'sqs_node' },
          { from: 'sqs_node', to: 'sns_node' },
          { from: 'sns_node', to: 'azure_node' }
        ]
      }
    };

    const createResp = await apiClient.post('/rule-chains', validCloudGraph, ACCOUNT);
    expect(createResp.code, 'create cloud rule chain').to.equal(200);
    expect(createResp.data).to.have.property('id').that.is.a('string');
    testChainId = createResp.data.id;
    cleanups.push(async () => {
      await apiClient.delete(`/rule-chains/${testChainId}`, {}, ACCOUNT);
    });

    // 读回详情验证 graph 包含三个云节点
    const readResp = await apiClient.get(`/rule-chains/${testChainId}`, {}, ACCOUNT);
    expect(readResp.code, 'read rule chain').to.equal(200);
    const nodes = readResp.data.graph.nodes;
    expect(nodes).to.have.lengthOf(4);

    const nodeTypes = nodes.map(n => n.type);
    expect(nodeTypes).to.include('external.aws_sqs');
    expect(nodeTypes).to.include('external.aws_sns');
    expect(nodeTypes).to.include('external.azure_iot_hub');
  });

  it('publishes a version snapshot for the cloud rule chain and verifies history', async function () {
    expect(testChainId, 'valid testChainId from previous step').to.be.a('string');

    // 1. 创建草稿版本
    const draftResp = await apiClient.post(`/rule-chains/${testChainId}/versions`, {
      graph_hash: `hash-tb5-${suffix}`
    }, ACCOUNT);
    expect(draftResp.code, 'create draft version').to.equal(200);
    expect(draftResp.data.version).to.be.an('object');
    expect(draftResp.data.version.version).to.equal(1);
    expect(draftResp.data.version.status).to.equal('draft');

    // 2. 发布草稿版本 1
    const pubResp = await apiClient.post('/rule-chains/versions/publish', {
      chain_id: testChainId,
      version: 1
    }, ACCOUNT);
    expect(pubResp.code, 'publish version').to.equal(200);
    expect(pubResp.data.audit).to.be.an('object');

    // 3. 查询版本历史并校验不可变快照与状态
    const verListResp = await apiClient.get(`/rule-chains/${testChainId}/versions`, {}, ACCOUNT);
    expect(verListResp.code, 'get version list').to.equal(200);
    expect(verListResp.data).to.be.an('array').with.lengthOf(1);
    const ver = verListResp.data[0];
    expect(ver.version).to.equal(1);
    expect(ver.status).to.equal('published');
    expect(ver.chain_id).to.equal(testChainId);
  });
});
