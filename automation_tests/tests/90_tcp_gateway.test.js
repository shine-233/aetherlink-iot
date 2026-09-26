/**
 * 文件用途：TCP 协议接入网关（TP-03，backend/internal/protocolgw/tcp）活栈契约测试——配置面守护。
 *
 * 覆盖：
 *   1. 默认关闭面（任何栈都跑）：protocols.tcp.enabled 未开启时网关不监听端口
 *      （ROADMAP TP-03 验收点：不占用平台默认端口）；若探测意外连通，判定为
 *      栈配置与测试环境变量不一致并给出可诊断报错；
 *   2. 启用路径（仅当 AETHERLINK_TCP_GATEWAY_ENABLED=1，即栈显式开启 protocols.tcp.enabled）：
 *      - 创建带 device_number 的设备 → TCP 首帧注册（4 字节大端 length-prefix）→
 *        遥测帧上行 → GET /telemetry/datas/current/:id 轮询到该键（uplink 总线闭环）；
 *      - 未知 device_number 注册 → 服务端断连（fail-closed 凭证映射边界）；
 *      - 非法注册帧（含空白的设备号）→ 服务端断连（注册解析 fail-closed）；
 *      - 离线命令缓冲：无会话时 POST /command/datas/pub → 重连注册 → 收到命令帧
 *        （断网缓冲环形缓冲 + 注册续传语义）；
 *   3. 租户隔离：租户 B 无法读取租户 A 设备的当前遥测。
 *
 * 环境变量（与 lib/network_runtime.js 的 loopback 默认哲学一致）：
 *   - AETHERLINK_TCP_GATEWAY_ENABLED  置 "1" 表示栈已开启 protocols.tcp.enabled
 *   - AETHERLINK_TCP_GATEWAY_HOST     默认 127.0.0.1
 *   - AETHERLINK_TCP_GATEWAY_PORT     默认 9877（backend/internal/protocolgw/tcp DefaultTCPPort）
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const net = require('net');
const apiClient = require('../lib/api_client');

const SUITE = 'TCP Gateway [90_tcp_gateway]';
const TCP_ENABLED = String(process.env.AETHERLINK_TCP_GATEWAY_ENABLED || '') === '1';
const TCP_HOST = String(process.env.AETHERLINK_TCP_GATEWAY_HOST || '127.0.0.1');
const TCP_PORT = parseInt(String(process.env.AETHERLINK_TCP_GATEWAY_PORT || '9877'), 10);
const SOCKET_TIMEOUT_MS = 8000;
const POLL_TIMEOUT_MS = 30000;
const POLL_INTERVAL_MS = 500;

const SUFFIX = Date.now().toString(36) + Math.floor(Math.random() * 100000).toString(36);
const DEVICE_NUMBER = 'TP03_TCP_' + SUFFIX;
const TELEMETRY_KEY = 'tcp_contract_temp';
const COMMAND_IDENTIFY = 'tcp_contract_switch';
let tenantADeviceId = null;

/** length-prefix 帧：4 字节大端长度 + payload（与 Go 侧 EncodeFrame 对齐）。 */
function lpFrame(payload) {
  const body = Buffer.from(payload, 'utf8');
  const frame = Buffer.alloc(4 + body.length);
  frame.writeUInt32BE(body.length, 0);
  body.copy(frame, 4);
  return frame;
}

/** 探测端口是否可连通（默认关闭面用）；布尔结果，不抛错。 */
function probeConnect(host, port) {
  return new Promise((resolve) => {
    const socket = net.connect({ host, port });
    let settled = false;
    const done = (result) => {
      if (settled) return;
      settled = true;
      socket.destroy();
      resolve(result);
    };
    socket.setTimeout(SOCKET_TIMEOUT_MS, () => done(false));
    socket.once('connect', () => done(true));
    socket.once('error', () => done(false));
  });
}

