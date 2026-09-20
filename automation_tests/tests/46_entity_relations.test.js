/**
 * 文件用途：通用实体关系（ROADMAP P1.1）的 API 契约测试。
 *
 * 覆盖的是"设备/资产/客户/网关之间那条有向关系边"的四面：
 *   1. 入参校验：缺字段、未知实体类型、超长 ID、超长关系类型、超大数据、自环必须被拒。
 *   2. 正常链路：创建 → 列表读回 → 按方向查询 → 按 ID 删除 → 再读已消失。
 *   3. 幂等：同端点同类型重复创建的实际行为（本用例记录实测，不猜）。
 *   4. 租户边界：别的租户管理员既读不到也删不掉本租户的关系；
 *      且别的租户创建同一条边时**只能落在自己的租户里**，不会命中/改写本租户的行。
 *   5. 删除保护：按实体删除默认 protect（有关系则拒绝），cascade 必须显式声明。
 *
 * 关键注意事项：
 *   - 租户**永远来自调用方 claims**，请求体/查询串里没有 tenant_id 入参（前端
 *     `service/api/entity-relation.ts` 刻意不提供该参数）。因此"跨租户"在本接口上
 *     的正确预期不是"写入被拒"，而是"隔离"：别的租户写不进本租户的图，也读不到。
 *     用例按隔离语义断言，不按"报错"断言。
 *   - 跨租户用 tenant_admin_b（第二租户管理员），不用 TENANT_USER —— 后者对
 *     多数业务路由本身就是 fail-closed，测出来的拒绝分不清是"跨租户被拦"
 *     还是"角色本身没权限"。
 *   - 关系是**有向**的：反向边必须显式写入，查询层不脑补。用例用
 *     `direction=in` 在只有出边时返回空来锁死这一点。
 *   - `DELETE /entity-relations/:id` 对"不存在"与"越权"都返回 **404**，不区分原因
 *     （区分就等于告诉调用方"这个 ID 在别处存在"）。前端 `index.vue:136` 只判断
 *     `error` 是否存在、不读 `code`，所以按 handler 注释收敛成 404 是安全的。
 *   - 端点 ID 不做存在性校验：关系可以指向不存在的实体，也可以指向别的租户的实体 ID。
 *     这是当前实现的真实状态（无 FK、无存在性查询），用例如实记录，不粉饰。
 *
 * 重构建议：若后续补上"端点实体必须存在且属于本租户"的前置校验，把
 * "creates a relation whose endpoints do not exist" 这条的预期改为 100002/100404。
 */

const { randomUUID } = require('crypto');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Generic entity relations [46_entity_relations]';
const ACCOUNT = 'tenant_admin';
const OTHER_TENANT_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NOT_FOUND = 100404;
const CODE_OP_DENIED = 201002;

const MISSING_ID = '00000000-0000-0000-0000-000000000000';
const PATH = '/entity-relations';

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  return resp.data;
}

