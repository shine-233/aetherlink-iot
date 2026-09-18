/**
 * 文件用途：P1.2 规则链可靠性（Rule Chain Reliability）自动化契约测试。
 *
 * 核心验证矩阵：
 *  1. 版本生命周期与不可变审计（Gate 4: 发布版本可回滚）：
 *     - 创建草稿版本 (v1) 并校验返回状态为 draft
 *     - 相同图哈希幂等去重（不重复产生空版本）
 *     - 发布 v1 (draft -> published)
 *     - 重复发布已被发布的版本坚决拒绝 (fail closed)
 *     - 创建新草稿 v2
 *     - 基于 published 版本执行回滚，产生新草稿 v3（rolled_back_from=1，原发布历史保持不可变）
 *     - 版本列表完整查询与租户隔离
 *  2. 死信队列（DLQ Sink）持久化（Gate 1: 失败节点进入 DLQ）：
 *     - 配置节点独立执行策略：timeout_ms=100, max_attempts=2, backoff_ms=50, dead_letter=true, retry_safe=true
 *     - 触发节点终局失败，自动下沉 rule_chain_dead_letters
 *     - 回读验证 attempts=2、error 摘要、chain_id、node_id、node_type 及 device_id
 *  3. 单消息 Trace 串联（Gate 3: 同一消息 Trace 可串联 & Gate 2: 延迟可观测）：
 *     - 按 exec_id 查询完整链路 trace
 *     - 验证单消息在 DAG 触发器节点与动作节点间的按时序执行链路串联
 *     - 验证 elapsed_ms 耗时与 pass/error_msg 可观测
 *  4. 输入回放执行面与不可逆副作用拦截（Gate 5: 回放不重复生产副作用）：
 *     - 回读记录的输入快照（replay records）
 *     - confirm_side_effects=false 时，命中有副作用节点（action.webhook）被闸门拦截拒绝
 *     - confirm_side_effects=true 时，放行执行并完成回放
 *  5. 严格多租户拓扑隔离：
 *     - 跨租户无法读取死信、无法读取 Trace 链路、无法读取回放快照、无法回放他租户执行
 */

const { expect } = require('chai');
require('../lib/runtime_config');
const apiClient = require('../lib/api_client');
const {
  createSimulationDevice,
  isMqttBrokerAvailable
} = require('../lib/seed_data');
const {
  publishMqttTelemetry,
  publishMqttRawTelemetry
} = require('../lib/mqtt_device_fixture');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');

const SUITE = 'P1.2 Rule Chain Reliability [59_rule_chain_reliability]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const NOT_FOUND_CODE = 100404;
const PARAM_ERROR_CODE = 100002;
const OP_DENIED_CODE = 100003;

