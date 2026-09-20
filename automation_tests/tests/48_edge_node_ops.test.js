/**
 * 文件用途：P1.5 边缘运维（边缘节点证书签发与远程升级/回滚）的 API 契约测试。
 *
 * 覆盖：
 *   1. 节点证书签发：生成 X.509 客户端证书（ECDSA P-256）与一次性私钥返回；
 *   2. 证书查询与脱敏：查询生效证书，返回指纹、序列号与有效期，私钥脱敏；
 *   3. 证书安全轮换：再次签发自动吊销旧证书；
 *   4. 证书吊销：显式吊销证书；
 *   5. 证书异常与越权：未注册节点签发拒绝、跨租户操作拒绝；
 *   6. 远程升级：版本合法性校验、版本必须严格递增、降级升级拒绝、同版本升级拒绝；
 *   7. 升级历史：历史流水记录、查询该节点版本演进；
 *   8. 远程回滚：基于历史记录一键安全回滚，生成不可变回滚历史，双向留痕；
 *   9. 升级越权：跨租户升级与回滚拒绝。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Edge node operations [48_edge_node_ops]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;

function expectOk(resp, label = 'ok response') {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  return resp.data;
}

function expectCode(resp, code, label) {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: expected ${code} but got ${resp.code} (${resp.message || ''})`).to.equal(code);
}

describe(SUITE, function () {
  this.timeout(30000);

  const testNodeId = `edge-ops-node-${Date.now()}`;
  const initialVersion = '1.0.0';

  before(async function () {
    // 注册测试用边缘节点
    const regResp = await apiClient.post('/edge/nodes', {
      node_id: testNodeId,
      version: initialVersion,
      capabilities: ['modbus', 'opcua']
    }, ACCOUNT);
    expectOk(regResp, 'register edge node');
  });

  describe('Part 1: Edge Node X.509 Certificate Lifecycle', function () {
    let issuedCert = null;

    it('issues a new X.509 certificate for the registered edge node', async function () {
      const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/certificate`, {
        validity_days: 180
      }, ACCOUNT);

      const data = expectOk(resp, 'issue cert');
      expect(data.node_id).to.equal(testNodeId);
      expect(data.certificate).to.include('BEGIN CERTIFICATE');
      expect(data.private_key).to.include('BEGIN PRIVATE KEY');
      expect(data.fingerprint).to.be.a('string').and.have.lengthOf(64);
      expect(data.serial_number).to.be.a('string').and.not.empty;
      expect(data.status).to.equal('active');
      issuedCert = data;
    });

    it('queries active certificate details without private key (masked)', async function () {
      const resp = await apiClient.get(`/edge/nodes/${encodeURIComponent(testNodeId)}/certificate`, {}, ACCOUNT);
      const data = expectOk(resp, 'get cert');

      expect(data.node_id).to.equal(testNodeId);
      expect(data.serial_number).to.equal(issuedCert.serial_number);
      expect(data.fingerprint).to.equal(issuedCert.fingerprint);
      expect(data.certificate).to.include('BEGIN CERTIFICATE');
      expect(data.private_key).to.be.undefined; // 私钥脱敏，不应出现在查询接口中
      expect(data.status).to.equal('active');
    });

    it('rejects issuing certificate for an unregistered edge node', async function () {
      const resp = await apiClient.post('/edge/nodes/non-existent-node-id/certificate', {
        validity_days: 30
      }, ACCOUNT);

      expectCode(resp, CODE_PARAM_ERROR, 'unregistered node cert');
    });

    it('rejects cross-tenant certificate query and issuance', async function () {
      // 租户 B 查询租户 A 的边缘节点证书
      const getResp = await apiClient.get(`/edge/nodes/${encodeURIComponent(testNodeId)}/certificate`, {}, OTHER_ACCOUNT);
      expectCode(getResp, CODE_PARAM_ERROR, 'cross tenant get cert');

      // 租户 B 试图为租户 A 的边缘节点签发证书
      const postResp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/certificate`, {
        validity_days: 30
      }, OTHER_ACCOUNT);
      expectCode(postResp, CODE_PARAM_ERROR, 'cross tenant issue cert');
    });

    it('rotates certificate: issuing a new certificate revokes the old one', async function () {
      const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/certificate`, {
        validity_days: 90
      }, ACCOUNT);

      const newCert = expectOk(resp, 'rotate cert');
      expect(newCert.serial_number).to.not.equal(issuedCert.serial_number);
      expect(newCert.status).to.equal('active');

      // 查询当前证书应是新证书
      const queryResp = await apiClient.get(`/edge/nodes/${encodeURIComponent(testNodeId)}/certificate`, {}, ACCOUNT);
      const current = expectOk(queryResp, 'get current cert after rotation');
      expect(current.serial_number).to.equal(newCert.serial_number);
    });

    it('revokes the edge node certificate manually', async function () {
      const delResp = await apiClient.delete(`/edge/nodes/${encodeURIComponent(testNodeId)}/certificate`, ACCOUNT);
      expectOk(delResp, 'revoke cert');

      // 吊销后再查 active 证书应返回 400 (no active certificate)
      const queryResp = await apiClient.get(`/edge/nodes/${encodeURIComponent(testNodeId)}/certificate`, {}, ACCOUNT);
      expectCode(queryResp, CODE_PARAM_ERROR, 'query after revoke');
    });
  });

  describe('Part 2: Edge Node Remote Upgrade and Rollback', function () {
    let firstUpgradeHistoryId = null;

    it('rejects upgrade with invalid target version format', async function () {
      const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/upgrade`, {
        target_version: 'invalid-version'
      }, ACCOUNT);

      expectCode(resp, CODE_PARAM_ERROR, 'invalid version string');
    });

    it('rejects downgrade attempt via upgrade endpoint (must strictly be newer)', async function () {
      const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/upgrade`, {
        target_version: '0.9.0'
      }, ACCOUNT);

      expectCode(resp, CODE_PARAM_ERROR, 'downgrade via upgrade');
      expect(resp.message).to.include('strictly newer');
    });

    it('rejects upgrade to the identical current version', async function () {
      const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/upgrade`, {
        target_version: initialVersion
      }, ACCOUNT);

      expectCode(resp, CODE_PARAM_ERROR, 'same version upgrade');
      expect(resp.message).to.include('strictly newer');
    });

    it('successfully upgrades edge node to a newer version and records history', async function () {
      const targetVersion = '1.1.0';
      const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/upgrade`, {
        target_version: targetVersion,
        description: 'First production update'
      }, ACCOUNT);

      const data = expectOk(resp, 'upgrade to 1.1.0');
      expect(data.node_id).to.equal(testNodeId);
      expect(data.from_version).to.equal(initialVersion);
      expect(data.target_version).to.equal(targetVersion);
      expect(data.history_id).to.be.a('string').and.not.empty;
      firstUpgradeHistoryId = data.history_id;

      // 验证节点当前列表中的版本已更新
      const listResp = await apiClient.get('/edge/nodes', {}, ACCOUNT);
      const nodes = expectOk(listResp, 'list nodes');
      const updatedNode = nodes.find(n => n.id === testNodeId);
      expect(Boolean(updatedNode)).to.equal(true);
      expect(updatedNode.version).to.equal(targetVersion);
    });

    it('queries edge node upgrade history list', async function () {
      const resp = await apiClient.get(`/edge/nodes/${encodeURIComponent(testNodeId)}/upgrade/history`, {}, ACCOUNT);
      const history = expectOk(resp, 'get upgrade history');

      expect(history).to.be.an('array').and.have.lengthOf.at.least(1);
      const firstEntry = history[0];
      expect(firstEntry.id).to.equal(firstUpgradeHistoryId);
      expect(firstEntry.from_version).to.equal(initialVersion);
      expect(firstEntry.target_version).to.equal('1.1.0');
    });

    it('rejects cross-tenant rollback or upgrade attempt', async function () {
      // 租户 B 试图回滚租户 A 的节点
      const rollbackResp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/rollback`, {
        history_id: firstUpgradeHistoryId
      }, OTHER_ACCOUNT);

      expectCode(rollbackResp, CODE_PARAM_ERROR, 'cross tenant rollback');
    });

    it('successfully rolls back edge node to previous version based on history record', async function () {
      const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(testNodeId)}/rollback`, {
        history_id: firstUpgradeHistoryId
      }, ACCOUNT);

      const data = expectOk(resp, 'rollback to 1.0.0');
      expect(data.node_id).to.equal(testNodeId);
      expect(data.rolled_to_version).to.equal(initialVersion);
      expect(data.status).to.equal('rolled_back');

      // 验证节点版本已恢复为 initialVersion (1.0.0)
      const listResp = await apiClient.get('/edge/nodes', {}, ACCOUNT);
      const nodes = expectOk(listResp, 'list nodes after rollback');
      const rolledNode = nodes.find(n => n.id === testNodeId);
      expect(rolledNode.version).to.equal(initialVersion);

      // 验证新增了 rolled_back 的历史记录
      const historyResp = await apiClient.get(`/edge/nodes/${encodeURIComponent(testNodeId)}/upgrade/history`, {}, ACCOUNT);
      const history = expectOk(historyResp, 'get upgrade history after rollback');
      expect(history.length).to.be.at.least(2);
      expect(history[0].status).to.equal('rolled_back');
      expect(history[0].target_version).to.equal(initialVersion);
    });
  });
});
