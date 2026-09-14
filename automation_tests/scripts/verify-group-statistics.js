// 运行期验证：分组统计（TP-8②）是否真的出现在 tree / list 响应里。
// 跑法：cd automation_tests && set -a && . ./.env.local && set +a && node scripts/verify-group-statistics.js
const fs = require('fs');
const path = require('path');

const BASE = 'http://127.0.0.1:9999/api/v1';

function tokenOf(role) {
  // 角色键用下划线（tenant_admin），storage state 文件名用连字符（tenant-admin.json）。
  const fileName = `${role.replace(/_/g, '-')}.json`;
  const p = path.resolve(__dirname, '..', 'e2e', '.auth', fileName);
  const j = JSON.parse(fs.readFileSync(p, 'utf8'));
  for (const origin of j.origins || []) {
    for (const kv of origin.localStorage || []) {
      if (kv.name === 'token') return JSON.parse(kv.value);
    }
  }
  throw new Error(`no token in ${p}`);
}

async function call(method, url, body, token) {
  const res = await fetch(`${BASE}${url}`, {
    method,
    headers: { 'x-token': token, 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined
  });
  return await res.json();
}

(async () => {
  const token = tokenOf('tenant_admin');
  const stamp = Date.now().toString(36);

  // 1) 父分组 + 子分组（子挂在父下，验证"父统计含子孙设备"）
  await call('POST', '/device/group', { name: `VS-Parent-${stamp}`, parent_id: '0' }, token);
  let tree = (await call('GET', '/device/group/tree', null, token)).data;
  const parent = tree.find(n => n.group && n.group.name === `VS-Parent-${stamp}`);
  if (!parent) throw new Error('parent group not found after create');
  const parentId = parent.group.id;

  await call('POST', '/device/group', { name: `VS-Child-${stamp}`, parent_id: parentId }, token);
  tree = (await call('GET', '/device/group/tree', null, token)).data;
  const parentNode = tree.find(n => n.group && n.group.id === parentId);
  const childNode = (parentNode.children || []).find(n => n.group && n.group.name === `VS-Child-${stamp}`);
  if (!childNode) throw new Error('child group not found under parent');
  const childId = childNode.group.id;

  // 2) 两台设备：一台挂父、一台挂子
  const createdDeviceIds = [];
  for (const [label, groupId] of [['parent', parentId], ['child', childId]]) {
    const name = `VS-Dev-${label}-${stamp}`;
    const created = await call('POST', '/device', {
      name,
      device_config_id: '',
      voucher: JSON.stringify({ username: name, password: `${name}-pw` })
    }, token);
    if (created.code !== 200) throw new Error(`create device ${name}: ${JSON.stringify(created)}`);
    // 设备 ID 由后端生成，必须从创建响应里取，不能用请求里的 name。
    const deviceId = created.data && created.data.id;
    if (!deviceId) throw new Error(`create device ${name} returned no id: ${JSON.stringify(created)}`);
    createdDeviceIds.push(deviceId);

    const bound = await call('POST', '/device/group/relation', { group_id: groupId, device_id_list: [deviceId] }, token);
    if (bound.code !== 200) throw new Error(`bind ${deviceId} -> ${groupId}: ${JSON.stringify(bound)}`);
  }

  // 3) 读 tree，断言统计
  tree = (await call('GET', '/device/group/tree', null, token)).data;
  const pNode = tree.find(n => n.group && n.group.id === parentId);
  const cNode = (pNode.children || []).find(n => n.group && n.group.id === childId);

  console.log('[tree] parent.statistics =', JSON.stringify(pNode.statistics));
  console.log('[tree] child.statistics  =', JSON.stringify(cNode.statistics));

  const parentTotal = pNode.statistics && pNode.statistics.device_total;
  const childTotal = cNode.statistics && cNode.statistics.device_total;

  // 4) 读 list，断言同一分组的统计一致
  const listResp = await call('GET', '/device/group?page=1&page_size=200', null, token);
  const listItems = listResp.data.list || [];
  const pItem = listItems.find(i => i.id === parentId);
  const cItem = listItems.find(i => i.id === childId);
  console.log('[list] parent.statistics =', JSON.stringify(pItem && pItem.statistics));
  console.log('[list] child.statistics  =', JSON.stringify(cItem && cItem.statistics));
  console.log('[list] parent 保留原有 group 字段 (name) =', pItem && pItem.name);

  const failures = [];
  if (!pNode.statistics || !cNode.statistics) failures.push('tree node missing statistics');
  if (parentTotal < 2) failures.push(`parent device_total=${parentTotal}, expected >=2 (own + child device)`);
  if (childTotal < 1) failures.push(`child device_total=${childTotal}, expected >=1`);
  if (parentTotal <= childTotal) failures.push('parent rollup must exceed child (child device included in parent)');
  if (!pItem || !pItem.statistics) failures.push('list item missing statistics');
  if (pItem && pItem.statistics && pItem.statistics.device_total !== parentTotal) {
    failures.push(`list/tree parent device_total mismatch: list=${pItem.statistics.device_total} tree=${parentTotal}`);
  }

  // 5) 清理
  await call('DELETE', `/device/group/${childId}`, null, token);
  await call('DELETE', `/device/group/${parentId}`, null, token);
  for (const deviceId of createdDeviceIds) {
    await call('DELETE', `/device/${deviceId}`, null, token);
  }

  if (failures.length) {
    console.error('\nFAIL:\n - ' + failures.join('\n - '));
    process.exit(1);
  }
  console.log('\nOK: tree 与 list 均带 statistics，且父分组统计正确汇总子孙设备');
})().catch(e => { console.error('FATAL', e); process.exit(1); });
