#!/usr/bin/env node
/**
 * 文件用途：本地最小 MQTT 3.1.1 回环 broker，**仅作为自动化测试夹具**。
 *
 * 为什么存在：
 *   本机没有 Docker、Go module proxy 不可达（proxy.golang.org 返回 Bad Gateway），
 *   因此无法构建仓库自带的 gmqtt broker。而遥测入库的唯一通道就是 MQTT：
 *   后端 `/telemetry/datas/simulation/send` 会 PUBLISH 到 `devices/telemetry`，
 *   再由后端自己的订阅端收下来入库。没有 broker，anomaly / 遥测类端到端用例无法运行。
 *
 * 边界（务必如实理解，不要据此宣称"MQTT 已验证"）：
 *   - 这是**测试替身**，不是生产组件，也不替代 gmqtt。它只实现让
 *     CONNECT / SUBSCRIBE / PUBLISH(QoS 0,1) / PINGREQ / DISCONNECT 跑通的最小子集。
 *   - 没有鉴权、没有持久会话、没有 retained、没有 QoS 2、没有集群、没有 TLS。
 *   - 因此 P0.2 的 shadow_ack、22_mqtt_device_pipeline 等在 gmqtt 上的行为差异
 *     **仍然必须在真实 broker 上复验**，本夹具不提供该证据。
 *
 * 用法：node scripts/local_mqtt_broker.js [--port 1883]
 */

const net = require('net');

const CONNECT = 1;
const CONNACK = 2;
const PUBLISH = 3;
const PUBACK = 4;
const SUBSCRIBE = 8;
const SUBACK = 9;
const UNSUBSCRIBE = 10;
const UNSUBACK = 11;
const PINGREQ = 12;
const PINGRESP = 13;
const DISCONNECT = 14;

function parseArgs(argv) {
  const args = { port: 1883, host: '127.0.0.1' };
  for (let i = 2; i < argv.length; i += 1) {
    if (argv[i] === '--port') args.port = Number(argv[argv.length > i + 1 ? i + 1 : i]);
    if (argv[i] === '--host') args.host = argv[i + 1];
  }
  if (!Number.isFinite(args.port) || args.port <= 0) args.port = 1883;
  return args;
}

/** 编码 MQTT 剩余长度（变长整数）。 */
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

/** 主题过滤匹配：支持单层 `+` 与多层 `#`（`#` 只能出现在末尾）。 */
function topicMatches(filter, topic) {
  const filterParts = filter.split('/');
  const topicParts = topic.split('/');
  for (let i = 0; i < filterParts.length; i += 1) {
    const f = filterParts[i];
    if (f === '#') return i === filterParts.length - 1;
    if (i >= topicParts.length) return false;
    if (f !== '+' && f !== topicParts[i]) return false;
  }
  return filterParts.length === topicParts.length;
}

class Broker {
  constructor() {
    /** @type {Map<net.Socket, Map<string, number>>} socket -> (filter -> qos) */
    this.subscriptions = new Map();
    this.stats = { connects: 0, publishes: 0, delivered: 0, subscribes: 0 };
  }

  addSubscription(socket, filter, qos) {
    let set = this.subscriptions.get(socket);
    if (!set) {
      set = new Map();
      this.subscriptions.set(socket, set);
    }
    set.set(filter, qos);
  }

  removeSocket(socket) {
    this.subscriptions.delete(socket);
  }

  publish(topic, payload, qos) {
    this.stats.publishes += 1;
    for (const [socket, filters] of this.subscriptions.entries()) {
      if (socket.destroyed) continue;
      let granted = 0;
      let matched = false;
      for (const [filter, subQos] of filters.entries()) {
        if (topicMatches(filter, topic)) {
          matched = true;
          granted = Math.max(granted, subQos);
        }
      }
      if (!matched) continue;
      const effective = Math.min(qos, granted);
      socket.write(buildPublish(topic, payload, effective));
      this.stats.delivered += 1;
    }
  }
}

let packetIdCounter = 1;
function nextPacketId() {
  packetIdCounter = packetIdCounter >= 0xffff ? 1 : packetIdCounter + 1;
  return packetIdCounter;
}

function buildPublish(topic, payload, qos) {
  const topicBytes = Buffer.from(topic, 'utf8');
  const variableHeader = Buffer.concat([Buffer.from([0x00, topicBytes.length]), topicBytes]);
  const pidBuf = Buffer.alloc(2);
  let body = variableHeader;
  if (qos > 0) {
    pidBuf.writeUInt16BE(nextPacketId(), 0);
    body = Buffer.concat([variableHeader, pidBuf]);
  }
  const flags = qos > 0 ? (qos << 1) : 0;
  const fixed = Buffer.concat([Buffer.from([(PUBLISH << 4) | flags]), encodeRemainingLength(body.length + payload.length)]);
  return Buffer.concat([fixed, body, payload]);
}