function expectCode(resp, code, label) {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: expected ${code} but got ${resp.code} (${resp.message || ''})`).to.equal(code);
}

function relationList(data) {
  expect(data, 'list payload').to.be.an('object');
  expect(data.list, 'data.list').to.be.an('array');
  expect(data.total, 'data.total').to.be.a('number');
  return data.list;
}

function idsOf(list) {
  return list.map(item => item.id);
}

async function createRelation(payload, accountKey = ACCOUNT) {
  return apiClient.post(PATH, payload, accountKey);
}

async function listRelations(params, accountKey = ACCOUNT) {
  return relationList(expectOk(await apiClient.get(PATH, params, accountKey)));
}

function bigMetadata(bytes) {
  // 造一个确定超过 8192 字节的 JSON 字符串（服务层按序列化后字节数校验）。
  return JSON.stringify({ blob: 'x'.repeat(bytes) });
}

describe(SUITE, function () {
  this.timeout(120000);

  // 每个端点用一次性 UUID：既不污染其它租户/其它轮次的数据，
  // 又能让"按 from_id 过滤"的断言不受历史残留影响。
  let deviceId = null;
  let assetId = null;
  let gatewayId = null;
  const created = []; // { id, account }

  async function trackCreate(payload, accountKey = ACCOUNT) {
    const data = expectOk(await createRelation(payload, accountKey));
    expect(data.id, 'created relation id').to.be.a('string').and.not.equal('');
    created.push({ id: data.id, account: accountKey });
    return data;
  }

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 46_entity_relations.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_TENANT_ACCOUNT);

    deviceId = randomUUID();
    assetId = randomUUID();
    gatewayId = randomUUID();
  });

  after(async function () {
    for (const item of created) {
      // 幂等清理：已经删过（或已被级联删掉）的行再删一次只会得到 deleted: 0。
      await apiClient.delete(`${PATH}/${item.id}`, {}, item.account);
    }
  });

  // ---------------------------------------------------------------- 入参校验

  it('rejects a create payload that omits every required field', async function () {
    const resp = await createRelation({});
    expectCode(resp, CODE_PARAM_ERROR, 'empty create payload');
  });

  it('rejects an entity type that is outside the controlled whitelist', async function () {
    const resp = await createRelation({
      from_type: 'banana',
      from_id: deviceId,
      relation_type: 'installed_in',
      to_type: 'asset',
      to_id: assetId
    });
    expectCode(resp, CODE_PARAM_ERROR, 'unknown from_type');
  });

  it('rejects a self-loop (same entity type and same entity id)', async function () {
    const resp = await createRelation({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'manages',
      to_type: 'device',
      to_id: deviceId
    });
    expectCode(resp, CODE_PARAM_ERROR, 'self loop');
  });

  it('rejects an entity id longer than 36 characters', async function () {
    const resp = await createRelation({
      from_type: 'device',
      from_id: 'd'.repeat(37),
      relation_type: 'installed_in',
      to_type: 'asset',
      to_id: assetId
    });
    expectCode(resp, CODE_PARAM_ERROR, 'over-long from_id');
  });

  it('rejects a relation type longer than 64 characters', async function () {
    const resp = await createRelation({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'r'.repeat(65),
      to_type: 'asset',
      to_id: assetId
    });
    expectCode(resp, CODE_PARAM_ERROR, 'over-long relation_type');
  });

  it('rejects metadata larger than the 8 KiB budget', async function () {
    const resp = await createRelation({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'installed_in',
      to_type: 'asset',
      to_id: assetId,
      metadata: bigMetadata(9000)
    });
    expectCode(resp, CODE_PARAM_ERROR, 'oversized metadata');
  });

  it('rejects a list query without any endpoint filter (no silent full-tenant dump)', async function () {
    const resp = await apiClient.get(PATH, {}, ACCOUNT);
    expectCode(resp, CODE_PARAM_ERROR, 'empty list query');
  });

  it('rejects an invalid direction value', async function () {
    const resp = await apiClient.get(
      PATH,
      { entity_type: 'device', entity_id: deviceId, direction: 'sideways' },
      ACCOUNT
    );
    expectCode(resp, CODE_PARAM_ERROR, 'invalid direction');
  });

  it('rejects an incomplete entity filter (type without id)', async function () {
    const resp = await apiClient.get(PATH, { entity_type: 'device' }, ACCOUNT);
    expectCode(resp, CODE_PARAM_ERROR, 'incomplete entity filter');
  });

  it('rejects non-integer limit/offset', async function () {
    const resp = await apiClient.get(
      PATH,
      { from_type: 'device', from_id: deviceId, limit: 'many' },
      ACCOUNT
    );
    expectCode(resp, CODE_PARAM_ERROR, 'non-integer limit');
  });

  // ---------------------------------------------------------------- 正常链路

  it('creates a relation and reads it back through the list endpoint', async function () {
    const createdRelation = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'installed_in',
      to_type: 'asset',
      to_id: assetId,
      metadata: JSON.stringify({ rack: 'A-01' })
    });

    expect(createdRelation.from_type, 'from_type is echoed').to.equal('device');
    expect(createdRelation.from_id).to.equal(deviceId);
    expect(createdRelation.relation_type).to.equal('installed_in');
    expect(createdRelation.to_type).to.equal('asset');
    expect(createdRelation.to_id).to.equal(assetId);

    const listed = await listRelations({ from_type: 'device', from_id: deviceId });
    expect(idsOf(listed), 'the new relation is visible in the list').to.include(createdRelation.id);
  });

  it('defaults omitted metadata to an empty object', async function () {
    const createdRelation = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'monitors',
      to_type: 'gateway',
      to_id: gatewayId
    });
    expect(
      JSON.parse(createdRelation.metadata),
      'metadata defaults to {} when omitted'
    ).to.deep.equal({});
  });

  it('records the actual behaviour of creating the same edge twice (idempotency probe)', async function () {
    const first = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'backed_up_by',
      to_type: 'gateway',
      to_id: gatewayId
    });
    const before = await listRelations({ from_type: 'device', from_id: deviceId });

    // 实测预期：唯一约束冲突被 service 层收敛为幂等——返回 200 且回填既有记录（同 ID），
    // 不新增行。若哪天改成报错，这条会红，那也是"实际行为变了"的信号，不是用例写错。
    const resp = await createRelation({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'backed_up_by',
      to_type: 'gateway',
      to_id: gatewayId
    });
    expect(resp.code, `duplicate create: got ${resp.code} (${resp.message || ''})`).to.equal(200);
    expect(resp.data.id, 'duplicate create returns the existing record (idempotent)').to.equal(first.id);

    const after = await listRelations({ from_type: 'device', from_id: deviceId });
    expect(after.length, 'duplicate create does not add a second row').to.equal(before.length);
  });

  it('treats the relation as directed: no reverse edge is fabricated', async function () {
    const listed = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'installed_in',
      to_type: 'asset',
      to_id: assetId
    });

    const out = await listRelations({ entity_type: 'device', entity_id: deviceId, direction: 'out' });
    expect(idsOf(out), 'the device is the "from" side').to.include(listed.id);

    const inOnTarget = await listRelations({ entity_type: 'asset', entity_id: assetId, direction: 'in' });
    expect(idsOf(inOnTarget), 'the asset is the "to" side').to.include(listed.id);

    const inOnSource = await listRelations({ entity_type: 'device', entity_id: deviceId, direction: 'in' });
    expect(idsOf(inOnSource), 'no reverse edge is invented for the source').to.not.include(listed.id);
  });

  it('creates a relation whose endpoint entities do not exist (no referential check)', async function () {
    // 当前实现只校验类型白名单与自环，不校验端点实体是否存在、是否属于本租户。
    // 这条锁住现状：如果哪天补了存在性校验，本条应改为断言被拒。
    const ghostId = randomUUID();
    const createdRelation = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'points_at_nothing',
      to_type: 'customer',
      to_id: ghostId
    });
    expect(createdRelation.to_id, 'the dangling endpoint is stored as-is').to.equal(ghostId);
  });

  it('rejects metadata that is not valid JSON at the parameter boundary', async function () {
    // metadata 以 *string 直写 jsonb 列。若服务层不做 JSON 校验，非法串会一路传到
    // 数据库、以 SQLSTATE 22P02 的形式暴露成 100000（系统错误），既不是参数错误，
    // 也会把库内细节带出去。这里锁死：必须是 100002 参数错误，且原因来自元数据校验。
    const resp = await createRelation({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'broken_metadata',
      to_type: 'asset',
      to_id: assetId,
      metadata: 'not-json-at-all'
    });
    expectCode(resp, CODE_PARAM_ERROR, 'invalid metadata');
    expect(String(resp.message), 'the rejection must come from metadata validation').to.include('JSON');
  });

  // ---------------------------------------------------------------- 删除

  it('deletes a relation by id and stops returning it', async function () {
    const createdRelation = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'temp_link',
      to_type: 'asset',
      to_id: assetId
    });
    const before = await listRelations({ from_type: 'device', from_id: deviceId });
    expect(idsOf(before)).to.include(createdRelation.id);

    const data = expectOk(await apiClient.delete(`${PATH}/${createdRelation.id}`, {}, ACCOUNT));
    expect(data.deleted, 'exactly one row is deleted').to.equal(1);

    const after = await listRelations({ from_type: 'device', from_id: deviceId });
    expect(idsOf(after), 'the deleted relation is gone').to.not.include(createdRelation.id);
  });

  it('returns not found when deleting an id that does not exist', async function () {
    // 修前是 200 + deleted: 0（与 handler 注释、前端 wrapper 注释都不一致）。
    // 前端只判断 error 是否存在、不看 code，故按注释收敛成 404。
    const resp = await apiClient.delete(`${PATH}/${MISSING_ID}`, {}, ACCOUNT);
    expectCode(resp, CODE_NOT_FOUND, 'unknown relation id');
  });

  it('refuses to delete an entity relations by entity while any relation exists (protect is the default)', async function () {
    const resp = await apiClient.delete(
      `${PATH}/by-entity`,
      { entity_type: 'device', entity_id: deviceId },
      ACCOUNT
    );
    expectCode(resp, CODE_OP_DENIED, 'protect policy');
  });

  it('rejects an unknown cascade policy', async function () {
    const resp = await apiClient.delete(
      `${PATH}/by-entity`,
      { entity_type: 'device', entity_id: deviceId, policy: 'nuke' },
      ACCOUNT
    );
    expectCode(resp, CODE_PARAM_ERROR, 'unknown policy');
  });

  it('rejects by-entity deletion for an entity type outside the whitelist', async function () {
    const resp = await apiClient.delete(
      `${PATH}/by-entity`,
      { entity_type: 'toaster', entity_id: deviceId, policy: 'cascade' },
      ACCOUNT
    );
    expectCode(resp, CODE_PARAM_ERROR, 'unknown entity type');
  });

  // ---------------------------------------------------------------- 租户边界

  it('hides a relation from another tenant', async function () {
    const createdRelation = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'installed_in',
      to_type: 'asset',
      to_id: assetId
    });

    const visible = await listRelations({ from_type: 'device', from_id: deviceId }, ACCOUNT);
    expect(idsOf(visible), 'the owner sees its own relation').to.include(createdRelation.id);

    const otherList = await listRelations(
      { from_type: 'device', from_id: deviceId },
      OTHER_TENANT_ACCOUNT
    );
    expect(idsOf(otherList), 'another tenant must not see the relation').to.not.include(createdRelation.id);
    expect(otherList.length, 'another tenant sees no rows for this endpoint').to.equal(0);
  });

  it('does not let another tenant delete our relation', async function () {
    const createdRelation = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'cross_tenant_delete_probe',
      to_type: 'asset',
      to_id: assetId
    });

    const resp = await apiClient.delete(`${PATH}/${createdRelation.id}`, {}, OTHER_TENANT_ACCOUNT);
    // 跨租户删除必须表现为"不存在"（404），不能是 200 + deleted: 0 —— 后者会让调用方以为删成功了。
    expectCode(resp, CODE_NOT_FOUND, 'cross-tenant delete');

    const stillThere = await listRelations({ from_type: 'device', from_id: deviceId }, ACCOUNT);
    expect(idsOf(stillThere), 'the relation survives the cross-tenant delete').to.include(createdRelation.id);
  });

  it('keeps another tenant write inside its own tenant graph', async function () {
    const mine = await trackCreate({
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'shared_edge_probe',
      to_type: 'asset',
      to_id: assetId
    });

    // 另一个租户写同一条边：唯一约束含 tenant_id，所以不会命中本租户的行，
    // 而是在它自己的租户里新建一条——隔离成立，但也不是"报错拒绝"。
    const theirs = expectOk(
      await createRelation(
        {
          from_type: 'device',
          from_id: deviceId,
          relation_type: 'shared_edge_probe',
          to_type: 'asset',
          to_id: assetId
        },
        OTHER_TENANT_ACCOUNT
      )
    );
    created.push({ id: theirs.id, account: OTHER_TENANT_ACCOUNT });
    expect(theirs.id, 'the other tenant gets its own row, not ours').to.not.equal(mine.id);

    const myList = await listRelations({ from_type: 'device', from_id: deviceId }, ACCOUNT);
    expect(
      myList.filter(item => item.relation_type === 'shared_edge_probe').length,
      'our tenant still holds exactly one row for this edge'
    ).to.equal(1);
    expect(
      myList.filter(item => item.id === theirs.id).length,
      'their row never leaks into our tenant'
    ).to.equal(0);
  });

  // ---------------------------------------------------------------- 认证

  it('rejects unauthenticated create, list and delete', async function () {
    const postResp = await apiClient.postNoAuth(PATH, {
      from_type: 'device',
      from_id: deviceId,
      relation_type: 'anonymous',
      to_type: 'asset',
      to_id: assetId
    });
    expect(postResp.code, 'unauthenticated create').to.equal(401);
    expect(String(postResp.message), 'unauthenticated create message').to.include('missing authentication');

    const getResp = await apiClient.getNoAuth(PATH, { from_type: 'device', from_id: deviceId });
    expect(getResp.code, 'unauthenticated list').to.equal(401);
    expect(String(getResp.message), 'unauthenticated list message').to.include('missing authentication');

    const delResp = await apiClient.deleteNoAuth(`${PATH}/${MISSING_ID}`);
    expect(delResp.code, 'unauthenticated delete').to.equal(401);
    expect(String(delResp.message), 'unauthenticated delete message').to.include('missing authentication');
  });

  // ---------------------------------------------------------------- 显式级联（放最后，会清掉本轮数据）

  it('cascades by-entity deletion only when the caller explicitly asks for it', async function () {
    const before = await listRelations({ entity_type: 'device', entity_id: deviceId, direction: 'any' });
    expect(before.length, 'the fixture device has at least one relation to cascade').to.be.greaterThan(0);

    const data = expectOk(
      await apiClient.delete(
        `${PATH}/by-entity`,
        { entity_type: 'device', entity_id: deviceId, policy: 'cascade' },
        ACCOUNT
      )
    );
    expect(data.deleted, 'cascade removes the relations').to.be.greaterThan(0);

    const after = await listRelations({ entity_type: 'device', entity_id: deviceId, direction: 'any' });
    expect(after.length, 'no relation survives the cascade').to.equal(0);
  });
});
