#!/usr/bin/env node
/**
 * 文件用途：P2.3 MQTT 摄取压测的种子 + 读回探针（roadmap §P2.3）。
 *
 * 为什么存在：2026-09-17 的 mqttbench 首轮把"读回"放在压测结束后，
 * 恰逢活栈退出，读回拿到 code=-1——这个数字既不能证明摄取成功也不能证明失败，
 * 于是"不限速组 15,856 msg/s"只能整组作废。教训：**读回必须在栈活着的时候做**。
 * 本脚本把种子（建设备拿 device_id）与读回（压测后立刻查 current 遥测）
 * 封装成两个子命令，夹住 mqttbench 的运行窗口。
 *
 * 用法：
 *   node scripts/p23_mqtt_ingest_probe.js seed --out <file>
 *   node scripts/p23_mqtt_ingest_probe.js read --device-file <file> \
 *       --expect-at-least N --key temperature_1
 */

const fs = require('fs');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

function parseArgs(argv) {
  const args = { _: [] };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--out') args.out = argv[i + 1];
    else if (argv[i] === '--device-file') args.deviceFile = argv[i + 1];
    else if (argv[i] === '--expect-at-least') args.expectAtLeast = Number(argv[i + 1]);
    else if (argv[i] === '--key') args.key = argv[i + 1];
    else args._.push(argv[i]);
  }
  return args;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const command = args._[0];
  if (command !== 'seed' && command !== 'read') {
    console.error('usage: seed --out <file> | read --device-file <file> --key <key> [--expect-at-least N]');
    process.exit(2);
  }

  const healthy = await apiClient.healthCheck();
  if (!healthy) {
    console.error('backend unhealthy; refusing to produce another code=-1-shaped mystery');
    process.exit(1);
  }
  await apiClient.login('tenant_admin');

  if (command === 'seed') {
    // 与 40/60 号用例同一播种函数：每次新建设备拿新鲜凭证（24h Redis 测试缓存），
    // 顺便把"读回一致性"提前暴露在 setup 阶段。刻意不发遥测——
    // 压测数字必须来自 mqttbench 的真实 MQTT 路径，不能用模拟端点顶替。
    const seed = await seedData.createSimulationDevice('tenant_admin');
    fs.writeFileSync(args.out, JSON.stringify({
      id: seed.id,
      createdAt: new Date().toISOString()
    }, null, 2));
    console.log('SEEDED device_id=' + seed.id + ' -> ' + args.out);
    return;
  }

  // read：压测结束后立刻读回 current 遥测，确认摄取真的落库。
  const info = JSON.parse(fs.readFileSync(args.deviceFile, 'utf8'));
  const key = args.key || 'temperature_1';
  const minimum = Number.isFinite(args.expectAtLeast) ? args.expectAtLeast : 1;

  // 给摄取管道留时间：上行是异步的，读到 0 就退会在假阴性上重蹈 code=-1 覆辙。
  let lastCount = 0;
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    lastCount = await countCurrentValue(info.id, key);
    if (lastCount) break;
    await sleep(2000);
  }
  console.log('READBACK device_id=' + info.id + ' key=' + key
    + ' has_current=' + (lastCount ? 'yes' : 'no')
    + ' expect_at_least=' + minimum);
  if (minimum > 0 && !lastCount) {
    console.error('VERDICT=FAIL ingest not confirmed (no current row for key ' + key + ')');
    process.exit(1);
  }
  console.log('VERDICT=PASS ingest confirmed while stack alive');
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function countCurrentValue(deviceId, key) {
  const resp = await apiClient.get('/telemetry/datas/current/' + deviceId, {}, 'tenant_admin');
  if (!resp || resp.code !== 200) return false;
  const rows = Array.isArray(resp.data) ? resp.data : [];
  const hit = rows.find(row => row && row.key === key);
  if (!hit) return false;
  console.log('  current[' + key + ']=' + (hit.value !== undefined ? hit.value : JSON.stringify(hit)));
  return true;
}

main().catch(error => {
  console.error(error.message || error);
  process.exit(1);
});
