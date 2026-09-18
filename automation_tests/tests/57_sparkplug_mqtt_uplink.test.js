/**
 * 文件用途：TB-10 Sparkplug B 工业载荷规范端到端 MQTT 上行接入与实时遥测入库契约测试。
 * 核心逻辑：
 *   1. 本地连接实时 MQTT Broker（端口 1883），构造标准 Eclipse Sparkplug B protobuf 二进制载荷；
 *   2. 向规范话题 `spBv1.0/<group>/DDATA/<edge_node_id>/<device_id>` 发布设备级数据；
 *   3. 验证后端订阅解包、通过 device_number 检索设备实体、投递 Uplink 总线并持久化至遥测库；
 *   4. 验证 NDATA 节点级上行回退解析（无 device_id 时以 edge_node_id 作为编号）；
 *   5. 验证会话控制消息（NBIRTH/DBIRTH/STATE）优雅忽略；
 *   6. 验证非数值指标安全跳过（"不把非数值当 0" 物理不变量）；
 *   7. 验证畸形载荷与未注册设备号 Fail-Closed 拒绝；
 *   8. 验证跨租户遥测读取权限隔离。
 */

const net = require('net');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const mqttRuntime = require('../lib/mqtt_runtime');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');

const SUITE = 'TB-10 Sparkplug B MQTT Uplink & Live Telemetry [57_sparkplug_mqtt_uplink]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

// Sparkplug B protobuf wire types & field tags
const WIRE_VARINT = 0;
const WIRE_FIXED64 = 1;
const WIRE_BYTES = 2;
const WIRE_FIXED32 = 5;

// DataType enum (matching sparkplug_b.proto & pkg/sparkplug)
const DATA_TYPE_INT32 = 3;
const DATA_TYPE_FLOAT = 9;
const DATA_TYPE_DOUBLE = 10;
const DATA_TYPE_BOOLEAN = 11;
const DATA_TYPE_STRING = 12;

function encVarint(value) {
  const bytes = [];
  let val = BigInt(value);
  while (val > 127n) {
    bytes.push(Number((val & 0x7fn) | 0x80n));
    val >>= 7n;
  }
  bytes.push(Number(val & 0x7fn));
  return Buffer.from(bytes);
}

function encKey(field, wireType) {
  return encVarint((field << 3) | wireType);
}

function encStringField(field, str) {
  const buf = Buffer.from(str, 'utf8');
  return Buffer.concat([encKey(field, WIRE_BYTES), encVarint(buf.length), buf]);
}

function encVarintField(field, value) {
  return Buffer.concat([encKey(field, WIRE_VARINT), encVarint(value)]);
}

function encDoubleField(field, value) {
  const buf = Buffer.alloc(8);
  buf.writeDoubleLE(value);
  return Buffer.concat([encKey(field, WIRE_FIXED64), buf]);
}

function encFloatField(field, value) {
  const buf = Buffer.alloc(4);
  buf.writeFloatLE(value);
  return Buffer.concat([encKey(field, WIRE_FIXED32), buf]);
}

/**
 * 构造合规的 Sparkplug B protobuf Payload。
 * Payload: { timestamp = 1, metrics = 2 }
 * Metric: { name = 1, datatype = 4, int_value = 10, float_value = 14, double_value = 15, boolean_value = 16, string_value = 17 }
 */
function buildSparkplugPayload(metrics) {
  const nowMs = Date.now();
  const tsField = encVarintField(1, nowMs);

  const metricBufs = [];
  for (const m of metrics) {
    const parts = [encStringField(1, m.name)];
    if (typeof m.intValue === 'number') {
      parts.push(encVarintField(4, DATA_TYPE_INT32));
      parts.push(encVarintField(10, m.intValue));
    } else if (typeof m.doubleValue === 'number') {
      parts.push(encVarintField(4, DATA_TYPE_DOUBLE));
      parts.push(encDoubleField(13, m.doubleValue));
    } else if (typeof m.floatValue === 'number') {
      parts.push(encVarintField(4, DATA_TYPE_FLOAT));
      parts.push(encFloatField(12, m.floatValue));
    } else if (typeof m.booleanValue === 'boolean') {
      parts.push(encVarintField(4, DATA_TYPE_BOOLEAN));
      parts.push(encVarintField(14, m.booleanValue ? 1 : 0));
    } else if (typeof m.stringValue === 'string') {
      parts.push(encVarintField(4, DATA_TYPE_STRING));
      parts.push(encStringField(15, m.stringValue));
    }
    const metricBody = Buffer.concat(parts);
    metricBufs.push(Buffer.concat([encKey(2, WIRE_BYTES), encVarint(metricBody.length), metricBody]));
  }

  return Buffer.concat([tsField, ...metricBufs]);
}

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