/** 建立 TCP 连接（启用路径用）；失败直接抛错让用例红。 */
function connect(host, port) {
  return new Promise((resolve, reject) => {
    const socket = net.connect({ host, port });
    socket.setTimeout(SOCKET_TIMEOUT_MS, () => {
      socket.destroy();
      reject(new Error('tcp gateway connect timeout ' + host + ':' + port));
    });
    socket.once('connect', () => {
      socket.setTimeout(0);
      resolve(socket);
    });
    socket.once('error', (err) => reject(err));
  });
}

/** 从 socket 读一个 length-prefix 帧的 payload；超时/对端关闭返回 null。 */
function readFrame(socket) {
  return new Promise((resolve) => {
    const chunks = [];
    let received = 0;
    let expected = null;
    const finish = (value) => {
      socket.removeListener('data', onData);
      socket.removeListener('error', onError);
      socket.removeListener('close', onClose);
      clearTimeout(timer);
      resolve(value);
    };
    const onData = (chunk) => {
      chunks.push(chunk);
      received += chunk.length;
      if (expected === null && received >= 4) {
        expected = 4 + Buffer.concat(chunks).readUInt32BE(0);
      }
      if (expected !== null && received >= expected) {
        finish(Buffer.concat(chunks).subarray(4, expected));
      }
    };
    const onError = () => finish(null);
    const onClose = () => finish(null);
    const timer = setTimeout(() => finish(null), SOCKET_TIMEOUT_MS);
    socket.on('data', onData);
    socket.once('error', onError);
    socket.once('close', onClose);
  });
}

/** 等待服务端断开连接；超时未断开返回 false。 */
function waitClosed(socket) {
  return new Promise((resolve) => {
    const finish = (value) => {
      socket.removeListener('error', onError);
      socket.removeListener('close', onClose);
      clearTimeout(timer);
      resolve(value);
    };
    const onError = () => finish(true);
    const onClose = () => finish(true);
    const timer = setTimeout(() => finish(false), SOCKET_TIMEOUT_MS);
    socket.once('error', onError);
    socket.once('close', onClose);
  });
}

/** 建连 + 发注册帧（length-prefix 模式）。 */
async function dialAndRegister(deviceNumber) {
  const socket = await connect(TCP_HOST, TCP_PORT);
  socket.write(lpFrame(deviceNumber));
  return socket;
}

/** 轮询当前遥测直到谓词成立（返回最后一份响应数据）。 */
async function pollCurrentTelemetry(accountKey, predicate) {
  const deadline = Date.now() + POLL_TIMEOUT_MS;
  let lastData = null;
  while (Date.now() < deadline) {
    try {
      const res = await apiClient.get('/telemetry/datas/current/' + tenantADeviceId, {}, accountKey);
      if (res && res.code === 200) {
        lastData = res.data;
        if (predicate(res.data)) return lastData;
      }
    } catch (e) {
      /* 轮询期内的瞬时错误一律忽略，等下一轮 */
    }
    await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
  }
  return lastData;
}

async function createTcpDevice(accountKey) {
  const res = await apiClient.post(
    '/device',
    {
      name: 'TP03_TCP_契约_' + SUFFIX,
      device_number: DEVICE_NUMBER,
      device_config_id: '',
      voucher: JSON.stringify({ username: DEVICE_NUMBER, password: 'tp03-contract' })
    },
    accountKey
  );
  expect(res.code, JSON.stringify(res)).to.equal(200);
  expect(res.data).to.be.an('object');
  expect(res.data.id).to.be.a('string').and.not.equal('');
  expect(String(res.data.device_number || DEVICE_NUMBER)).to.equal(DEVICE_NUMBER);
  return res.data.id;
}

