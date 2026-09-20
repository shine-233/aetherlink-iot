#!/usr/bin/env node
/**
 * 文件用途：读回某设备的当前遥测，用于确认 MQTT 压测的消息是否真的落库（一次性校验脚本）。
 * 关键注意事项：只读，不写入。
 */
const path = require('path');
process.chdir(path.resolve(__dirname, '..'));
require('../lib/runtime_config');
const apiClient = require('../lib/api_client');

(async () => {
  const deviceId = process.argv[2];
  if (!deviceId) throw new Error('usage: node verify-telemetry-landed.js <device_id>');
  await apiClient.login('tenant_admin');
  const resp = await apiClient.get('/telemetry/datas/current/' + deviceId, {}, 'tenant_admin');
  const rows = (resp && resp.data && (resp.data.list || resp.data)) || [];
  console.log('code=' + resp.code + ' row_count=' + rows.length);
  for (const row of rows.slice(0, 10)) {
    console.log(`  key=${row.key} value=${row.value} ts=${row.ts}`);
  }
})().catch(error => {
  console.error('FAILED: ' + (error && error.message));
  process.exitCode = 1;
});