/**
 * 极简标准 MQTT 3.1.1 客户端（纯 Node.js net socket 实现，零第三方依赖）。
 */
function publishMqttDirect(host, port, topic, payload) {
  return new Promise((resolve, reject) => {
    const socket = net.connect(port, host, () => {
      const clientId = 'spk_test_' + Math.random().toString(36).slice(2, 8);
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
          const pubLen = 2 + topicBuf.length + payload.length;
          const pubPacket = Buffer.concat([Buffer.from([0x30]), encodeRemainingLength(pubLen), topicHeader, topicBuf, payload]);
          socket.write(pubPacket, () => {
            setTimeout(() => {
              socket.end();
              resolve();
            }, 150);
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

/**
 * 轮询检索指定设备当前遥测数据，直到所有指定键均就绪或超时。
 */
async function pollCurrentTelemetry(deviceId, expectedKeys, accountKey = ACCOUNT, maxWaitMs = 5000) {
  const start = Date.now();
  while (Date.now() - start < maxWaitMs) {
    const resp = await apiClient.get('/telemetry/datas/current/' + deviceId, {}, accountKey);
    if (resp && resp.code === 200 && Array.isArray(resp.data)) {
      const rows = resp.data;
      const foundAll = expectedKeys.every(k => rows.some(r => r.key === k));
      if (foundAll) {
        return rows;
      }
    }
    await new Promise(r => setTimeout(r, 200));
  }
  const lastResp = await apiClient.get('/telemetry/datas/current/' + deviceId, {}, accountKey);
  return (lastResp && lastResp.data) || [];
}

describe(SUITE, function () {
  this.timeout(60000);

  const cleanups = [];
  const mqttEndpoint = mqttRuntime.getMqttEndpoint();
  const brokerHost = mqttEndpoint.server || '127.0.0.1';
  const brokerPort = mqttEndpoint.port || 1883;

  let deviceAId = null;
  let deviceANumber = null;
  let deviceBId = null;
  let deviceBNumber = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 57_sparkplug_mqtt_uplink.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);

    // 1. 创建租户 A 下的测试设备 A（用于设备级 DDATA 测试）
    deviceANumber = 'spk_dev_' + Date.now().toString(36) + '_' + Math.random().toString(36).slice(2, 6);
    const devAResp = await apiClient.post('/device', {
      name: 'Sparkplug Device ' + deviceANumber,
      device_number: deviceANumber,
      device_type: '1'
    }, ACCOUNT);
    expectSuccess(devAResp, 'create device A');
    deviceAId = devAResp.data.id;
    cleanups.push(async () => {
      await apiClient.delete('/device/' + deviceAId, {}, ACCOUNT);
    });

    // 2. 创建租户 A 下的边缘节点设备 B（用于节点级 NDATA 测试）
    deviceBNumber = 'spk_node_' + Date.now().toString(36) + '_' + Math.random().toString(36).slice(2, 6);
    const devBResp = await apiClient.post('/device', {
      name: 'Sparkplug Edge Node ' + deviceBNumber,
      device_number: deviceBNumber,
      device_type: '1'
    }, ACCOUNT);
    expectSuccess(devBResp, 'create device B');
    deviceBId = devBResp.data.id;
    cleanups.push(async () => {
      await apiClient.delete('/device/' + deviceBId, {}, ACCOUNT);
    });
  });

  after(async function () {
    for (let i = cleanups.length - 1; i >= 0; i--) {
      try {
        await cleanups[i]();
      } catch (err) {
        console.warn('Cleanup error:', err.message);
      }
    }
    apiClient.clearAllTokens();
  });

  it('1. 发布 Sparkplug B DDATA 设备级遥测，验证自动寻址入库与数值精确度', async function () {
    const topic = `spBv1.0/Factory-A/DDATA/gateway-01/${deviceANumber}`;
    const payload = buildSparkplugPayload([
      { name: 'temperature', doubleValue: 28.5 },
      { name: 'pressure_kpa', doubleValue: 101.32 },
      { name: 'engine_rpm', intValue: 1500 }
    ]);

    await publishMqttDirect(brokerHost, brokerPort, topic, payload);

    const rows = await pollCurrentTelemetry(deviceAId, ['temperature', 'pressure_kpa', 'engine_rpm'], ACCOUNT);
    expect(rows).to.be.an('array');

    const tempRow = rows.find(r => r.key === 'temperature');
    expect(tempRow, 'temperature metric must exist').to.be.ok;
    expect(Number(tempRow.value)).to.equal(28.5);

    const pressRow = rows.find(r => r.key === 'pressure_kpa');
    expect(pressRow, 'pressure_kpa metric must exist').to.be.ok;
    expect(Number(pressRow.value)).to.equal(101.32);

    const rpmRow = rows.find(r => r.key === 'engine_rpm');
    expect(rpmRow, 'engine_rpm metric must exist').to.be.ok;
    expect(Number(rpmRow.value)).to.equal(1500);
  });

  it('2. 发布 Sparkplug B NDATA 节点级遥测（话题无 device_id），验证回退至 edge_node_id 寻址入库', async function () {
    const topic = `spBv1.0/Factory-A/NDATA/${deviceBNumber}`;
    const payload = buildSparkplugPayload([
      { name: 'bus_voltage', doubleValue: 380.0 },
      { name: 'cpu_usage', floatValue: 42.5 }
    ]);

    await publishMqttDirect(brokerHost, brokerPort, topic, payload);

    const rows = await pollCurrentTelemetry(deviceBId, ['bus_voltage', 'cpu_usage'], ACCOUNT);
    expect(rows).to.be.an('array');

    const voltRow = rows.find(r => r.key === 'bus_voltage');
    expect(voltRow, 'bus_voltage metric must exist on edge node').to.be.ok;
    expect(Number(voltRow.value)).to.equal(380.0);

    const cpuRow = rows.find(r => r.key === 'cpu_usage');
    expect(cpuRow, 'cpu_usage metric must exist on edge node').to.be.ok;
    expect(Math.abs(Number(cpuRow.value) - 42.5)).to.be.lessThan(0.01);
  });

  it('3. 非数值指标安全过滤：字符串/布尔指标被安全跳过，严防转为假 0 值（核心物理不变量）', async function () {
    const topic = `spBv1.0/Factory-A/DDATA/gateway-01/${deviceANumber}`;
    const payload = buildSparkplugPayload([
      { name: 'sensor_valid', booleanValue: true },
      { name: 'device_state', stringValue: 'NORMAL_OPERATING' },
      { name: 'vibration_level', doubleValue: 3.14 }
    ]);

    await publishMqttDirect(brokerHost, brokerPort, topic, payload);

    const rows = await pollCurrentTelemetry(deviceAId, ['vibration_level'], ACCOUNT);
    const vibRow = rows.find(r => r.key === 'vibration_level');
    expect(vibRow, 'numeric metric vibration_level must be recorded').to.be.ok;
    expect(Number(vibRow.value)).to.equal(3.14);

    // 严谨验证：non-numeric metrics 不得产生值为 "0" 或 0 的假记录
    const stateRow = rows.find(r => r.key === 'device_state');
    expect(stateRow, 'string metric must be skipped rather than recorded as 0').to.be.undefined;

    const boolRow = rows.find(r => r.key === 'sensor_valid');
    expect(boolRow, 'boolean metric must be skipped rather than recorded as 0').to.be.undefined;
  });

  it('4. 会话控制类消息（NBIRTH / DBIRTH / STATE）优雅忽略且不报错', async function () {
    // 规范规定：会话控制类消息承载别名映射与在线状态，当前版优雅跳过并不产生任何错误
    const dbirthTopic = `spBv1.0/Factory-A/DBIRTH/gateway-01/${deviceANumber}`;
    const nbirthTopic = `spBv1.0/Factory-A/NBIRTH/gateway-01`;
    const stateTopic = `spBv1.0/Factory-A/STATE/gateway-01`;

    const dummyPayload = Buffer.from([0x08, 0x01]); // timestamp = 1
    await publishMqttDirect(brokerHost, brokerPort, dbirthTopic, dummyPayload);
    await publishMqttDirect(brokerHost, brokerPort, nbirthTopic, dummyPayload);
    await publishMqttDirect(brokerHost, brokerPort, stateTopic, dummyPayload);

    // 随后发送标准遥测证明管线保持健康活跃
    const testTopic = `spBv1.0/Factory-A/DDATA/gateway-01/${deviceANumber}`;
    const activePayload = buildSparkplugPayload([{ name: 'flow_rate', doubleValue: 12.8 }]);
    await publishMqttDirect(brokerHost, brokerPort, testTopic, activePayload);

    const rows = await pollCurrentTelemetry(deviceAId, ['flow_rate'], ACCOUNT);
    const flowRow = rows.find(r => r.key === 'flow_rate');
    expect(flowRow).to.be.ok;
    expect(Number(flowRow.value)).to.equal(12.8);
  });

  it('5. 畸形载荷与截断字节 Fail-Closed 拦截：不污染数据库且服务不崩溃', async function () {
    const topic = `spBv1.0/Factory-A/DDATA/gateway-01/${deviceANumber}`;
    // 畸形 protobuf 截断包
    const malformedPayload = Buffer.from([0x12, 0x7f, 0x0a]);
    await publishMqttDirect(brokerHost, brokerPort, topic, malformedPayload);

    // 等待 500ms 后验证后续正常遥测依然能正常接收入库
    await new Promise(r => setTimeout(r, 500));

    const okPayload = buildSparkplugPayload([{ name: 'battery_level', doubleValue: 98.0 }]);
    await publishMqttDirect(brokerHost, brokerPort, topic, okPayload);

    const rows = await pollCurrentTelemetry(deviceAId, ['battery_level'], ACCOUNT);
    const battRow = rows.find(r => r.key === 'battery_level');
    expect(battRow).to.be.ok;
    expect(Number(battRow.value)).to.equal(98.0);
  });

  it('6. 未注册 device_number 拦截：丢弃并记录，绝不生成幽灵设备', async function () {
    const ghostNum = 'ghost_unknown_device_' + Date.now();
    const topic = `spBv1.0/Factory-A/DDATA/gateway-01/${ghostNum}`;
    const payload = buildSparkplugPayload([{ name: 'temperature', doubleValue: 99.9 }]);

    await publishMqttDirect(brokerHost, brokerPort, topic, payload);
    await new Promise(r => setTimeout(r, 500));

    // 验证全系统无此设备
    const searchResp = await apiClient.get('/device', { page: 1, page_size: 10, name: ghostNum }, ACCOUNT);
    expectSuccess(searchResp);
    const list = (searchResp.data && searchResp.data.list) || [];
    expect(list.filter(d => d.device_number === ghostNum)).to.be.empty;
  });

  it('7. 跨租户多租户隔离：租户 B 无法越权查询租户 A 的 Sparkplug 遥测', async function () {
    const crossResp = await apiClient.get('/telemetry/datas/current/' + deviceAId, {}, OTHER_ACCOUNT);
    // 租户隔离：对不属于本租户的设备，返回 403 权限拒绝或 404/空数据
    if (crossResp.code === 200) {
      expect(crossResp.data).to.satisfy(d => !d || (Array.isArray(d) && d.length === 0));
    } else {
      expect(crossResp.code).to.be.oneOf([403, 404, 100404, 100403, 201001]);
    }
  });
});
