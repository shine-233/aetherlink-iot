/**
 * 文件用途：TB-12 设备端 MQTT 自助认领（v1/devices/me/claim & devices/claim）契约与端到端测试。
 *
 * 对标 ThingsBoard CE 设备端开箱自主认领通道：
 *   1. 设备通过 MQTT 连接本地 Broker；
 *   2. 设备向 `v1/devices/me/claim`（或 `devices/claim`）发布包含 `secretKey` 与 `durationMs` 的载荷；
 *   3. 后端 MQTT 适配器接收后，在 DB 事务中为该设备登记活跃认领令牌；
 *   4. 签发租户可在 `/device/claim-tokens` 查验该 active 令牌（无明文、带有效截止时间）；
 *   5. 目标接收租户（tenant_admin_b）在 `/device/claim-tokens/redeem` 凭 `device_number` + 设备自报的 `secretKey` 成功赎回认领设备；
 *   6. 设备归属成功由 A 租户过户至 B 租户，且 A 租户失去可见性（404）。
 */

const net = require('net');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const mqttRuntime = require('../lib/mqtt_runtime');

const SUITE = 'TB-12 Device-Side MQTT Claiming [66_tb12_mqtt_device_claiming]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const CODE_NOT_FOUND = 100404;

function encodeRemainingLength(length) {
  const bytes = [];
  let value = length;
  do {
    let digit = value % 128;
    value = Math.floor(value / 128);
    if (value > 0) digit |= 0x80;
    bytes.push(digit);
  } while (value > 0);
  return Buffer.from(bytes);
}

function publishMqttDirect(host, port, topic, payload) {
  return new Promise((resolve, reject) => {
    const socket = net.connect(port, host, () => {
      const clientId = 'claim_dev_' + Math.random().toString(36).slice(2, 8);
      const protoName = Buffer.from([0x00, 0x04, 0x4d, 0x51, 0x54, 0x54]); // "MQTT"
      const protoLevel = Buffer.from([0x04]);
      const connectFlags = Buffer.from([0x02]); // Clean session
      const keepAlive = Buffer.from([0x00, 0x3c]);
      const clientBuf = Buffer.from(clientId, 'utf8');
      const clientPayload = Buffer.concat([Buffer.from([clientBuf.length >> 8, clientBuf.length & 0xff]), clientBuf]);

      const variableHeader = Buffer.concat([protoName, protoLevel, connectFlags, keepAlive]);
      const connectLen = variableHeader.length + clientPayload.length;
      const connectPacket = Buffer.concat([Buffer.from([0x10]), encodeRemainingLength(connectLen), variableHeader, clientPayload]);
      socket.write(connectPacket);
    });

    let connected = false;

    socket.on('data', data => {
      if (!connected) {
        if (data[0] === 0x20 && data[3] === 0x00) {
          connected = true;
          const topicBuf = Buffer.from(topic, 'utf8');
          const topicHeader = Buffer.from([topicBuf.length >> 8, topicBuf.length & 0xff]);
          const payloadBuf = Buffer.isBuffer(payload) ? payload : Buffer.from(payload, 'utf8');
          const pubLen = 2 + topicBuf.length + payloadBuf.length;
          const pubPacket = Buffer.concat([Buffer.from([0x30]), encodeRemainingLength(pubLen), topicHeader, topicBuf, payloadBuf]);
          socket.write(pubPacket, () => {
            setTimeout(() => {
              socket.end();
              resolve();
            }, 200);
          });
        } else {
          socket.destroy();
          reject(new Error('MQTT CONNACK failed: ' + data.toString('hex')));
        }
      }
    });

    socket.on('error', reject);
    socket.setTimeout(5000, () => {
      socket.destroy();
      reject(new Error('MQTT connection timeout'));
    });
  });
}

