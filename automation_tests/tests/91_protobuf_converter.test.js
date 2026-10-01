/**
 * 文件用途：通用 Protobuf 载荷编解码（TB-19，135.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. PROTOBUF 转换器 CRUD 生命周期（POST /api/v1/converters 创建/PUT 更新、
 *      GET /api/v1/converters/:id 详情回读 proto_schema、DELETE 删除）；
 *   2. Dry-Run 仿真（POST /api/v1/converters/test）：固定 .proto + 预编码二进制载荷
 *      （手工按 protobuf wire format 构造，不依赖被测后端）经 hex 与 base64 双通道解码，
 *      断言 telemetry 映射（含枚举符号名、repeated 下标、嵌套消息、map 键）；
 *   3. 未知字段容错（payload 携带未注册 tag 99/100 不阻断解码）；
 *   4. fail-closed 负例（proto_schema 缺失/非法、载荷既非 hex 也非 base64）；
 *   5. 参数校验（converter_mode oneof 拒绝非法模式）；
 *   6. 严格多租户隔离（租户 B 无法查询/仿真/删除租户 A 的转换器）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Protobuf Converter [91_protobuf_converter]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

// 与 backend/internal/service/data_converter_protobuf_test.go 同源的固定 .proto：
// 覆盖标量/枚举/repeated/嵌套消息/map/bytes/float32。
const PROTO_SCHEMA = `syntax = "proto3";
package iot.tb19;

message EnvironmentReading {
  string device_label = 1;
  double temperature_c = 2;
  int64 humidity_ppm = 3;
  bool alarm_active = 4;
  Status status = 5;
  repeated double samples = 6;
  NestedMeta meta = 7;
  map<string, string> labels = 8;
  bytes payload_ref = 9;
  float battery_v = 10;

  enum Status {
    UNKNOWN = 0;
    OK = 1;
    DEGRADED = 2;
  }
  message NestedMeta {
    string zone = 1;
    int32 rssi = 2;
  }
}
`;

// 标准 unsigned varint 编码：负数按 64 位补码符号扩展（与 Go uint64 语义一致），
// 例如 -71 → B9 FF FF FF FF FF FF FF FF 01（10 字节）。
function uvarint(value) {
  const out = [];
  let n = BigInt.asUintN(64, typeof value === 'bigint' ? value : BigInt(value));
  do {
    let byte = Number(n & 0x7fn);
    n >>= 7n;
    if (n !== 0n) {
      byte |= 0x80;
    }
    out.push(byte);
  } while (n !== 0n);
  return Buffer.from(out);
}

function tag(fieldNumber, wireType) {
  return uvarint((fieldNumber << 3) | wireType);
}

// 手工按 wire format 预编码 EnvironmentReading（与 Go 单测 buildTb19Payload 一致）：
// device_label="dev-01", temperature_c=21.5, humidity_ppm=65432, alarm_active=true,
// status=DEGRADED(2), samples=[20.5,21.0](packed), meta{zone="zone-a",rssi=-71},
// labels{"zone":"A1"}, payload_ref=0x010203, battery_v=float32(3.7)。
function buildPayload() {
  const chunks = [];
  const label = Buffer.from('dev-01', 'utf8');
  chunks.push(tag(1, 2), uvarint(label.length), label); // field 1 string
  const temperature = Buffer.alloc(8);
  temperature.writeDoubleLE(21.5, 0);
  chunks.push(tag(2, 1), temperature); // field 2 double
  chunks.push(tag(3, 0), uvarint(65432)); // field 3 int64
  chunks.push(tag(4, 0), Buffer.from([0x01])); // field 4 bool
  chunks.push(tag(5, 0), Buffer.from([0x02])); // field 5 enum DEGRADED
  const packed = Buffer.alloc(16);
  packed.writeDoubleLE(20.5, 0);
  packed.writeDoubleLE(21.0, 8);
  chunks.push(tag(6, 2), uvarint(packed.length), packed); // field 6 packed repeated
  const zone = Buffer.from('zone-a', 'utf8');
  const meta = Buffer.concat([tag(1, 2), uvarint(zone.length), zone, tag(2, 0), uvarint(-71)]);
  chunks.push(tag(7, 2), uvarint(meta.length), meta); // field 7 submessage
  const key = Buffer.from('zone', 'utf8');
  const value = Buffer.from('A1', 'utf8');
  const entry = Buffer.concat([tag(1, 2), uvarint(key.length), key, tag(2, 2), uvarint(value.length), value]);
  chunks.push(tag(8, 2), uvarint(entry.length), entry); // field 8 map entry
  const bytesRef = Buffer.from([0x01, 0x02, 0x03]);
  chunks.push(tag(9, 2), uvarint(bytesRef.length), bytesRef); // field 9 bytes
  const battery = Buffer.alloc(4);
  battery.writeFloatLE(3.7, 0);
  chunks.push(tag(10, 5), battery); // field 10 float
  return Buffer.concat(chunks);
}

// 追加两个未注册字段（99=varint 42、100=2 字节长度限定），验证未知字段容错。
function appendUnknownFields(payload) {
  const chunks = [payload];
  chunks.push(tag(99, 0), Buffer.from([0x2a])); // unknown field 99 = 42
  const unknownBody = Buffer.from([0x01, 0x02]);
  chunks.push(tag(100, 2), uvarint(unknownBody.length), unknownBody); // unknown field 100
  return Buffer.concat(chunks);
}

const CONVERTER_CONFIG = JSON.stringify({
  message_type: 'EnvironmentReading',
  device_name: 'device_label',
  telemetry: {
    temperature: 'temperature_c',
    humidity: 'humidity_ppm',
    alarm: 'alarm_active',
    status: 'status',
    sample0: 'samples[0]',
    samples: 'samples',
    zone: 'meta.zone',
    rssi: 'meta.rssi',
    label_zone: 'labels.zone',
    ref: 'payload_ref',
    battery: 'battery_v'
  },
  attributes: { missing_attr: 'labels.fw' }
});

describe(SUITE, function () {
  this.timeout(120000);

  let createdConverterId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 91_protobuf_converter.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (createdConverterId) {
      try {
        await apiClient.delete('/converters/' + createdConverterId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. POST /converters 创建 PROTOBUF 转换器并回显 proto_schema', async function () {
    const res = await apiClient.post(
      '/converters',
      {
        name: '契约测试Protobuf转换器_91',
        type: 'UPLINK',
        converter_mode: 'PROTOBUF',
        configuration: CONVERTER_CONFIG,
        proto_schema: PROTO_SCHEMA
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.id).to.be.a('string').and.not.equal('');
    expect(res.data.converter_mode).to.equal('PROTOBUF');
    expect(res.data.proto_schema).to.equal(PROTO_SCHEMA);
    createdConverterId = res.data.id;
  });

  it('2. POST /converters 非法 converter_mode 被参数校验拒绝', async function () {
    const res = await apiClient.post(
      '/converters',
      {
        name: '契约测试非法模式_91',
        type: 'UPLINK',
        converter_mode: 'GRPC',
        configuration: '{}'
      },
      TENANT_A
    );
    expect(res.code).to.not.equal(200);
  });

  it('3. GET /converters/:id 详情回读 proto_schema', async function () {
    const res = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.proto_schema).to.equal(PROTO_SCHEMA);
    expect(res.data.configuration).to.equal(CONVERTER_CONFIG);
  });

  it('4. POST /converters/test hex 载荷解码出期望遥测', async function () {
    const res = await apiClient.post(
      '/converters/test',
      {
        converter_id: createdConverterId,
        payload: buildPayload().toString('hex')
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.success, JSON.stringify(res.data)).to.equal(true);
    expect(res.data.device_name).to.equal('dev-01');
    const telemetry = res.data.telemetry || {};
    expect(telemetry.temperature).to.equal(21.5);
    expect(telemetry.humidity).to.equal(65432);
    expect(telemetry.alarm).to.equal(true);
    expect(telemetry.status).to.equal('DEGRADED');
    expect(telemetry.sample0).to.equal(20.5);
    expect(telemetry.samples).to.deep.equal([20.5, 21.0]);
    expect(telemetry.zone).to.equal('zone-a');
    expect(telemetry.rssi).to.equal(-71);
    expect(telemetry.label_zone).to.equal('A1');
    expect(telemetry.ref).to.equal('010203');
    // 缺失的属性路径被跳过且不臆造。
    expect(res.data.attributes || {}).to.deep.equal({});
  });

  it('5. POST /converters/test base64 载荷与 hex 通道结果一致', async function () {
    const res = await apiClient.post(
      '/converters/test',
      {
        converter_id: createdConverterId,
        payload: buildPayload().toString('base64')
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.success).to.equal(true);
    expect(res.data.telemetry.temperature).to.equal(21.5);
    expect(res.data.telemetry.humidity).to.equal(65432);
  });

  it('6. POST /converters/test 未知字段容错不阻断解码', async function () {
    const res = await apiClient.post(
      '/converters/test',
      {
        converter_id: createdConverterId,
        payload: appendUnknownFields(buildPayload()).toString('hex')
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.success).to.equal(true);
    expect(res.data.telemetry.temperature).to.equal(21.5);
    const logs = (res.data.logs || []).join('\n');
    expect(logs).to.include('unknown');
  });

  it('7. POST /converters/test 内联非法 .proto fail-closed', async function () {
    const res = await apiClient.post(
      '/converters/test',
      {
        converter_mode: 'PROTOBUF',
        payload: '0a00',
        proto_schema: 'message Broken {',
        configuration: JSON.stringify({ telemetry: { temperature: 'temperature_c' } })
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.success).to.equal(false);
    expect(res.data.error).to.include('invalid proto schema');
  });

  it('8. POST /converters/test 内联缺 proto_schema fail-closed', async function () {
    const res = await apiClient.post(
      '/converters/test',
      {
        converter_mode: 'PROTOBUF',
        payload: '0a00',
        configuration: JSON.stringify({ telemetry: { temperature: 'temperature_c' } })
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.success).to.equal(false);
    expect(res.data.error).to.include('proto_schema is empty');
  });

  it('9. POST /converters/test 载荷既非 hex 也非 base64 fail-closed', async function () {
    const res = await apiClient.post(
      '/converters/test',
      {
        converter_id: createdConverterId,
        payload: '!!!not-encodable!!!'
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.success).to.equal(false);
    expect(res.data.error).to.include('invalid protobuf payload');
  });

  it('10. PUT /converters 更新 proto_schema 生效', async function () {
    const updatedSchema = PROTO_SCHEMA.replace('int64 humidity_ppm = 3;', 'int64 humidity_raw = 3;');
    const res = await apiClient.put(
      '/converters',
      {
        id: createdConverterId,
        name: '契约测试Protobuf转换器_91_改',
        proto_schema: updatedSchema
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.name).to.equal('契约测试Protobuf转换器_91_改');
    expect(res.data.proto_schema).to.equal(updatedSchema);
    const check = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_A);
    expect(check.code).to.equal(200);
    expect(check.data.proto_schema).to.equal(updatedSchema);
  });

  it('11. 租户 B 无法查询/仿真/删除租户 A 的 PROTOBUF 转换器', async function () {
    const getRes = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_B);
    expect(getRes.code).to.not.equal(200);

    const testRes = await apiClient.post(
      '/converters/test',
      { converter_id: createdConverterId, payload: buildPayload().toString('hex') },
      TENANT_B
    );
    expect(testRes.code).to.not.equal(200);

    const deleteRes = await apiClient.delete('/converters/' + createdConverterId, {}, TENANT_B);
    expect(deleteRes.code).to.not.equal(200);
    const stillThere = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_A);
    expect(stillThere.code).to.equal(200);
  });

  it('12. DELETE /converters/:id 删除后不可再查询', async function () {
    const res = await apiClient.delete('/converters/' + createdConverterId, {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const check = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_A);
    expect(check.code).to.not.equal(200);
    createdConverterId = null;
  });
});
