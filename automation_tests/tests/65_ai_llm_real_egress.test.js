/**
 * 文件用途：TB-14「AI 规则节点/LLM 真实模型端点联调」的运行期证据（失败路径）。
 *
 * 场景：向真实公网端点（https://api.openai.com/v1）发起一次真实 HTTPS 调用，
 * 使用无效凭据（canary key）——OpenAI 返回 401。这证明：
 *   1. DNS 解析 → TLS 连接 → 请求形状（POST /chat/completions）→ 响应处理
 *      的完整 egress 链路在运行期真实可达；
 *   2. 错误如实上抛（"AI provider returned HTTP 401"），不假装成功；
 *   3. canary 凭据不出现在任何响应里（P0.7 无明文契约的延续）。
 *
 * 如实边界：
 *   - 成功推理需要有效凭据（真实 LLM 账号），本用例不伪造；
 *   - **有出口的机器上**本用例断言 401（真实 egress 证据）；本机无直接公网出口
 *     （safe-egress 设计上禁代理），传输失败时用例 skip 并写明——环境阻塞
 *     与代码缺陷严格分账。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'TB-14 AI LLM real endpoint egress [65_ai_llm_real_egress]';
const ACCOUNT = 'tenant_admin';
const CANARY_KEY = 'TB14-CANARY-4f8d2a91-real-endpoint-probe-key';

describe(SUITE, function () {
  this.timeout(120000);

  let modelId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 65_ai_llm_real_egress.test.js');
    }
    await apiClient.login(ACCOUNT);

    const create = await apiClient.post('/ai/models', {
      name: 'tb14-real-egress-' + Date.now(),
      base_url: 'https://api.openai.com/v1',
      model: 'gpt-4o-mini',
      api_key: CANARY_KEY,
      purpose: 'chat'
    }, ACCOUNT);
    expect(create.code, 'create AI model profile').to.equal(200);
    modelId = create.data && (create.data.id || create.data.ID);
    expect(modelId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (modelId) {
      try {
        await apiClient.delete('/ai/models/' + modelId, {}, ACCOUNT);
      } catch (err) { /* 尽力回收 */ }
    }
  });

  it('reaches the real provider over HTTPS and surfaces the 401 without leaking credentials', async function () {
    const resp = await apiClient.post('/ai/assistant/chat', {
      model_id: modelId,
      messages: [{ role: 'user', content: 'tb14 egress health probe' }],
      max_tokens: 8
    }, ACCOUNT);

    // 无效凭据必须被如实拒绝——平台不伪造成功，也不吞掉错误。
    expect(resp.code, 'invalid credentials must surface as an error').to.not.equal(200);
    const message = resp.message || '';

    if (message.includes('AI provider request failed')) {
      // 传输层失败 = 本机没有直接公网出口（safe-egress 设计上禁代理）。
      // 这是环境阻塞，不是代码缺陷——skip 并写明，绝不伪装成通过。
      this.skip('egress transport unreachable on this machine (direct internet required; safe-egress forbids proxies)');
    }

    expect(message, 'provider status must be surfaced')
      .to.include('AI provider returned HTTP 401');
    expect(JSON.stringify(resp), 'canary key must never leak').to.not.include(CANARY_KEY);
  });
});