/**
 * 返回可变报文头在 packet 中的起始下标（即跳过固定头与变长剩余长度字段）。
 * 剩余长度最多 4 字节，因此这里的上界是 5。
 */
function headerEnd(packet) {
  let cursor = 1;
  let digit;
  do {
    digit = packet[cursor];
    cursor += 1;
  } while ((digit & 0x80) !== 0 && cursor < 5);
  return cursor;
}

function handlePacket(broker, socket, packet) {
  const type = packet[0] >> 4;

  if (type === CONNECT) {
    broker.stats.connects += 1;
    // CONNACK: session present=0, return code=0 (accepted)
    socket.write(Buffer.from([CONNACK << 4, 0x02, 0x00, 0x00]));
    return;
  }

  if (type === PINGREQ) {
    socket.write(Buffer.from([PINGRESP << 4, 0x00]));
    return;
  }

  if (type === DISCONNECT) {
    broker.removeSocket(socket);
    socket.end();
    return;
  }

  if (type === SUBSCRIBE) {
    // 固定头(1) + 剩余长度(1..4) + packet id(2) + 若干 (topic, qos)
    const offset = headerEnd(packet);
    const pid = packet.readUInt16BE(offset);
    let cursor = offset + 2;
    const returnCodes = [];
    while (cursor + 2 < packet.length) {
      const len = packet.readUInt16BE(cursor);
      cursor += 2;
      const filter = packet.toString('utf8', cursor, cursor + len);
      cursor += len;
      const qos = packet[cursor] & 0x03;
      cursor += 1;
      broker.addSubscription(socket, filter, qos);
      // 不支持 QoS 2：降级为 QoS 1（客户端会按 granted 发送）
      returnCodes.push(Math.min(qos, 1));
      broker.stats.subscribes += 1;
    }
    const body = Buffer.concat([Buffer.from([(pid >> 8) & 0xff, pid & 0xff]), Buffer.from(returnCodes)]);
    socket.write(Buffer.concat([Buffer.from([SUBACK << 4]), encodeRemainingLength(body.length), body]));
    return;
  }

  if (type === UNSUBSCRIBE) {
    const offset = headerEnd(packet);
    const pid = packet.readUInt16BE(offset);
    socket.write(Buffer.from([UNSUBACK << 4, 0x02, (pid >> 8) & 0xff, pid & 0xff]));
    return;
  }

  if (type === PUBLISH) {
    const qos = (packet[0] >> 1) & 0x03;
    const offset = headerEnd(packet);
    const topicLen = packet.readUInt16BE(offset);
    const topic = packet.toString('utf8', offset + 2, offset + 2 + topicLen);
    let cursor = offset + 2 + topicLen;
    let pid = 0;
    if (qos > 0) {
      pid = packet.readUInt16BE(cursor);
      cursor += 2;
    }
    const payload = packet.slice(cursor);
    broker.publish(topic, payload, qos);
    if (qos === 1) {
      socket.write(Buffer.from([PUBACK << 4, 0x02, (pid >> 8) & 0xff, pid & 0xff]));
    }
    return;
  }
}

/** 从 TCP 流中切出完整 MQTT 报文（含变长剩余长度）。 */
function createFrameParser(onPacket) {
  let buffer = Buffer.alloc(0);
  return chunk => {
    buffer = Buffer.concat([buffer, chunk]);
    for (;;) {
      if (buffer.length < 2) return;
      let multiplier = 1;
      let length = 0;
      let cursor = 1;
      let digit;
      do {
        if (cursor >= buffer.length) return; // 需要更多字节
        digit = buffer[cursor];
        length += (digit & 0x7f) * multiplier;
        multiplier *= 128;
        cursor += 1;
      } while ((digit & 0x80) !== 0 && cursor < 5);

      const total = cursor + length;
      if (buffer.length < total) return;
      const packet = buffer.slice(0, total);
      buffer = buffer.slice(total);
      onPacket(packet);
      if (buffer.length === 0) return;
    }
  };
}

function main() {
  const args = parseArgs(process.argv);
  const broker = new Broker();

  const server = net.createServer(socket => {
    socket.setNoDelay(true);
    const parse = createFrameParser(packet => {
      try {
        handlePacket(broker, socket, packet);
      } catch (error) {
        process.stderr.write(`packet handling failed: ${error && error.message}\n`);
      }
    });
    socket.on('data', parse);
    socket.on('error', () => broker.removeSocket(socket));
    socket.on('close', () => broker.removeSocket(socket));
  });

  server.listen(args.port, args.host, () => {
    process.stdout.write(`minimal-mqtt-broker listening on mqtt://${args.host}:${args.port}\n`);
  });

  const shutdown = () => {
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 1000).unref();
  };
  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);
}

if (require.main === module) main();

module.exports = { topicMatches, encodeRemainingLength, Broker };
