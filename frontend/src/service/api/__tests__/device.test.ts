/**
 * 文件用途: 设备 API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证设备分组、接入、配置、遥测、事件、命令、共享和调试接口。
 * 关键注意事项: 本文件覆盖面大但仍是前端请求层证据，不能代替后端设备权限和数据一致性测试。
 * 重构建议: 按设备列表、配置、遥测、共享/RDI、调试接口拆分，降低单文件审查成本。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { mockGet, mockPost, mockPut, mockDelete, mockDelete2 } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn(),
  mockDelete2: vi.fn()
}));

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost,
    put: mockPut,
    delete: mockDelete,
    delete2: mockDelete2
  }
}));

import {
  getDeviceGroup,
  deviceDictProtocolService,
  deviceDictProtocolServiceFirstLevel,
  deviceDictProtocolServiceSecondLevel,
  deviceGroupTree,
  deviceGroup,
  putDeviceGroup,
  putDeviceActive,
  deleteDeviceGroup,
  deviceGroupDetail,
  deviceList,
  deviceMapTelemetry,
  detachDeviceFromConfig,
  deviceListByGroup,
  deviceDetail,
  deviceGroupRelation,
  getDeviceGroupRelation,
  deviceAlarmStatus,
  deviceAlarmHistory,
  deviceAlarmList,
  deviceAlarmHistoryPut,
  deviceTemplate,
  getServiceList,
  deviceTemplateDetail,
  deviceConfig,
  deviceConfigAdd,
  deviceConfigEdit,
  deviceConfigInfo,
  deviceConfigDel,
  deviceConfigVoucherType,
  protocolPluginConfigForm,
  deviceConfigBatch,
  deleteDeviceGroupRelation,
  getDeviceConnectInfo,
  getPlugininfoByService,
  getDeviceConfigList,
  updateDeviceVoucher,
  deviceAdd,
  deviceConnectForm,
  checkDevice,
  deleteDevice,
  setDeviceScriptEnable,
  getDataScriptList,
  dataScriptAdd,
  dataScriptEdit,
  dataScriptQuiz,
  dataScriptDel,
  telemetryDataCurrent,
  telemetryDataCurrentKeys,
  telemetryDataHistoryList,
  telemetryDataDel,
  getTelemetryLogList,
  telemetryDataPub,
  expectMessageAdd,
  expectMessageList,
  expectMessageDelete,
  getAttributeDataSet,
  deleteAttributeDataSet,
  getAttributeDataSetLogs,
  attributeDataPub,
  getAttributeDatasKey,
  getEventDataSet,
  getCommandDataSetLogs,
  getCommandDeliveryDiagnostics,
  commandDataPub,
  invokeDirectMethod,
  commandDataById,
  getDeviceTwin,
  setDeviceTwinDesired,
  previewFleetCommandJob,
  submitFleetCommandJob,
  listFleetCommandJobs,
  getFleetCommandJob,
  getFleetCommandJobRows,
  getFleetCommandJobSupportBundle,
  cancelFleetCommandJob,
  retryFleetCommandJob,
  deviceTemplateSelect,
  telemetryHistoryData,
  deviceUpdateConfig,
  deviceConfigMenu,
  deviceLocation,
  deviceUpdate,
  childDeviceTableList,
  childDeviceSelectList,
  addChildDevice,
  removeChildDevice,
  getSimulation,
  sendSimulation,
  getSimulationInit,
  sendSimulationData,
  deviceCustomCommandsIdList,
  deviceProtocolServiceList,
  deviceStatusHistory,
  deviceDiagnostics,
  getTopicMappingList,
  createTopicMapping,
  updateTopicMapping,
  deleteTopicMapping,
  getDeviceDebugStatus,
  getDeviceOnlineStatus,
  setDeviceDebugStatus,
  getDeviceConnectionGuide,
  getDeviceDebugLogs,
  openDeviceMQTTDebugSession,
  getDeviceMQTTDebugSession,
  applyDeviceMQTTDebugCommand,
  closeDeviceMQTTDebugSession
} from '../device';

describe('Device API 层 - device.ts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getDeviceGroup', () => {
    it('调用 GET /device/group 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1 };
      await getDeviceGroup(params);
      expect(mockGet).toHaveBeenCalledWith('/device/group', { params });
    });
  });

  describe('deviceDictProtocolService', () => {
    it('调用 GET /dict/protocol/service', async () => {
      mockGet.mockResolvedValue({ error: null, data: [] });
      const params = { type: '1' };
      await deviceDictProtocolService(params);
      expect(mockGet).toHaveBeenCalledWith('/dict/protocol/service', params);
    });
  });

  describe('deviceDictProtocolServiceFirstLevel', () => {
    it('调用 GET /service/plugin/select', async () => {
      mockGet.mockResolvedValue({ error: null, data: [] });
      const params = {};
      await deviceDictProtocolServiceFirstLevel(params);
      expect(mockGet).toHaveBeenCalledWith('/service/plugin/select', params);
    });
  });

  describe('deviceDictProtocolServiceSecondLevel', () => {
    it('调用 GET /service/access/list', async () => {
      mockGet.mockResolvedValue({ error: null, data: [] });
      const params = { service_id: 's1' };
      await deviceDictProtocolServiceSecondLevel(params);
      expect(mockGet).toHaveBeenCalledWith('/service/access/list', params);
    });
  });

  describe('deviceGroupTree', () => {
    it('调用 GET /device/group/tree', async () => {
      mockGet.mockResolvedValue({ error: null, data: [] });
      const params = {};
      await deviceGroupTree(params);
      expect(mockGet).toHaveBeenCalledWith('/device/group/tree', params);
    });
  });

  describe('deviceGroup', () => {
    it('调用 POST /device/group 并发送分组数据', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { id: '', parent_id: '0', name: '分组1', description: '描述' };
      await deviceGroup(params);
      expect(mockPost).toHaveBeenCalledWith('/device/group', params);
    });
  });

  describe('putDeviceGroup', () => {
    it('调用 PUT /device/group 并发送修改数据', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { id: '1', parent_id: '0', name: '分组2', description: '修改' };
      await putDeviceGroup(params);
      expect(mockPut).toHaveBeenCalledWith('/device/group', params);
    });
  });

  describe('putDeviceActive', () => {
    it('调用 PUT /device/active', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await putDeviceActive(params);
      expect(mockPut).toHaveBeenCalledWith('/device/active', params);
    });
  });

  describe('deleteDeviceGroup', () => {
    it('调用 DELETE /device/group/{id}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'g1' };
      await deleteDeviceGroup(params);
      expect(mockDelete).toHaveBeenCalledWith('/device/group/g1');
    });
  });

  describe('deviceGroupDetail', () => {
    it('调用 GET /device/group/detail/{id}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'g1' };
      await deviceGroupDetail(params);
      expect(mockGet).toHaveBeenCalledWith('/device/group/detail/g1');
    });
  });

  describe('deviceList', () => {
    it('调用 GET /device 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1, page_size: 10 };
      await deviceList(params);
      expect(mockGet).toHaveBeenCalledWith('/device', { params });
    });
  });

  describe('deviceMapTelemetry', () => {
    it('调用 GET /device/map/telemetry/{id}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await deviceMapTelemetry('dev1');
      expect(mockGet).toHaveBeenCalledWith('/device/map/telemetry/dev1');
    });
  });

  describe('detachDeviceFromConfig', () => {
    it('调用 PUT /device/update/config 解除设备配置绑定', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', config: {} };
      await detachDeviceFromConfig(params);
      expect(mockPut).toHaveBeenCalledWith('/device/update/config', params);
    });
  });

  describe('deviceListByGroup', () => {
    it('调用 GET /device/group/relation/list 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { group_id: 'g1' };
      await deviceListByGroup(params);
      expect(mockGet).toHaveBeenCalledWith('/device/group/relation/list', { params });
    });
  });

  describe('deviceDetail', () => {
    it('调用 GET /device/detail/{id}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await deviceDetail('dev1');
      expect(mockGet).toHaveBeenCalledWith('/device/detail/dev1');
    });
  });

  describe('deviceGroupRelation', () => {
    it('调用 POST /device/group/relation', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', group_id: 'g1' };
      await deviceGroupRelation(params);
      expect(mockPost).toHaveBeenCalledWith('/device/group/relation', params);
    });
  });

  describe('getDeviceGroupRelation', () => {
    it('调用 GET /device/group/relation 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getDeviceGroupRelation(params);
      expect(mockGet).toHaveBeenCalledWith('/device/group/relation', { params });
    });
  });

  describe('deviceAlarmStatus', () => {
    it('调用 GET /alarm/info/history/device 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await deviceAlarmStatus(params);
      expect(mockGet).toHaveBeenCalledWith('/alarm/info/history/device', { params });
    });
  });

  describe('deviceAlarmHistory', () => {
    it('调用 GET /alarm/info/history 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await deviceAlarmHistory(params);
      expect(mockGet).toHaveBeenCalledWith('/alarm/info/history', { params });
    });
  });

  describe('deviceAlarmList', () => {
    it('调用 GET /scene_automations/alarm 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1 };
      await deviceAlarmList(params);
      expect(mockGet).toHaveBeenCalledWith('/scene_automations/alarm', { params });
    });
  });

  describe('deviceAlarmHistoryPut', () => {
    it('调用 PUT /alarm/info/history', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'a1', description: '已处理' };
      await deviceAlarmHistoryPut(params);
      expect(mockPut).toHaveBeenCalledWith('/alarm/info/history', params);
    });
  });

  describe('deviceTemplate', () => {
    it('调用 GET /device/template 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1 };
      await deviceTemplate(params);
      expect(mockGet).toHaveBeenCalledWith('/device/template', { params });
    });
  });

  describe('getServiceList', () => {
    it('调用 GET /service/list 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1 };
      await getServiceList(params);
      expect(mockGet).toHaveBeenCalledWith('/service/list', { params });
    });
  });

  describe('deviceTemplateDetail', () => {
    it('调用 GET /device/template/detail/{id}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { id: 't1' };
      await deviceTemplateDetail(params);
      expect(mockGet).toHaveBeenCalledWith('/device/template/detail/t1');
    });
  });

  describe('deviceConfig', () => {
    it('调用 GET /device_config 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1 };
      await deviceConfig(params);
      expect(mockGet).toHaveBeenCalledWith('/device_config', { params });
    });
  });

  describe('deviceConfigAdd', () => {
    it('调用 POST /device_config', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { name: '配置1' };
      await deviceConfigAdd(params);
      expect(mockPost).toHaveBeenCalledWith('/device_config', params);
    });
  });

  describe('deviceConfigEdit', () => {
    it('调用 PUT /device_config', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'c1', name: '配置2' };
      await deviceConfigEdit(params);
      expect(mockPut).toHaveBeenCalledWith('/device_config', params);
    });
  });

  describe('deviceConfigInfo', () => {
    it('调用 GET /device_config/{id}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'c1' };
      await deviceConfigInfo(params);
      expect(mockGet).toHaveBeenCalledWith('/device_config/c1');
    });
  });

  describe('deviceConfigDel', () => {
    it('调用 DELETE /device_config/{id}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'c1' };
      await deviceConfigDel(params);
      expect(mockDelete).toHaveBeenCalledWith('/device_config/c1');
    });
  });

  describe('deviceConfigVoucherType', () => {
    it('调用 GET /device_config/voucher_type 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = {};
      await deviceConfigVoucherType(params);
      expect(mockGet).toHaveBeenCalledWith('/device_config/voucher_type', { params });
    });
  });

  describe('protocolPluginConfigForm', () => {
    it('调用 GET /protocol_plugin/config_form 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_config_id: 'c1' };
      await protocolPluginConfigForm(params);
      expect(mockGet).toHaveBeenCalledWith('/protocol_plugin/config_form', { params });
    });
  });

  describe('deviceConfigBatch', () => {
    it('调用 PUT /device_config/batch', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { ids: ['1', '2'] };
      await deviceConfigBatch(params);
      expect(mockPut).toHaveBeenCalledWith('/device_config/batch', params);
    });
  });

  describe('deleteDeviceGroupRelation', () => {
    it('调用 delete2 /device/group/relation', async () => {
      mockDelete2.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await deleteDeviceGroupRelation(params);
      expect(mockDelete2).toHaveBeenCalledWith('/device/group/relation', params);
    });
  });

  describe('getDeviceConnectInfo', () => {
    it('调用 GET /device/connect/info 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getDeviceConnectInfo(params);
      expect(mockGet).toHaveBeenCalledWith('/device/connect/info', { params });
    });
  });

  describe('getPlugininfoByService', () => {
    it('调用 GET /service/plugin/info 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { service_id: 's1' };
      await getPlugininfoByService(params);
      expect(mockGet).toHaveBeenCalledWith('/service/plugin/info', { params });
    });
  });

  describe('getDeviceConfigList', () => {
    it('调用 GET /device_config 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1 };
      await getDeviceConfigList(params);
      expect(mockGet).toHaveBeenCalledWith('/device_config', { params });
    });
  });

  describe('updateDeviceVoucher', () => {
    it('调用 POST /device/update/voucher', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', voucher: 'v1' };
      await updateDeviceVoucher(params);
      expect(mockPost).toHaveBeenCalledWith('/device/update/voucher', params);
    });
  });

  describe('deviceAdd', () => {
    it('调用 POST /device', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { name: '设备1' };
      await deviceAdd(params);
      expect(mockPost).toHaveBeenCalledWith('/device', params);
    });
  });

  describe('deviceConnectForm', () => {
    it('uses the current spelling for GET /device/connect/form', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await deviceConnectForm(params);
      expect(mockGet).toHaveBeenCalledWith('/device/connect/form', { params });
    });
  });

  describe('checkDevice', () => {
    it('调用 GET /device/check/{deviceNumber}，对特殊字符编码', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await checkDevice('dev/1');
      expect(mockGet).toHaveBeenCalledWith('/device/check/dev%2F1');
    });

    it('普通 deviceNumber 不编码', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await checkDevice('dev123');
      expect(mockGet).toHaveBeenCalledWith('/device/check/dev123');
    });
  });

  describe('deleteDevice', () => {
    it('调用 DELETE /device/{id}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'd1' };
      await deleteDevice(params);
      expect(mockDelete).toHaveBeenCalledWith('/device/d1');
    });
  });

  describe('setDeviceScriptEnable', () => {
    it('调用 PUT /data_script/enable', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { id: 's1', enabled: true };
      await setDeviceScriptEnable(params);
      expect(mockPut).toHaveBeenCalledWith('/data_script/enable', params);
    });
  });

  describe('getDataScriptList', () => {
    it('调用 GET /data_script 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1 };
      await getDataScriptList(params);
      expect(mockGet).toHaveBeenCalledWith('/data_script', { params });
    });
  });

  describe('dataScriptAdd', () => {
    it('调用 POST /data_script', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { name: '脚本1' };
      await dataScriptAdd(params);
      expect(mockPost).toHaveBeenCalledWith('/data_script', params);
    });
  });

  describe('dataScriptEdit', () => {
    it('调用 PUT /data_script', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { id: 's1', name: '脚本2' };
      await dataScriptEdit(params);
      expect(mockPut).toHaveBeenCalledWith('/data_script', params);
    });
  });

  describe('dataScriptQuiz', () => {
    it('调用 POST /data_script/quiz 并携带 needMessage 配置', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { script: 'test' };
      await dataScriptQuiz(params);
      expect(mockPost).toHaveBeenCalledWith('/data_script/quiz', params, { needMessage: true });
    });
  });

  describe('dataScriptDel', () => {
    it('调用 DELETE /data_script/{id}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      const params = { id: 's1' };
      await dataScriptDel(params);
      expect(mockDelete).toHaveBeenCalledWith('/data_script/s1');
    });
  });

  describe('telemetryDataCurrent', () => {
    it('调用 GET /telemetry/datas/current/{id}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await telemetryDataCurrent('dev1');
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/current/dev1', {});
    });

    it('支持传入额外的 requestConfig', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const requestConfig = { headers: { 'X-Custom': '1' } };
      await telemetryDataCurrent('dev1', requestConfig);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/current/dev1', requestConfig);
    });
  });

  describe('telemetryDataCurrentKeys', () => {
    it('调用 GET /telemetry/datas/current/keys 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', keys: 'temp' };
      await telemetryDataCurrentKeys(params);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/current/keys', { params });
    });
  });

  describe('telemetryDataHistoryList', () => {
    it('成功时直接返回 statistic 响应', async () => {
      const successResponse = { error: null, data: { list: [] } };
      mockGet.mockResolvedValue(successResponse);
      const params = { device_id: 'd1', key: 'temp' };
      const result = await telemetryDataHistoryList(params);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/statistic', { params });
      expect(result).toBe(successResponse);
    });

    it('非 OID 编码错误时直接返回 statistic 响应', async () => {
      const errorResponse = { error: { message: 'some other error' }, data: null };
      mockGet.mockResolvedValue(errorResponse);
      const params = { device_id: 'd1', key: 'temp' };
      const result = await telemetryDataHistoryList(params);
      expect(result).toBe(errorResponse);
    });

    it('OID 编码错误 + custom time_range 时回退到 history 接口', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValueOnce(oidError);
      mockGet.mockResolvedValueOnce({ error: null, data: { list: [{ ts: 1000, value: 42, key: 'temp' }] } });
      const params = { device_id: 'd1', key: 'temp', time_range: 'custom', start_time: '1000', end_time: '2000' };
      const result = await telemetryDataHistoryList(params);
      expect(result).toEqual({ data: [{ key: 'temp', x: 1000, y: 42 }], error: null });
    });

    it('OID 编码错误 + 预设 time_range 时回退到 history 接口', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValueOnce(oidError);
      mockGet.mockResolvedValueOnce({ error: null, data: { list: [{ ts: 1000, value: 42, key: 'temp' }] } });
      const params = { device_id: 'd1', key: 'temp', time_range: 'last_5m' };
      const result = await telemetryDataHistoryList(params);
      expect(result.error).toBeNull();
      expect(result.data).toEqual([{ key: 'temp', x: 1000, y: 42 }]);
    });

    it('OID 编码错误 + 无效 time_range 时返回原始 statistic 响应', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValue(oidError);
      const params = { device_id: 'd1', key: 'temp', time_range: 'invalid_range' };
      const result = await telemetryDataHistoryList(params);
      expect(result).toBe(oidError);
    });

    it('OID 编码错误 + 回退 history 也失败时返回原始 statistic 响应', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValueOnce(oidError);
      mockGet.mockResolvedValueOnce({ error: { message: 'history error' }, data: null });
      const params = { device_id: 'd1', key: 'temp', time_range: 'last_5m' };
      const result = await telemetryDataHistoryList(params);
      expect(result).toBe(oidError);
    });

    it('OID 编码错误 + 无 time_range 时返回原始 statistic 响应', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValue(oidError);
      const params = { device_id: 'd1', key: 'temp' };
      const result = await telemetryDataHistoryList(params);
      expect(result).toBe(oidError);
    });

    it('normalizeTelemetryHistoryPageData: 使用 value/y/avg 和 ts/time/x 字段', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValueOnce(oidError);
      mockGet.mockResolvedValueOnce({
        error: null,
        data: {
          list: [
            { ts: 1000, value: 10, key: 'k1' },
            { time: 2000, y: 20, key: 'k2' },
            { x: 3000, avg: 30, key: 'k3' }
          ]
        }
      });
      const params = { device_id: 'd1', key: 'temp', time_range: 'custom', start_time: '1000', end_time: '4000' };
      const result = await telemetryDataHistoryList(params);
      expect(result.data).toEqual([
        { key: 'k1', x: 1000, y: 10 },
        { key: 'k2', x: 2000, y: 20 },
        { key: 'k3', x: 3000, y: 30 }
      ]);
    });

    it('normalizeTelemetryHistoryPageData: 过滤掉 x/y 为 NaN 的项', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValueOnce(oidError);
      mockGet.mockResolvedValueOnce({
        error: null,
        data: {
          list: [
            { ts: 1000, value: 10, key: 'k1' },
            { ts: NaN, value: 20, key: 'k2' },
            { ts: 3000, value: NaN, key: 'k3' }
          ]
        }
      });
      const params = { device_id: 'd1', key: 'temp', time_range: 'custom', start_time: '1000', end_time: '4000' };
      const result = await telemetryDataHistoryList(params);
      expect(result.data).toEqual([{ key: 'k1', x: 1000, y: 10 }]);
    });

    it('normalizeTelemetryHistoryPageData: 缺失 x/y 字段时保持旧的 0 默认值', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValueOnce(oidError);
      mockGet.mockResolvedValueOnce({
        error: null,
        data: {
          list: [{ key: 'missing-point' }]
        }
      });
      const params = { device_id: 'd1', key: 'temp', time_range: 'custom', start_time: '1000', end_time: '4000' };
      const result = await telemetryDataHistoryList(params);
      expect(result.data).toEqual([{ key: 'missing-point', x: 0, y: 0 }]);
    });

    it('normalizeTelemetryHistoryPageData: list 非数组时返回空数组', async () => {
      const oidError = { error: { message: 'failed to encode args[2] OID 25' }, data: null };
      mockGet.mockResolvedValueOnce(oidError);
      mockGet.mockResolvedValueOnce({ error: null, data: null });
      const params = { device_id: 'd1', key: 'temp', time_range: 'custom', start_time: '1000', end_time: '2000' };
      const result = await telemetryDataHistoryList(params);
      expect(result.data).toEqual([]);
    });

    it('支持传入额外的 requestConfig', async () => {
      mockGet.mockResolvedValue({ error: null, data: [] });
      const params = { device_id: 'd1', key: 'temp' };
      const requestConfig = { timeout: 5000 };
      await telemetryDataHistoryList(params, requestConfig);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/statistic', { timeout: 5000, params });
    });

    it('shouldFallbackTelemetryHistory: error.message 为字符串时匹配', async () => {
      const oidError = { error: null, message: 'failed to encode args[2] OID 25' };
      mockGet.mockResolvedValueOnce(oidError);
      mockGet.mockResolvedValueOnce({ error: null, data: { list: [{ ts: 1000, value: 10, key: 'k1' }] } });
      const params = { device_id: 'd1', key: 'temp', time_range: 'custom', start_time: '1000', end_time: '2000' };
      const result = await telemetryDataHistoryList(params);
      expect(result.error).toBeNull();
    });
  });

  describe('telemetryDataDel', () => {
    it('调用 delete2 /telemetry/datas', async () => {
      mockDelete2.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', key: 'temp' };
      await telemetryDataDel(params);
      expect(mockDelete2).toHaveBeenCalledWith('/telemetry/datas', params);
    });
  });

  describe('getTelemetryLogList', () => {
    it('调用 GET /telemetry/datas/set/logs 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getTelemetryLogList(params);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/set/logs', { params });
    });
  });

  describe('telemetryDataPub', () => {
    it('调用 POST /telemetry/datas/pub', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', data: {} };
      await telemetryDataPub(params);
      expect(mockPost).toHaveBeenCalledWith('/telemetry/datas/pub', params);
    });
  });

  describe('expectMessageAdd', () => {
    it('调用 POST /expected/data', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', data: {} };
      await expectMessageAdd(params);
      expect(mockPost).toHaveBeenCalledWith('/expected/data', params);
    });
  });

  describe('expectMessageList', () => {
    it('调用 GET /expected/data/list 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await expectMessageList(params);
      expect(mockGet).toHaveBeenCalledWith('/expected/data/list', { params });
    });
  });

  describe('expectMessageDelete', () => {
    it('调用 DELETE /expected/data/{params}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      await expectMessageDelete('msg1');
      expect(mockDelete).toHaveBeenCalledWith('/expected/data/msg1');
    });
  });

  describe('getAttributeDataSet', () => {
    it('调用 GET /attribute/datas/{device_id}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getAttributeDataSet(params, {});
      expect(mockGet).toHaveBeenCalledWith('/attribute/datas/d1', {});
    });
  });

  describe('deleteAttributeDataSet', () => {
    it('调用 DELETE /attribute/datas/{params}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      await deleteAttributeDataSet('attr1');
      expect(mockDelete).toHaveBeenCalledWith('/attribute/datas/attr1');
    });
  });

  describe('getAttributeDataSetLogs', () => {
    it('调用 GET /attribute/datas/set/logs 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getAttributeDataSetLogs(params);
      expect(mockGet).toHaveBeenCalledWith('/attribute/datas/set/logs', { params });
    });
  });

  describe('attributeDataPub', () => {
    it('调用 POST /attribute/datas/pub', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', data: {} };
      await attributeDataPub(params);
      expect(mockPost).toHaveBeenCalledWith('/attribute/datas/pub', params);
    });
  });

  describe('getAttributeDatasKey', () => {
    it('调用 GET /attribute/datas/key 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', key: 'k1' };
      await getAttributeDatasKey(params);
      expect(mockGet).toHaveBeenCalledWith('/attribute/datas/key', { params });
    });
  });

  describe('getEventDataSet', () => {
    it('调用 GET /event/datas 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getEventDataSet(params);
      expect(mockGet).toHaveBeenCalledWith('/event/datas', { params });
    });
  });

  describe('getCommandDataSetLogs', () => {
    it('调用 GET /command/datas/set/logs 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getCommandDataSetLogs(params);
      expect(mockGet).toHaveBeenCalledWith('/command/datas/set/logs', { params });
    });
  });

  describe('getCommandDeliveryDiagnostics', () => {
    it('调用 GET /command/datas/delivery/diagnostics/{deviceId} 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await getCommandDeliveryDiagnostics('d1', { limit: 5 });
      expect(mockGet).toHaveBeenCalledWith('/command/datas/delivery/diagnostics/d1', { params: { limit: 5 } });
    });
  });

  describe('commandDataPub', () => {
    it('调用 POST /command/datas/pub', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', data: {} };
      await commandDataPub(params);
      expect(mockPost).toHaveBeenCalledWith('/command/datas/pub', params);
    });
  });

  describe('invokeDirectMethod', () => {
    it('调用 POST /command/datas/direct-method 并传递等待超时', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', identify: 'reboot', value: '{}', timeout_seconds: 10 };
      await invokeDirectMethod(params);
      expect(mockPost).toHaveBeenCalledWith('/command/datas/direct-method', params);
    });
  });

  describe('commandDataById', () => {
    it('调用 GET /command/datas/{id}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await commandDataById('dev1');
      expect(mockGet).toHaveBeenCalledWith('/command/datas/dev1');
    });
  });

  describe('device twin API wrappers', () => {
    it('调用 GET /device/twin/{id} 并传递 requestConfig', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const requestConfig = { silentError: true };
      await getDeviceTwin('d1', requestConfig);
      expect(mockGet).toHaveBeenCalledWith('/device/twin/d1', requestConfig);
    });

    it('调用 PUT /device/twin/{id}/desired', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { source: 'telemetry' as const, key: 'temperature', desired: 23 };
      await setDeviceTwinDesired('d1', params);
      expect(mockPut).toHaveBeenCalledWith('/device/twin/d1/desired', params);
    });
  });

  describe('fleet command job API wrappers', () => {
    const payload = {
      device_ids: ['d1'],
      command_type: 'command',
      identifier: 'reboot',
      payload: {}
    } as any;

    it('调用 POST /command/datas/jobs/preview 并静默错误', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await previewFleetCommandJob(payload);
      expect(mockPost).toHaveBeenCalledWith('/command/datas/jobs/preview', payload, { silentError: true });
    });

    it('调用 POST /command/datas/jobs/submit 并静默错误', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await submitFleetCommandJob(payload);
      expect(mockPost).toHaveBeenCalledWith('/command/datas/jobs/submit', payload, { silentError: true });
    });

    it('调用 POST /command/datas/jobs/submit 时可请求 summary-only 响应', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await submitFleetCommandJob(payload, { include_rows: false });
      expect(mockPost).toHaveBeenCalledWith('/command/datas/jobs/submit', payload, {
        params: { include_rows: false },
        silentError: true
      });
    });

    it('调用 GET /command/datas/jobs 并携带筛选参数', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1, page_size: 10, status: 'failed' };
      await listFleetCommandJobs(params);
      expect(mockGet).toHaveBeenCalledWith('/command/datas/jobs', { params, silentError: true });
    });

    it('调用 GET /command/datas/jobs/{jobId}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await getFleetCommandJob('job-1');
      expect(mockGet).toHaveBeenCalledWith('/command/datas/jobs/job-1', {
        params: { include_rows: true },
        silentError: true
      });
    });

    it('调用 GET /command/datas/jobs/{jobId}/support-bundle', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await getFleetCommandJobSupportBundle('job-1');
      expect(mockGet).toHaveBeenCalledWith('/command/datas/jobs/job-1/support-bundle', { silentError: true });
    });

    it('调用 GET /command/datas/jobs/{jobId}/rows 时可携带状态筛选', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1, page_size: 200, status_filter: 'needs_attention' as const };
      await getFleetCommandJobRows('job-1', params);
      expect(mockGet).toHaveBeenCalledWith('/command/datas/jobs/job-1/rows', {
        params,
        silentError: true
      });
    });

    it('调用 POST /command/datas/jobs/{jobId}/cancel', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await cancelFleetCommandJob('job-1');
      expect(mockPost).toHaveBeenCalledWith('/command/datas/jobs/job-1/cancel', {}, { silentError: true });
    });

    it('调用 POST /command/datas/jobs/{jobId}/cancel 时可请求 summary-only 响应', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await cancelFleetCommandJob('job-1', { include_rows: false });
      expect(mockPost).toHaveBeenCalledWith('/command/datas/jobs/job-1/cancel', {}, {
        params: { include_rows: false },
        silentError: true
      });
    });

    it('调用 POST /command/datas/jobs/{jobId}/retry', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await retryFleetCommandJob('job-1');
      expect(mockPost).toHaveBeenCalledWith('/command/datas/jobs/job-1/retry', {}, { silentError: true });
    });

    it('调用 POST /command/datas/jobs/{jobId}/retry 时可请求 summary-only 响应', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      await retryFleetCommandJob('job-1', { include_rows: false });
      expect(mockPost).toHaveBeenCalledWith('/command/datas/jobs/job-1/retry', {}, {
        params: { include_rows: false },
        silentError: true
      });
    });
  });

  describe('deviceTemplateSelect', () => {
    it('调用 GET /device/template/chart/select', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await deviceTemplateSelect();
      expect(mockGet).toHaveBeenCalledWith('/device/template/chart/select');
    });
  });

  describe('telemetryHistoryData', () => {
    it('调用 GET /telemetry/datas/history/page 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', key: 'temp' };
      await telemetryHistoryData(params);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/history/page', { params });
    });

    it('支持传入额外的 requestConfig', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', key: 'temp' };
      const requestConfig = { timeout: 5000 };
      await telemetryHistoryData(params, requestConfig);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/history/page', { timeout: 5000, params });
    });
  });

  describe('deviceUpdateConfig', () => {
    it('调用 PUT /device/update/config', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', config: {} };
      await deviceUpdateConfig(params);
      expect(mockPut).toHaveBeenCalledWith('/device/update/config', params);
    });
  });

  describe('deviceConfigMenu', () => {
    it('调用 GET /device/template/menu 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_config_id: 'c1' };
      await deviceConfigMenu(params);
      expect(mockGet).toHaveBeenCalledWith('/device/template/menu', { params });
    });
  });

  describe('deviceLocation', () => {
    it('调用 PUT /device', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'd1', location: {} };
      await deviceLocation(params);
      expect(mockPut).toHaveBeenCalledWith('/device', params);
    });
  });

  describe('deviceUpdate', () => {
    it('调用 PUT /device', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'd1', name: '设备2' };
      await deviceUpdate(params);
      expect(mockPut).toHaveBeenCalledWith('/device', params);
    });
  });

  describe('childDeviceTableList', () => {
    it('调用 GET /device/sub-list/{id} 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { id: 'gw1', page: 1 };
      await childDeviceTableList(params);
      expect(mockGet).toHaveBeenCalledWith('/device/sub-list/gw1', { params });
    });
  });

  describe('childDeviceSelectList', () => {
    it('调用 GET /device/list', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await childDeviceSelectList();
      expect(mockGet).toHaveBeenCalledWith('/device/list', {});
    });
  });

  describe('addChildDevice', () => {
    it('调用 POST /device/son/add', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { parent_id: 'gw1', child_id: 'd1' };
      await addChildDevice(params);
      expect(mockPost).toHaveBeenCalledWith('/device/son/add', params);
    });
  });

  describe('removeChildDevice', () => {
    it('调用 PUT /device/sub-remove', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const params = { parent_id: 'gw1', child_id: 'd1' };
      await removeChildDevice(params);
      expect(mockPut).toHaveBeenCalledWith('/device/sub-remove', params);
    });
  });

  describe('getSimulation', () => {
    it('调用 GET /telemetry/datas/simulation 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getSimulation(params);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/simulation', { params });
    });
  });

  describe('sendSimulation', () => {
    it('调用 POST /telemetry/datas/simulation', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', data: {} };
      await sendSimulation(params);
      expect(mockPost).toHaveBeenCalledWith('/telemetry/datas/simulation', params);
    });
  });

  describe('getSimulationInit', () => {
    it('调用 GET /telemetry/datas/simulation/init 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1' };
      await getSimulationInit(params);
      expect(mockGet).toHaveBeenCalledWith('/telemetry/datas/simulation/init', { params });
    });
  });

  describe('sendSimulationData', () => {
    it('调用 POST /telemetry/datas/simulation/send', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', data: 'test', server: 'localhost', port: 1883, topic: 'test/topic' };
      await sendSimulationData(params);
      expect(mockPost).toHaveBeenCalledWith('/telemetry/datas/simulation/send', params);
    });
  });

  describe('deviceCustomCommandsIdList', () => {
    it('调用 GET /device/model/custom/commands/{paramsId}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await deviceCustomCommandsIdList('d1');
      expect(mockGet).toHaveBeenCalledWith('/device/model/custom/commands/d1');
    });
  });

  describe('deviceProtocolServiceList', () => {
    it('uses the current spelling for GET /service/plugin/select', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { page: 1 };
      await deviceProtocolServiceList(params);
      expect(mockGet).toHaveBeenCalledWith('/service/plugin/select', { params });
    });
  });

  describe('deviceStatusHistory', () => {
    it('调用 GET /device/status/history 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_id: 'd1', page: 1, page_size: 10, start_time: 1000, end_time: 2000, status: 1 };
      await deviceStatusHistory(params);
      expect(mockGet).toHaveBeenCalledWith('/device/status/history', { params });
    });
  });

  describe('deviceDiagnostics', () => {
    it('调用 GET /devices/{deviceId}/diagnostics', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await deviceDiagnostics('d1');
      expect(mockGet).toHaveBeenCalledWith('/devices/d1/diagnostics');
    });
  });

  describe('getTopicMappingList', () => {
    it('调用 GET /device/topic-mappings 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { device_config_id: 'c1', page: 1, page_size: 10 };
      await getTopicMappingList(params);
      expect(mockGet).toHaveBeenCalledWith('/device/topic-mappings', { params });
    });
  });

  describe('createTopicMapping', () => {
    it('调用 POST /device/topic-mappings', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const data = { device_config_id: 'c1', name: '映射1', direction: 'up' as const, source_topic: 'src', target_topic: 'tgt' };
      await createTopicMapping(data);
      expect(mockPost).toHaveBeenCalledWith('/device/topic-mappings', data);
    });
  });

  describe('updateTopicMapping', () => {
    it('调用 PUT /device/topic-mappings/{id}', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const data = { name: '映射2' };
      await updateTopicMapping('tm1', data);
      expect(mockPut).toHaveBeenCalledWith('/device/topic-mappings/tm1', data);
    });

    it('id 为数字时也正确拼接', async () => {
      mockPut.mockResolvedValue({ error: null, data: {} });
      const data = { name: '映射3' };
      await updateTopicMapping(123, data);
      expect(mockPut).toHaveBeenCalledWith('/device/topic-mappings/123', data);
    });
  });

  describe('deleteTopicMapping', () => {
    it('调用 DELETE /device/topic-mappings/{id}', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      await deleteTopicMapping('tm1');
      expect(mockDelete).toHaveBeenCalledWith('/device/topic-mappings/tm1');
    });

    it('id 为数字时也正确拼接', async () => {
      mockDelete.mockResolvedValue({ error: null, data: {} });
      await deleteTopicMapping(456);
      expect(mockDelete).toHaveBeenCalledWith('/device/topic-mappings/456');
    });
  });

  describe('getDeviceDebugStatus', () => {
    it('调用 GET /device/{deviceId}/debug/status', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await getDeviceDebugStatus('d1');
      expect(mockGet).toHaveBeenCalledWith('/device/d1/debug/status');
    });
  });

  describe('getDeviceOnlineStatus', () => {
    it('调用 GET /device/online/status/{deviceId}', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await getDeviceOnlineStatus('d1');
      expect(mockGet).toHaveBeenCalledWith('/device/online/status/d1');
    });
  });

  describe('setDeviceDebugStatus', () => {
    it('调用 POST /device/{deviceId}/debug', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      const data = { enabled: true };
      await setDeviceDebugStatus('d1', data);
      expect(mockPost).toHaveBeenCalledWith('/device/d1/debug', data);
    });
  });

  describe('getDeviceConnectionGuide', () => {
    it('调用 GET /device/{deviceId}/onboarding/connection-guide 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { debug_log_limit: 5, command_log_limit: 3 };
      await getDeviceConnectionGuide('d1', params);
      expect(mockGet).toHaveBeenCalledWith('/device/d1/onboarding/connection-guide', { params });
    });
  });

  describe('getDeviceDebugLogs', () => {
    it('调用 GET /device/{deviceId}/debug/logs 并携带 params', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      const params = { limit: 100, offset: 0 };
      await getDeviceDebugLogs('d1', params);
      expect(mockGet).toHaveBeenCalledWith('/device/d1/debug/logs', { params });
    });

    it('不传 params 时调用 GET /device/{deviceId}/debug/logs', async () => {
      mockGet.mockResolvedValue({ error: null, data: {} });
      await getDeviceDebugLogs('d1');
      expect(mockGet).toHaveBeenCalledWith('/device/d1/debug/logs', { params: undefined });
    });
  });

  describe('device MQTT debug workbench', () => {
    it('maps isolated session and command operations to device-scoped routes', async () => {
      mockPost.mockResolvedValue({ error: null, data: {} });
      mockGet.mockResolvedValue({ error: null, data: {} });
      mockDelete.mockResolvedValue({ error: null, data: {} });

      await openDeviceMQTTDebugSession('device/1');
      await getDeviceMQTTDebugSession('device/1', 'session/1', { after_sequence: 4, limit: 20 });
      await applyDeviceMQTTDebugCommand('device/1', 'session/1', {
        action: 'subscribe',
        topic: 'devices/device/1/telemetry',
        qos: 1
      });
      await closeDeviceMQTTDebugSession('device/1', 'session/1');

      expect(mockPost).toHaveBeenNthCalledWith(1, '/device/device%2F1/mqtt-debug/session');
      expect(mockGet).toHaveBeenCalledWith('/device/device%2F1/mqtt-debug/session/session%2F1', {
        params: { after_sequence: 4, limit: 20 }
      });
      expect(mockPost).toHaveBeenNthCalledWith(
        2,
        '/device/device%2F1/mqtt-debug/session/session%2F1/command',
        { action: 'subscribe', topic: 'devices/device/1/telemetry', qos: 1 }
      );
      expect(mockDelete).toHaveBeenCalledWith('/device/device%2F1/mqtt-debug/session/session%2F1');
    });
  });
});