describe(SUITE, function () {
  this.timeout(120000);

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 90_tcp_gateway.test.js');
    }
    await apiClient.login('tenant_admin');
    await apiClient.login('tenant_admin_b');
  });

  after(async function () {
    if (tenantADeviceId) {
      try {
        await apiClient.delete('/device/' + tenantADeviceId, {}, 'tenant_admin');
      } catch (e) {
        /* 清理失败不影响结论 */
      }
    }
    apiClient.clearAllTokens();
  });

  it('1. 默认面：protocols.tcp.enabled 未开启时网关不监听端口', async function () {
    if (TCP_ENABLED) {
      this.skip('stack declares AETHERLINK_TCP_GATEWAY_ENABLED=1; default-off probe not applicable');
    }
    const occupied = await probeConnect(TCP_HOST, TCP_PORT);
    expect(occupied, 'port ' + TCP_HOST + ':' + TCP_PORT + ' is listening while ' +
      'AETHERLINK_TCP_GATEWAY_ENABLED is unset. Either protocols.tcp.enabled was turned on ' +
      'without exporting AETHERLINK_TCP_GATEWAY_ENABLED=1 for this run, or another process ' +
      'occupies the port.').to.equal(false);
  });

  it('2. 启用路径：创建带 device_number 的设备', async function () {
    if (!TCP_ENABLED) {
      this.skip('TCP gateway not enabled in this stack (set AETHERLINK_TCP_GATEWAY_ENABLED=1 with protocols.tcp.enabled)');
    }
    tenantADeviceId = await createTcpDevice('tenant_admin');
  });

  it('3. 启用路径：首帧 device_number 注册后遥测上行落库', async function () {
    if (!TCP_ENABLED) this.skip('TCP gateway not enabled');
    const socket = await dialAndRegister(DEVICE_NUMBER);
    try {
      socket.write(lpFrame(JSON.stringify({ [TELEMETRY_KEY]: 42.5 })));
      const data = await pollCurrentTelemetry('tenant_admin', (rows) =>
        JSON.stringify(rows || null).indexOf(TELEMETRY_KEY) >= 0
      );
      expect(JSON.stringify(data || null), 'telemetry from TCP frame should reach current storage').to.include(TELEMETRY_KEY);
    } finally {
      socket.destroy();
    }
  });

  it('4. 启用路径：未知 device_number 注册被服务端断连（fail-closed）', async function () {
    if (!TCP_ENABLED) this.skip('TCP gateway not enabled');
    const socket = await dialAndRegister('TP03_NO_SUCH_DEVICE_' + SUFFIX);
    const closed = await waitClosed(socket);
    socket.destroy();
    expect(closed, 'unknown device_number registration must be closed by the gateway').to.equal(true);
  });

  it('5. 启用路径：非法注册帧（设备号含空白）被服务端断连', async function () {
    if (!TCP_ENABLED) this.skip('TCP gateway not enabled');
    const socket = await dialAndRegister('TP03 BAD NUMBER');
    const closed = await waitClosed(socket);
    socket.destroy();
    expect(closed, 'malformed registration frame must be closed by the gateway').to.equal(true);
  });

  it('6. 启用路径：离线命令入断网缓冲，重连注册后收到命令帧', async function () {
    if (!TCP_ENABLED) this.skip('TCP gateway not enabled');
    expect(tenantADeviceId, 'test 2 must have created the device').to.be.a('string').and.not.equal('');

    // 离线下发：设备无 TCP 会话（且偶发竞态下命令晚到也会转为在线直投），两种路径最终都
    // 表现为"注册后/期间收到一条含 identify 的命令帧"，本用例只断言帧到达。
    const pub = await apiClient.post(
      '/command/datas/pub',
      { device_id: tenantADeviceId, identify: COMMAND_IDENTIFY, value: '{"led":"on"}' },
      'tenant_admin'
    );
    expect(pub.code, JSON.stringify(pub)).to.equal(200);

    const socket = await dialAndRegister(DEVICE_NUMBER);
    try {
      const payload = await readFrame(socket);
      expect(payload, 'gateway should deliver the buffered command frame after registration').to.not.equal(null);
      const text = payload.toString('utf8');
      expect(text).to.include(COMMAND_IDENTIFY);
      expect(() => JSON.parse(text), 'command frame payload must be JSON').to.not.throw();
    } finally {
      socket.destroy();
    }
  });

  it('7. 租户隔离：租户 B 无法读取租户 A 设备的当前遥测', async function () {
    if (!TCP_ENABLED) this.skip('TCP gateway not enabled');
    expect(tenantADeviceId).to.be.a('string').and.not.equal('');
    const res = await apiClient.get('/telemetry/datas/current/' + tenantADeviceId, {}, 'tenant_admin_b');
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });
});