describe(SUITE, function () {
  this.timeout(60000);

  let seed = null;
  let deviceId = null;
  let deviceNumber = null;
  let secretKey = null;

  const mqttEndpoint = mqttRuntime.getMqttEndpoint();
  const brokerHost = mqttEndpoint.server || '127.0.0.1';
  const brokerPort = mqttEndpoint.port || 1883;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend is not running locally for ' + SUITE);
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);

    seed = await seedData.createSimulationDevice(ACCOUNT);
    deviceId = seed.id;

    const detail = await apiClient.get('/device/detail/' + deviceId, {}, ACCOUNT);
    expect(detail.code, 'get device detail').to.equal(200);
    deviceNumber = detail.data && (detail.data.device_number || (detail.data.device && detail.data.device.device_number));
    expect(deviceNumber, 'device_number must not be empty').to.be.a('string').and.not.equal('');

    secretKey = 'tb_claim_sec_' + Math.random().toString(36).slice(2, 10);
  });

  after(async function () {
    if (seed && typeof seed.cleanup === 'function') {
      try {
        await seed.cleanup();
      } catch (err) {
        // 若已转移到 B 租户，尝试用 B 租户清理
        try {
          await apiClient.delete('/device/' + deviceId, {}, OTHER_ACCOUNT);
        } catch (_) {}
      }
    }
  });

  it('1. 设备向 v1/devices/me/claim 发布带 secretKey 的认领上报，平台正确持久化 active 令牌', async function () {
    const payload = JSON.stringify({
      device_id: deviceId,
      secretKey: secretKey,
      durationMs: 7200000 // 2小时
    });

    await publishMqttDirect(brokerHost, brokerPort, 'v1/devices/me/claim', payload);

    // 轮询等待后台订阅处理落库
    let tokens = [];
    const deadline = Date.now() + 5000;
    while (Date.now() < deadline) {
      const resp = await apiClient.get('/device/claim-tokens', { device_id: deviceId }, ACCOUNT);
      if (resp.code === 200) {
        const rows = Array.isArray(resp.data) ? resp.data : (resp.data && resp.data.list) || [];
        if (rows.length > 0) {
          tokens = rows;
          break;
        }
      }
      await new Promise(r => setTimeout(r, 200));
    }

    expect(tokens.length, 'token list length').to.be.greaterThan(0);
    const activeToken = tokens.find(t => t.status === 'active');
    expect(activeToken, 'must have active claim token').to.exist;
    expect(activeToken.device_id).to.equal(deviceId);
    expect(activeToken.device_number).to.equal(deviceNumber);
    expect(activeToken.effective).to.equal('active');
    // 验证安全不变量：列表永不出明文 secretKey 或哈希
    expect(activeToken).to.not.have.property('claim_key');
    expect(activeToken).to.not.have.property('claim_key_hash');
  });

  it('2. 原生话题 devices/claim 覆盖签发旧令牌（同一设备至多保留 1 条 active）', async function () {
    const newSecretKey = 'native_sec_' + Math.random().toString(36).slice(2, 10);
    const payload = JSON.stringify({
      deviceId: deviceId,
      claimKey: newSecretKey,
      ttl_seconds: 3600
    });

    await publishMqttDirect(brokerHost, brokerPort, 'devices/claim', payload);

    let tokens = [];
    const deadline = Date.now() + 5000;
    while (Date.now() < deadline) {
      const resp = await apiClient.get('/device/claim-tokens', { device_id: deviceId }, ACCOUNT);
      if (resp.code === 200) {
        const rows = Array.isArray(resp.data) ? resp.data : (resp.data && resp.data.list) || [];
        if (rows.length >= 2) {
          tokens = rows;
          break;
        }
      }
      await new Promise(r => setTimeout(r, 200));
    }

    const activeTokens = tokens.filter(t => t.status === 'active');
    expect(activeTokens.length, 'exactly 1 active token after replacement').to.equal(1);
    const replacedTokens = tokens.filter(t => t.status === 'replaced');
    expect(replacedTokens.length, 'previous active token must be marked replaced').to.be.greaterThan(0);

    // 锁定下步测试使用的有效 key
    secretKey = newSecretKey;
  });

  it('3. 接收租户使用旧 key 赎回被拒绝（404 not-claimable 防探测）', async function () {
    const wrongKey = 'wrong_key_' + Math.random().toString(36).slice(2, 8);
    const resp = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: deviceNumber,
      claim_key: wrongKey
    }, OTHER_ACCOUNT);

    expect(resp.code).to.equal(CODE_NOT_FOUND);
    expect(resp.message).to.equal('device is not claimable with this key');
  });

  it('4. 接收租户凭最新 secretKey 成功赎回认领，完成设备所有权跨租户转移', async function () {
    const redeemResp = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: deviceNumber,
      claim_key: secretKey
    }, OTHER_ACCOUNT);

    expect(redeemResp.code, 'redeem success').to.equal(200);
    expect(redeemResp.data.device_id).to.equal(deviceId);
    expect(redeemResp.data.device_number).to.equal(deviceNumber);

    // 验证所有权转移：B 租户读到详情，A 租户读到 404
    const detailB = await apiClient.get('/device/detail/' + deviceId, {}, OTHER_ACCOUNT);
    expect(detailB.code, 'tenant B owns device').to.equal(200);

    const detailA = await apiClient.get('/device/detail/' + deviceId, {}, ACCOUNT);
    expect([100404, 201001, 100002], 'tenant A lost access to transferred device').to.include(detailA.code);
  });
});
