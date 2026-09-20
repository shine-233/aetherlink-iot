#!/usr/bin/env node
/**
 * 文件用途：TP-4 剩余项取证——MQTT 调试会话里 Topic 映射的订阅/发布交互
 * （roadmap §7.3-1：「Topic 映射的订阅/发布交互未取证（需先开启调试会话，
 * 会真在 broker 上开会话）」）。
 *
 * 演练内容（全部走真实 HTTP API + 真实 broker 连接）：
 *   1. 开启设备 MQTT 调试会话（隔离 debug 客户端真实连接 broker）；
 *   2. 按主题映射白名单订阅下行映射主题（devices/telemetry/control/{number} 等）；
 *   3. 向同一映射主题发布载荷（broker 真实往返）；
 *   4. 快照断言：connected、订阅生效、发布/接收两条消息被会话捕获；
 *   5. 主题策略防线取证：跨设备主题订阅被拒、只读主题（devices/status）发布被拒；
 *   6. 关闭会话。
 *
 * 用法：
 *   node scripts/tp4_mqtt_debug_topic_drill.js --device-id <uuid>
 *   node scripts/tp4_mqtt_debug_topic_drill.js --device-file scratch/p23b_device.json
 *
 * 判定：VERDICT=PASS（回执 JSONL 落 --out 指定文件，缺省 scratch/tp4_mqtt_debug_drill.jsonl）。
 */

const fs = require('fs');
const path = require('path');
const apiClient = require('../lib/api_client');

function parseArgs(argv) {
  const args = { out: path.join('scratch', 'tp4_mqtt_debug_drill.jsonl') };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--device-id') args.deviceId = argv[i + 1];
    else if (argv[i] === '--device-file') args.deviceFile = argv[i + 1];
    else if (argv[i] === '--out') args.out = argv[i + 1];
  }
  return args;
}

const lines = [];
function record(event) {
  lines.push(JSON.stringify({ at: new Date().toISOString(), ...event }));
  console.log(JSON.stringify(event));
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  fs.mkdirSync(path.dirname(args.out), { recursive: true });

  if (!(await apiClient.healthCheck())) {
    throw new Error('backend unhealthy; refusing to fabricate TP-4 evidence');
  }
  await apiClient.login('tenant_admin');

  let deviceId = args.deviceId;
  if (!deviceId && args.deviceFile) {
    deviceId = JSON.parse(fs.readFileSync(args.deviceFile, 'utf8')).id;
  }
  if (!deviceId) {
    console.error('usage: --device-id <uuid> | --device-file <file>');
    process.exit(2);
  }

  // 1. 设备详情 → device_number（主题映射的身份段）。
  const detail = await apiClient.get('/device/detail/' + deviceId, {}, 'tenant_admin');
  if (detail.code !== 200) {
    throw new Error('device detail failed: ' + detail.code + ' ' + (detail.message || ''));
  }
  const row = detail.data || {};
  const deviceNumber = row.device_number || row.deviceNumber || row.pid_number || row.PIDNumber;
  if (!deviceNumber) {
    throw new Error('device_number missing on device detail response');
  }
  record({ event: 'device_resolved', device_id: deviceId, device_number: deviceNumber });

  // 2. 开调试会话（真实 broker 连接）。
  const open = await apiClient.post('/device/' + deviceId + '/mqtt-debug/session', {}, 'tenant_admin');
  if (open.code !== 200) {
    throw new Error('open debug session failed: ' + open.code + ' ' + (open.message || ''));
  }
  const sessionId = open.data.session_id;
  record({ event: 'session_opened', session_id: sessionId, connected: open.data.connected });

  const command = async payload => apiClient.post(
    '/device/' + deviceId + '/mqtt-debug/session/' + sessionId + '/command',
    payload,
    'tenant_admin'
  );
  const snapshot = async () => apiClient.get(
    '/device/' + deviceId + '/mqtt-debug/session/' + sessionId,
    { after_sequence: 0, limit: 50 },
    'tenant_admin'
  );

  try {
    // 3. 订阅两条映射主题（下行控制 + 命令通配）。
    const controlTopic = 'devices/telemetry/control/' + deviceNumber;
    const commandTopic = 'devices/command/' + deviceNumber + '/+';
    const sub1 = await command({ action: 'subscribe', topic: controlTopic, qos: 1 });
    if (sub1.code !== 200) throw new Error('subscribe control topic rejected: ' + sub1.message);
    const sub2 = await command({ action: 'subscribe', topic: commandTopic, qos: 1 });
    if (sub2.code !== 200) throw new Error('subscribe command topic rejected: ' + sub2.message);
    record({ event: 'subscribed', topics: [controlTopic, commandTopic] });

    // 4. 向映射主题发布（broker 真实往返；会话自身是订阅者，应捕获到该消息）。
    const payload = JSON.stringify({ drill: 'tp4', at: new Date().toISOString() });
    const pub = await command({ action: 'publish', topic: controlTopic, qos: 1, payload });
    if (pub.code !== 200) throw new Error('publish to control topic rejected: ' + pub.message);
    record({ event: 'published', topic: controlTopic, payload_bytes: payload.length });

    // 等 broker 往返落进会话环形缓冲。
    let snap = null;
    for (let i = 0; i < 20; i += 1) {
      await new Promise(resolve => setTimeout(resolve, 250));
      const resp = await snapshot();
      if (resp.code !== 200) throw new Error('snapshot failed: ' + resp.message);
      snap = resp.data;
      const received = (snap.messages || []).filter(
        m => m.direction === 'inbound' && m.topic === controlTopic
      );
      if (received.length > 0) break;
    }

    const publishedSeen = (snap.messages || []).some(
      m => m.direction === 'outbound' && m.outcome === 'published' && m.topic === controlTopic
    );
    const receivedSeen = (snap.messages || []).some(
      m => m.direction === 'inbound' && m.topic === controlTopic
    );
    const subsListed = (snap.subscriptions || []);
    record({
      event: 'snapshot',
      connected: snap.connected,
      subscriptions: subsListed,
      message_count: (snap.messages || []).length,
      published_seen: publishedSeen,
      received_seen: receivedSeen
    });

    // 5. 主题策略防线（跨设备订阅 / 只读主题发布都必须被拒）。
    const crossDevice = await command({
      action: 'subscribe',
      topic: 'devices/command/other-device-number/+',
      qos: 1
    });
    const readOnlyPublish = await command({
      action: 'publish',
      topic: 'devices/status/' + deviceNumber,
      qos: 1,
      payload: '{"online":true}'
    });
    record({
      event: 'policy_guards',
      cross_device_subscribe_rejected: crossDevice.code !== 200,
      cross_device_code: crossDevice.code,
      readonly_publish_rejected: readOnlyPublish.code !== 200,
      readonly_publish_code: readOnlyPublish.code
    });

    // 6. 判定。
    const verdictOk = snap.connected
      && subsListed.includes(controlTopic)
      && subsListed.includes(commandTopic)
      && publishedSeen
      && receivedSeen
      && crossDevice.code !== 200
      && readOnlyPublish.code !== 200;
    record({ event: 'verdict', verdict: verdictOk ? 'PASS' : 'FAIL' });
    if (!verdictOk) process.exitCode = 1;
  } finally {
    const close = await apiClient.delete(
      '/device/' + deviceId + '/mqtt-debug/session/' + sessionId, {}, 'tenant_admin'
    );
    record({ event: 'session_closed', code: close.code });
    fs.writeFileSync(args.out, lines.join('\n') + '\n');
  }
}

main().catch(err => {
  console.error('DRILL ERROR:', err.message);
  process.exit(1);
});
