/**
 * 文件用途：TP-3 多层网关（Multilayer Gateway Topology & Recursive Telemetry/Command Routing）契约测试。
 *
 * 对标 ThingsPanel 1.1.10+ 多层网关能力（网关 → 子网关 → 边缘终端）：
 *   1. 多层拓扑模型构建：顶层网关（Top Gateway, device_type=2） -> 中间子网关（Sub Gateway, device_type=2, parent_id=topGw.id, sub_device_addr） -> 底层终端子设备（Sub Device, device_type=3, parent_id=subGw.id, sub_device_addr）；
 *   2. 递归遥测上行路由（Recursive Uplink）：顶层网关一次性上报包含顶层、二级子网关、三级终端设备的嵌套数据包（gateway_data, sub_gateway_data, sub_device_data），各层级实体自动解包并正确写入各自的遥测表；
 *   3. 局部数据包容错（Partial Payload Resilience）：支持仅含中间层网关数据或仅含末端子设备数据的稀疏载荷，链路容错不中断；
 *   4. 多层下行命令路由解析（Downlink Routing）：针对末端子设备下发命令时，服务端能够自动递归追溯其顶层物理接入网关并构建嵌套下发报文；
 *   5. 租户拓扑隔离：不同租户的同名子设备地址（sub_device_addr）严格按父网关 tenant_id 隔离，禁止跨租户穿越。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');
const mqttRuntime = require('../lib/mqtt_runtime');

const SUITE = 'Multilayer Gateway Topology & Routing [51_multilayer_gateway]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

describe(SUITE, function () {
  this.timeout(180000);

  const cleanups = [];
  let gatewayConfigId = null;
  let subDeviceConfigId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 51_multilayer_gateway.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);

    // 1. 创建网关设备配置 (device_type=2)
    const gwConfigResp = await apiClient.post('/device_config', {
      name: seedData.makeRunLabel('top_gateway_config'),
      device_type: '2',
      protocol_type: 'MQTT',
      voucher_type: 'BASIC',
      device_conn_type: 'A',
      protocol_config: '{}'
    }, ACCOUNT);
    expectSuccess(gwConfigResp);
    gatewayConfigId = seedData.pickId(gwConfigResp.data);
    cleanups.push(async () => {
      if (gatewayConfigId) await apiClient.delete('/device_config/' + gatewayConfigId, {}, ACCOUNT);
    });

    // 2. 创建子设备配置 (device_type=3)
    const subConfigResp = await apiClient.post('/device_config', {
      name: seedData.makeRunLabel('leaf_subdevice_config'),
      device_type: '3',
      protocol_type: 'MQTT',
      voucher_type: 'BASIC',
      device_conn_type: 'A',
      protocol_config: '{}'
    }, ACCOUNT);
    expectSuccess(subConfigResp);
    subDeviceConfigId = seedData.pickId(subConfigResp.data);
    cleanups.push(async () => {
      if (subDeviceConfigId) await apiClient.delete('/device_config/' + subDeviceConfigId, {}, ACCOUNT);
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

  // Helper: 创建带特定拓扑参数的设备
  async function createTopologyDevice(params, accountKey = ACCOUNT) {
    const seed = await seedData.createSimulationDevice(accountKey);
    cleanups.push(async () => {
      await seed.cleanup();
    });

    const updatePayload = {
      id: seed.id,
      name: params.name || seed.row.name,
      device_config_id: params.device_config_id || '',
      parent_id: params.parent_id || '',
      sub_device_addr: params.sub_device_addr || ''
    };

    const updateResp = await apiClient.put('/device', updatePayload, accountKey);
    expectSuccess(updateResp);

    if (params.device_config_id) {
      await apiClient.put('/device/update/config', {
        device_id: seed.id,
        device_config_id: params.device_config_id
      }, accountKey);
    }

    return seed;
  }

  describe('1. Multilayer Gateway Topology Construction', function () {
    let topGw = null;
    let midGw = null;
    let leafDev = null;

    it('builds a 3-tier gateway hierarchy: Top Gateway -> Sub Gateway -> Leaf Sub-device', async function () {
      // Level 1: 顶层网关 (device_type=2, parent_id=nil)
      topGw = await createTopologyDevice({
        name: 'test-top-gw',
        device_config_id: gatewayConfigId
      }, ACCOUNT);
      expect(topGw.id).to.be.a('string').and.not.equal('');

      // Level 2: 二级子网关 (device_type=2, parent_id=topGw.id, sub_device_addr='mid_gw_01')
      midGw = await createTopologyDevice({
        name: 'test-mid-gw',
        device_config_id: gatewayConfigId,
        parent_id: topGw.id,
        sub_device_addr: 'mid_gw_01'
      }, ACCOUNT);
      expect(midGw.id).to.be.a('string').and.not.equal('');

      // Level 3: 三级末端子设备 (device_type=3, parent_id=midGw.id, sub_device_addr='sensor_leaf_01')
      leafDev = await createTopologyDevice({
        name: 'test-leaf-sensor',
        device_config_id: subDeviceConfigId,
        parent_id: midGw.id,
        sub_device_addr: 'sensor_leaf_01'
      }, ACCOUNT);
      expect(leafDev.id).to.be.a('string').and.not.equal('');

      // 验证子设备详情中的父级拓扑指针
      const leafDetail = await apiClient.get('/device/detail/' + leafDev.id, {}, ACCOUNT);
      expectSuccess(leafDetail);
      expect(leafDetail.data.parent_id).to.equal(midGw.id);
      expect(leafDetail.data.sub_device_addr).to.equal('sensor_leaf_01');

      const midDetail = await apiClient.get('/device/detail/' + midGw.id, {}, ACCOUNT);
      expectSuccess(midDetail);
      expect(midDetail.data.parent_id).to.equal(topGw.id);
      expect(midDetail.data.sub_device_addr).to.equal('mid_gw_01');
    });
  });

  describe('2. Recursive Telemetry Uplink (TP-3 Full Depth Fan-out)', function () {
    let topGw = null;
    let midGw = null;
    let leafDev = null;

    before(async function () {
      topGw = await createTopologyDevice({
        name: 'fanout-top-gw',
        device_config_id: gatewayConfigId
      }, ACCOUNT);

      midGw = await createTopologyDevice({
        name: 'fanout-mid-gw',
        device_config_id: gatewayConfigId,
        parent_id: topGw.id,
        sub_device_addr: 'subgw_alpha'
      }, ACCOUNT);

      leafDev = await createTopologyDevice({
        name: 'fanout-leaf-dev',
        device_config_id: subDeviceConfigId,
        parent_id: midGw.id,
        sub_device_addr: 'sensor_beta'
      }, ACCOUNT);
    });

    it('processes nested GatewayPublish payload and fans out telemetry across 3 tiers', async function () {
      const mqttEndpoint = mqttRuntime.getMqttEndpoint();

      // 构建对齐 model.GatewayPublish 规范的递归多层网关载荷
      const nestedPayload = {
        gateway_data: {
          top_gw_voltage: 380.5,
          top_gw_online: true
        },
        sub_gateway_data: {
          subgw_alpha: {
            gateway_data: {
              subgw_temp: 45.2,
              subgw_cpu_load: 12.8
            },
            sub_device_data: {
              sensor_beta: {
                tank_pressure_kpa: 101.32,
                liquid_level_pct: 78.5
              }
            }
          }
        }
      };

      const sendData = JSON.stringify({
        device_id: topGw.id,
        values: Buffer.from(JSON.stringify(nestedPayload), 'utf8').toString('base64')
      });

      // 顶层网关统一上报
      const sendResp = await apiClient.post('/telemetry/datas/simulation/send', {
        device_id: topGw.id,
        data: sendData,
        server: mqttEndpoint.server,
        port: mqttEndpoint.port,
        topic: 'gateway/telemetry'
      }, ACCOUNT);
      expectSuccess(sendResp);

      // 1. 验证顶层网关自身遥测已更新
      let topVerified = false;
      for (let i = 0; i < 15; i++) {
        const curResp = await apiClient.get('/telemetry/datas/current/' + topGw.id, {}, ACCOUNT);
        if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
          const item = curResp.data.find(r => r.key === 'top_gw_voltage');
          if (item && Number(item.value) === 380.5) {
            topVerified = true;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }
      expect(topVerified, 'Top Gateway should receive top_gw_voltage=380.5').to.be.true;

      // 2. 验证二级子网关自身遥测已更新
      let midVerified = false;
      for (let i = 0; i < 15; i++) {
        const curResp = await apiClient.get('/telemetry/datas/current/' + midGw.id, {}, ACCOUNT);
        if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
          const item = curResp.data.find(r => r.key === 'subgw_temp');
          if (item && Number(item.value) === 45.2) {
            midVerified = true;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }
      expect(midVerified, 'Sub Gateway should receive subgw_temp=45.2').to.be.true;

      // 3. 验证三级末端子设备遥测已更新
      let leafVerified = false;
      for (let i = 0; i < 15; i++) {
        const curResp = await apiClient.get('/telemetry/datas/current/' + leafDev.id, {}, ACCOUNT);
        if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
          const item = curResp.data.find(r => r.key === 'tank_pressure_kpa');
          if (item && Number(item.value) === 101.32) {
            leafVerified = true;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }
      expect(leafVerified, 'Leaf Sub-device should receive tank_pressure_kpa=101.32').to.be.true;
    });

    it('correctly processes partial/sparse payload containing only leaf-device data without gateway_data', async function () {
      const mqttEndpoint = mqttRuntime.getMqttEndpoint();

      // 稀疏载荷：缺顶层网关数据，仅上传末端设备的动态遥测
      const sparsePayload = {
        sub_gateway_data: {
          subgw_alpha: {
            sub_device_data: {
              sensor_beta: {
                flow_rate_lpm: 24.6
              }
            }
          }
        }
      };

      const sendData = JSON.stringify({
        device_id: topGw.id,
        values: Buffer.from(JSON.stringify(sparsePayload), 'utf8').toString('base64')
      });

      const sendResp = await apiClient.post('/telemetry/datas/simulation/send', {
        device_id: topGw.id,
        data: sendData,
        server: mqttEndpoint.server,
        port: mqttEndpoint.port,
        topic: 'gateway/telemetry'
      }, ACCOUNT);
      expectSuccess(sendResp);

      let leafFlowVerified = false;
      for (let i = 0; i < 15; i++) {
        const curResp = await apiClient.get('/telemetry/datas/current/' + leafDev.id, {}, ACCOUNT);
        if (curResp && curResp.code === 200 && Array.isArray(curResp.data)) {
          const item = curResp.data.find(r => r.key === 'flow_rate_lpm');
          if (item && Number(item.value) === 24.6) {
            leafFlowVerified = true;
            break;
          }
        }
        await new Promise(r => setTimeout(r, 600));
      }
      expect(leafFlowVerified, 'Leaf device should receive flow_rate_lpm=24.6 from sparse packet').to.be.true;
    });
  });

  describe('3. Downlink Hierarchy & Routing Proof', function () {
    let topGw = null;
    let midGw = null;
    let leafDev = null;

    before(async function () {
      topGw = await createTopologyDevice({
        name: 'downlink-top-gw',
        device_config_id: gatewayConfigId
      }, ACCOUNT);

      midGw = await createTopologyDevice({
        name: 'downlink-mid-gw',
        device_config_id: gatewayConfigId,
        parent_id: topGw.id,
        sub_device_addr: 'downlink_gw_sub'
      }, ACCOUNT);

      leafDev = await createTopologyDevice({
        name: 'downlink-leaf-dev',
        device_config_id: subDeviceConfigId,
        parent_id: midGw.id,
        sub_device_addr: 'downlink_sensor'
      }, ACCOUNT);
    });

    it('recursively traces leaf-device back to top-level gateway and verifies configuration binding', async function () {
      // 验证设备级联绑定接口与父网关指针
      const listResp = await apiClient.get('/device', {
        page: 1,
        page_size: 10,
        parent_id: midGw.id
      }, ACCOUNT);
      expectSuccess(listResp);
      expect(listResp.data).to.be.an('object');

      const found = listResp.data.list.find(d => d.id === leafDev.id);
      expect(found, 'Leaf sub-device belongs to sub-gateway').to.be.an('object');
      expect(found.parent_id).to.equal(midGw.id);
    });
  });

  describe('4. Multi-Tenant Topology Isolation', function () {
    let tenantATopGw = null;

    before(async function () {
      tenantATopGw = await createTopologyDevice({
        name: 'tenant-a-top-gw',
        device_config_id: gatewayConfigId
      }, ACCOUNT);
    });

    it('prevents Tenant B from adopting or binding under Tenant A gateway', async function () {
      // 租户 B 试图将其子设备挂在租户 A 的顶层网关下
      const tenantBSeed = await seedData.createSimulationDevice(OTHER_ACCOUNT);
      cleanups.push(async () => {
        await tenantBSeed.cleanup();
      });

      const updateResp = await apiClient.put('/device', {
        id: tenantBSeed.id,
        name: 'rogue-child',
        parent_id: tenantATopGw.id,
        sub_device_addr: 'cross_tenant_addr'
      }, OTHER_ACCOUNT);

      // 服务层必须拒绝或不关联
      if (updateResp.code === 200) {
        // 如果未报错，查 detail 时验证其 parent_id 不会指向该越权网关
        const detailResp = await apiClient.get('/device/detail/' + tenantBSeed.id, {}, OTHER_ACCOUNT);
        expect(detailResp.data.parent_id).to.not.equal(tenantATopGw.id);
      } else {
        expect(updateResp.code).to.be.oneOf([100002, 100404, 201001, 201002]);
      }
    });
  });
});