describe(SUITE, function () {
  this.timeout(90000);

  const cleanups = [];
  const suffix = Date.now().toString().slice(-6);

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 59_rule_chain_reliability.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);
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

  describe('1. 版本生命周期与回滚审计 (Gate 4)', function () {
    let chainId = '';
    const chainName = `rc-ver-${suffix}`;

    it('创建规则链并建立首个草稿版本 v1', async function () {
      const graph = {
        nodes: [
          { id: 't1', type: 'trigger.telemetry', config: {} },
          { id: 'f1', type: 'filter.threshold', config: { key: 'temp', op: '>', value: 20 } }
        ],
        edges: [{ from: 't1', to: 'f1' }]
      };
      const createResp = await apiClient.post('/rule-chains', {
        name: chainName,
        description: 'version lifecycle test chain',
        enabled: true,
        graph
      }, ACCOUNT);
      expectSuccess(createResp);
      expect(createResp.data.id).to.be.a('string').and.not.be.empty;
      chainId = createResp.data.id;
      cleanups.push(async () => {
        await apiClient.delete(`/rule-chains/${chainId}`, {}, ACCOUNT);
      });

      // 创建草稿版本 v1
      const v1Resp = await apiClient.post(`/rule-chains/${chainId}/versions`, {
        graph_hash: `hash-v1-${suffix}`
      }, ACCOUNT);
      expectSuccess(v1Resp);
      expect(v1Resp.data.version).to.be.an('object');
      expect(v1Resp.data.version.version).to.equal(1);
      expect(v1Resp.data.version.status).to.equal('draft');
      expect(v1Resp.data.audit).to.be.an('object');
    });

    it('相同图哈希再次创建草稿时幂等去重（不产生空版本）', async function () {
      const dupResp = await apiClient.post(`/rule-chains/${chainId}/versions`, {
        graph_hash: `hash-v1-${suffix}`
      }, ACCOUNT);
      expectSuccess(dupResp);
      // 图未变更时不产生新版本，data.version 应当为 null
      expect(dupResp.data.version).to.be.null;
    });

    it('发布草稿版本 v1 并拦截重复发布', async function () {
      const pubResp = await apiClient.post('/rule-chains/versions/publish', {
        chain_id: chainId,
        version: 1
      }, ACCOUNT);
      expectSuccess(pubResp);
      expect(pubResp.data.audit).to.be.an('object');
      const auditAction = pubResp.data.audit.Action || pubResp.data.audit.action;
      expect(auditAction).to.equal('published');

      // 重复发布 v1 应当被拦截拒绝
      const rePubResp = await apiClient.post('/rule-chains/versions/publish', {
        chain_id: chainId,
        version: 1
      }, ACCOUNT);
      expect(rePubResp.code).to.not.equal(200);
      expect(rePubResp.message).to.be.a('string');
    });

    it('创建草稿 v2 并从已发布的 v1 回滚，生成新草稿 v3', async function () {
      // 创建草稿 v2
      const v2Resp = await apiClient.post(`/rule-chains/${chainId}/versions`, {
        graph_hash: `hash-v2-${suffix}`
      }, ACCOUNT);
      expectSuccess(v2Resp);
      expect(v2Resp.data.version.version).to.equal(2);
      expect(v2Resp.data.version.status).to.equal('draft');

      // 从已发布的 v1 执行回滚
      const rollbackResp = await apiClient.post('/rule-chains/versions/rollback', {
        chain_id: chainId,
        version: 1
      }, ACCOUNT);
      expectSuccess(rollbackResp);
      expect(rollbackResp.data.version).to.be.an('object');
      expect(rollbackResp.data.version.version).to.equal(3);
      expect(rollbackResp.data.version.status).to.equal('draft');
      expect(rollbackResp.data.version.rolled_back_from).to.equal(1);

      // 查询全量版本列表
      const listResp = await apiClient.get(`/rule-chains/${chainId}/versions`, {}, ACCOUNT);
      expectSuccess(listResp);
      expect(listResp.data).to.be.an('array').with.lengthOf(3);
      const statuses = listResp.data.map(v => ({ ver: v.version, status: v.status }));
      expect(statuses).to.deep.include({ ver: 1, status: 'published' });
      expect(statuses).to.deep.include({ ver: 2, status: 'draft' });
      expect(statuses).to.deep.include({ ver: 3, status: 'draft' });
    });

    it('跨租户无法操作或查看他人规则链版本', async function () {
      const otherListResp = await apiClient.get(`/rule-chains/${chainId}/versions`, {}, OTHER_ACCOUNT);
      expectSuccess(otherListResp);
      expect(otherListResp.data).to.deep.equal([]);

      const otherPubResp = await apiClient.post('/rule-chains/versions/publish', {
        chain_id: chainId,
        version: 2
      }, OTHER_ACCOUNT);
      expect(otherPubResp.code).to.not.equal(200);
    });
  });

  describe('2. 死信队列（DLQ Sink）持久化与观测 (Gate 1 & 2)', function () {
    let chainId = '';
    let deviceSeed;
    let failingExecId = '';

    before(async function () {
      if (!(await isMqttBrokerAvailable())) {
        this.skip();
      }
      deviceSeed = await createSimulationDevice(ACCOUNT);
      cleanups.push(async () => {
        await deviceSeed.cleanup();
      });

      // 配置一个必然失败的节点：指向不可达端口，并且配置 policy (max_attempts=2, backoff_ms=50, dead_letter=true)
      const failingGraph = {
        nodes: [
          { id: 't_fail', type: 'trigger.telemetry', config: { debug: true, trace: true } },
          {
            id: 'node_failing',
            type: 'action.webhook',
            config: {
              debug: true,
              trace: true,
              url: 'https://dlq-failing.invalid/sink',
              policy: {
                timeout_ms: 200,
                max_attempts: 2,
                backoff_ms: 50,
                dead_letter: true,
                retry_safe: true
              }
            }
          }
        ],
        edges: [{ from: 't_fail', to: 'node_failing' }]
      };

      const chainResp = await apiClient.post('/rule-chains', {
        name: `rc-dlq-${suffix}`,
        description: 'dlq sink failure test chain',
        enabled: true,
        graph: failingGraph
      }, ACCOUNT);
      expectSuccess(chainResp);
      chainId = chainResp.data.id;
      cleanups.push(async () => {
        await apiClient.delete(`/rule-chains/${chainId}`, {}, ACCOUNT);
      });

      // 等待规则链缓存生效
      await new Promise(resolve => setTimeout(resolve, 1500));
    });

    it('触发节点终局失败，自动下沉并持久化至死信队列', async function () {
      if (!deviceSeed || !chainId) this.skip();

      // 上报遥测触发规则链
      // MQTT 适配器要求原生信封结构 {"device_id": id, "values": base64(payload)}
      const envelope = JSON.stringify({
        device_id: deviceSeed.id,
        values: Buffer.from(JSON.stringify({ current_power: 120 }), 'utf8').toString('base64')
      });
      const simResp = await apiClient.post('/telemetry/datas/simulation/send', {
        device_id: deviceSeed.id,
        data: envelope
      }, ACCOUNT);
      if (!simResp || simResp.code !== 200) {
        await publishMqttRawTelemetry(deviceSeed, envelope, ACCOUNT);
      }

      // 轮询查询死信队列（最多等待 15 秒）
      const deadline = Date.now() + 15000;
      let deadLetters = [];
      while (Date.now() < deadline) {
        const resp = await apiClient.get(`/rule-chains/${chainId}/dead-letters`, {
          page: 1,
          page_size: 20
        }, ACCOUNT);
        if (resp && resp.code === 200 && resp.data && resp.data.list && resp.data.list.length > 0) {
          deadLetters = resp.data.list;
          break;
        }
        await new Promise(resolve => setTimeout(resolve, 600));
      }

      expect(deadLetters, '死信队列中应当包含终局失败记录').to.have.lengthOf.at.least(1);
      const dl = deadLetters.find(item => item.node_id === 'node_failing');
      expect(dl).to.be.an('object');
      expect(dl.chain_id).to.equal(chainId);
      expect(dl.node_type).to.equal('action.webhook');
      expect(dl.attempts).to.equal(2); // 验证重试了 2 次并可观测
      expect(dl.error).to.be.a('string').and.not.be.empty;
      expect(dl.exec_id).to.be.a('string').and.not.be.empty;
      expect(dl.tenant_id).to.be.a('string').and.not.be.empty;

      failingExecId = dl.exec_id;
    });

    it('全局租户级死信端点支持 exec_id 过滤查询', async function () {
      if (!failingExecId) this.skip();

      const resp = await apiClient.get('/rule-chains/dead-letters', {
        exec_id: failingExecId
      }, ACCOUNT);
      expectSuccess(resp);
      expect(resp.data.list).to.be.an('array').with.lengthOf.at.least(1);
      expect(resp.data.list[0].exec_id).to.equal(failingExecId);
    });

    it('跨租户无法读取死信记录', async function () {
      if (!chainId) this.skip();

      const resp = await apiClient.get(`/rule-chains/${chainId}/dead-letters`, {}, OTHER_ACCOUNT);
      // 其他租户由于不拥有该 chainId，应返回 404 或空列表
      expect([200, NOT_FOUND_CODE]).to.include(resp.code);
      if (resp.code === 200) {
        expect(resp.data.list).to.deep.equal([]);
      }
    });

    describe('3. 单消息 Trace 串联与耗时可观测 (Gate 3 & 2)', function () {
      it('按 exec_id 能够完整串联消息在 DAG 节点间的时序轨迹与耗时', async function () {
        if (!failingExecId || !chainId) this.skip();

        const traceResp = await apiClient.get(`/rule-chains/${chainId}/executions/${failingExecId}/traces`, {}, ACCOUNT);
        expectSuccess(traceResp);
        expect(traceResp.data).to.be.an('array').with.lengthOf.at.least(2);

        // 验证 trace 的时序与节点身份
        const triggerTrace = traceResp.data.find(t => t.node_id === 't_fail');
        const actionTrace = traceResp.data.find(t => t.node_id === 'node_failing');

        expect(triggerTrace, '触发节点 trace 必须记录').to.be.an('object');
        expect(triggerTrace.pass).to.be.true;
        expect(triggerTrace.exec_id).to.equal(failingExecId);
        expect(triggerTrace.elapsed_ms).to.be.a('number').and.at.least(0);

        expect(actionTrace, '失败节点 trace 必须记录').to.be.an('object');
        expect(actionTrace.pass).to.be.false;
        expect(actionTrace.exec_id).to.equal(failingExecId);
        expect(actionTrace.error_msg).to.be.a('string').and.not.be.empty;
        expect(actionTrace.elapsed_ms).to.be.a('number').and.at.least(0);
      });

      it('跨租户查询执行批次 Trace 返回空列表', async function () {
        if (!failingExecId) this.skip();

        const resp = await apiClient.get(`/rule-chains/executions/${failingExecId}/traces`, {}, OTHER_ACCOUNT);
        expectSuccess(resp);
        expect(resp.data).to.deep.equal([]);
      });
    });

    describe('4. 输入回放执行面与不可逆副作用闸门 (Gate 5)', function () {
      it('能够成功读取捕获的输入快照记录 (replay records)', async function () {
        if (!failingExecId || !chainId) this.skip();

        const recResp = await apiClient.get(`/rule-chains/${chainId}/executions/${failingExecId}/replay-records`, {}, ACCOUNT);
        expectSuccess(recResp);
        expect(recResp.data).to.be.an('array').with.lengthOf.at.least(1);
        const record = recResp.data[0];
        const recordExecId = record.execution_id || record.ExecutionID;
        expect(recordExecId).to.equal(failingExecId);
        expect(record.payload || record.Payload).to.be.a('string').and.not.be.empty;
      });

      it('未确认副作用 (confirm_side_effects=false) 时，回放被安全闸门拒绝拦截', async function () {
        if (!failingExecId || !chainId) this.skip();

        const replayResp = await apiClient.post(`/rule-chains/${chainId}/replay`, {
          execution_id: failingExecId,
          confirm_side_effects: false
        }, ACCOUNT);

        // 必须被闸门拦截拒绝，返回 CodeOpDenied 错误码并明确提示包含副作用节点
        expect(replayResp.code).to.not.equal(200);
        expect(replayResp.message).to.include('replay would re-run side-effect nodes');
        expect(replayResp.message).to.include('ConfirmSideEffects');
      });

      it('显式确认副作用 (confirm_side_effects=true) 时，回放放行执行并完成节点重跑', async function () {
        if (!failingExecId || !chainId) this.skip();

        const replayResp = await apiClient.post(`/rule-chains/${chainId}/replay`, {
          execution_id: failingExecId,
          confirm_side_effects: true
        }, ACCOUNT);

        expectSuccess(replayResp);
        expect(replayResp.data).to.be.an('object');
        expect(replayResp.data.execution_id).to.equal(failingExecId);
        expect(replayResp.data.replayed_nodes).to.be.a('number').and.at.least(1);
        expect(replayResp.data.errors).to.be.an('array'); // node_failing 的不可达错误会被记录但不触发闸门 panic
      });

      it('跨租户回放他人执行批次被拒绝', async function () {
        if (!failingExecId || !chainId) this.skip();

        const replayResp = await apiClient.post(`/rule-chains/${chainId}/replay`, {
          execution_id: failingExecId,
          confirm_side_effects: true
        }, OTHER_ACCOUNT);

        expect([NOT_FOUND_CODE, PARAM_ERROR_CODE, OP_DENIED_CODE]).to.include(replayResp.code);
      });
    });
  });
});
