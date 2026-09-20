#!/usr/bin/env node
/**
 * 文件用途：为 MQTT 摄取压测取一份真实设备连接信息（一次性辅助脚本）。
 * 关键注意事项：只读取，不创建/修改任何数据；凭据打印到 stdout 供压测工具使用。
 */
const path = require('path');
process.chdir(path.resolve(__dirname, '..'));
require('../lib/runtime_config');
const apiClient = require('../lib/api_client');

(async () => {
  await apiClient.login('tenant_admin');
  const list = await apiClient.get('/device', { page: 1, page_size: 5 }, 'tenant_admin');
  const rows = (list && list.data && (list.data.list || list.data)) || [];
  console.log('device_count=' + rows.length);
  if (rows.length === 0) {
    console.log('NO_DEVICE');
    return;
  }
  const device = rows[0];
  console.log('device_id=' + device.id);
  console.log('device_number=' + (device.device_number || device.deviceNumber || ''));
  const info = await apiClient.get('/device/connect/info', { device_id: device.id }, 'tenant_admin');
  console.log('connect_info_code=' + info.code);
  console.log('connect_info=' + JSON.stringify(info.data));
})().catch(error => {
  console.error('FAILED: ' + (error && error.message));
  process.exitCode = 1;
});
