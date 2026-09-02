/**
 * 文件用途: 设备域 API wrapper，覆盖设备分组、接入服务、生命周期、配置、遥测、共享和详情页调用。
 * 核心逻辑: 将页面/store 的设备操作统一映射到后端 `/device`、`/service` 等接口，并复用全局 request 实例。
 * 关键注意事项: 参数名、响应 envelope、RDI 设备激活和共享相关接口是前后端契约面，变更前需核对后端路由与自动化测试。
 * 重构建议: 按设备列表、配置、接入、共享/RDI 分域拆分大文件，并保留 index 兼容导出与契约测试。
 */
import type { CustomAxiosRequestConfig } from '@aetherlink/axios'
import { request } from '../request'
export {
  cancelFleetCommandJob,
  createFleetSavedFilter,
  deleteFleetSavedFilter,
  getFleetCommandJob,
  getFleetCommandJobRows,
  getFleetCommandJobSummary,
  getFleetCommandJobSupportBundle,
  listFleetCommandJobs,
  listFleetSavedFilters,
  previewFleetCommandJob,
  retryFleetCommandJob,
  submitFleetCommandJob,
  updateFleetSavedFilter
} from './device-command-jobs-api'
export type {
  CommandJobRowsStatusFilter,
  FleetCommandJobAuditSummary,
  FleetCommandJobEvent,
  FleetCommandJobExecutionChecklistItem,
  FleetCommandJobExecutionSummary,
  FleetCommandJobGovernanceSummary,
  FleetCommandJobListAttentionCounts,
  FleetCommandJobListItem,
  FleetCommandJobListResult,
  FleetCommandJobPayload,
  FleetCommandJobPreviewBlocker,
  FleetCommandJobPreviewPathCounts,
  FleetCommandJobPreviewResult,
  FleetCommandJobPreviewRow,
  FleetCommandJobProgressHealth,
  FleetCommandJobRowsResult,
  FleetCommandJobPreviewDevice,
  FleetCommandJobSubmitResult,
  FleetCommandJobSubmitRow,
  FleetCommandJobSupportBundle,
  FleetCommandJobSupportDiagnostic,
  FleetCommandJobSupportDevice,
  FleetSavedFilterItem,
  FleetSavedFilterListResult,
  FleetSavedFilterPayload
} from './device-command-jobs-api'
export {
  applyDeviceMQTTDebugCommand,
  closeDeviceMQTTDebugSession,
  getDeviceConnectionDiagnostics,
  getDeviceConnectionGuide,
  getDeviceDebugLogs,
  getDeviceDebugStatus,
  getDeviceMQTTDebugSession,
  openDeviceMQTTDebugSession,
  setDeviceDebug,
  setDeviceDebugStatus
} from './device-onboarding-api'
export type {
  DeviceConnectionGuideQuery,
  DeviceConnectionGuideResponse,
  DeviceDebugLogEntry,
  DeviceDebugLogsResponse,
  DeviceDebugStatus,
  DeviceMQTTDebugAction,
  DeviceMQTTDebugCommand,
  DeviceMQTTDebugMessage,
  DeviceMQTTDebugSnapshot,
  DeviceMQTTDebugSubscription
} from './device-onboarding-api'
export {
  deviceMapTelemetry,
  getDeviceTwin,
  getSimulation,
  getSimulationInit,
  getTelemetryLogList,
  sendSimulation,
  sendSimulationData,
  setDeviceTwinDesired,
  telemetryDataCurrent,
  telemetryDataCurrentKeys,
  telemetryDataDel,
  telemetryDataHistoryList,
  telemetryDataPub,
  telemetryHistoryData
} from './device-telemetry-twin-api'
export type {
  DeviceTwinDesiredPayload,
  DeviceTwinRow,
  DeviceTwinSource,
  DeviceTwinState,
  DeviceTwinSummary
} from './device-telemetry-twin-api'

/** 获取设备分组 */
export const getDeviceGroup = async (params: object) => {
  return await request.get('/device/group', { params })
}

/** 接入方式下拉菜单：原始协议服务字典接口 */
export const deviceDictProtocolService = async (params: CustomAxiosRequestConfig & Record<string, unknown>) => {
  return await request.get<DeviceManagement.TreeStructure>('/dict/protocol/service', params)
}
/** 接入方式下拉一级菜单 */
export const deviceDictProtocolServiceFirstLevel = async (params: CustomAxiosRequestConfig & Record<string, unknown>) => {
  return await request.get<DeviceManagement.ProtocolAndService>('/service/plugin/select', params)
}
/** 接入方式下拉二级菜单 */
export const deviceDictProtocolServiceSecondLevel = async (params: CustomAxiosRequestConfig & Record<string, unknown>) => {
  return await request.get<DeviceManagement.ServiceList>('/service/access/list', params)
}

/** 获取设备分组树 */
export const deviceGroupTree = async (params: CustomAxiosRequestConfig & Record<string, unknown>) => {
  return await request.get<DeviceManagement.TreeStructure>('/device/group/tree', params)
}
/** 新增设备分组 */
export const deviceGroup = async (params: { id: string; parent_id: string; name: string; description: string }) => {
  return await request.post<Api.BaseApi.Data>('/device/group', params)
}

/** 修改设备分组 */
export const putDeviceGroup = async (params: { id: string; parent_id: string; name: string; description: string }) => {
  return await request.put<Api.BaseApi.Data>('/device/group', params)
}

/** 激活设备 */
export const putDeviceActive = async (params: object) => {
  return await request.put<Api.BaseApi.Data>('/device/active', params)
}

/** 删除设备分组 */
export const deleteDeviceGroup = async (params: { id: string }) => {
  return await request.delete<Api.BaseApi.Data>(`/device/group/${params.id}`)
}

/** 获取设备分详情 */
export const deviceGroupDetail = async (params: { id: string }) => {
  return await request.get<DeviceManagement.DetailData>(`/device/group/detail/${params.id}`)
}

/** 获取设备列表 */
export const deviceList = async (params: object) => {
  return await request.get<DeviceManagement.DeviceDatas>(`/device`, {
    params
  })
}

const updateDeviceConfigBinding = async (params: object) => {
  return await request.put<DeviceManagement.DeviceDatas>(`/device/update/config`, params)
}

/** Unbind a device from its device configuration. */
export const detachDeviceFromConfig = async (params: object) => {
  return await updateDeviceConfigBinding(params)
}
/** 获取设备列表 */
export const deviceListByGroup = async (params: object) => {
  return await request.get<DeviceManagement.DeviceDatas>(`/device/group/relation/list`, {
    params
  })
}

/** 获取设备详情 */
export const deviceDetail = async (id: string) => {
  const url = `/device/detail/${id}`
  return await request.get<DeviceManagement.DeviceDetail>(url)
}

/** 获取设备分组关系 */
export const deviceGroupRelation = async (params: object) => {
  return await request.post<Api.BaseApi.Data>(`/device/group/relation`, params)
}

export const getDeviceGroupRelation = async (params: object) => {
  return await request.get(`/device/group/relation`, { params })
}

/** 获取设备告警状态 */
export const deviceAlarmStatus = async (params: object) => {
  return await request.get(`/alarm/info/history/device`, { params })
}

/** 获取设备告警历史 */
export const deviceAlarmHistory = async (params: object) => {
  return await request.get(`/alarm/info/history`, { params })
}

/** 获取设备告警配置列表 */
export const deviceAlarmList = async (params: object) => {
  return await request.get(`/scene_automations/alarm`, { params })
}

/** 修改设备告警描述 */
export const deviceAlarmHistoryPut = async (params: object) => {
  return await request.put(`/alarm/info/history`, params)
}

/** 获取设备物模型列表 */
export const deviceTemplate = async (params: object) => {
  return await request.get<DeviceManagement.TemplateDatas>(`/device/template`, {
    params
  })
}

/** 创建设备物模型 */
export const deviceTemplateAdd = async (params: object) => {
  return await request.post<Api.BaseApi.Data>(`/device/template`, params)
}

/** 获取服务列表 */
export const getServiceList = async (params: object) => {
  return await request.get<DeviceManagement.ServiceList>(`/service/list`, {
    params
  })
}

/** 获取设备物模型列表 */
export const deviceTemplateDetail = async (params: { id: string }) => {
  return await request.get<DeviceManagement.TemplateDetailData>(`/device/template/detail/${params.id}`)
}

/** 获取设备配置列表 */
export const deviceConfig = async (params: object) => {
  return await request.get<DeviceManagement.ConfigDatas>(`/device_config`, {
    params
  })
}

/** 创建设备配置 */
export const deviceConfigAdd = async (params: object) => {
  return await request.post<Api.BaseApi.Data>(`/device_config`, params)
}

/** 更新设备配置 */
export const deviceConfigEdit = async (params: object) => {
  return await request.put<Api.BaseApi.Data>(`/device_config`, params)
}

/** 获取设备配置 */
export const deviceConfigInfo = async (params: { id: string }) => {
  return await request.get<DeviceManagement.ConfigData>(`/device_config/${params.id}`)
}
/** 删除设备配置 */
export const deviceConfigDel = async (params: { id: string }) => {
  return await request.delete<Api.BaseApi.Data>(`/device_config/${params.id}`)
}
/** 设备配置-凭证类型下拉 */
export const deviceConfigVoucherType = async (params: object) => {
  return await request.get<Api.BaseApi.Data>(`/device_config/voucher_type`, { params })
}
/** 设备配置-获取设备配置表单 */
export const protocolPluginConfigForm = async (params: object) => {
  return await request.get(`/protocol_plugin/config_form`, { params })
}
/** 批量新设备配置关联的设备 */
export const deviceConfigBatch = async (params: object) => {
  return await request.put<Api.BaseApi.Data>(`/device_config/batch`, params)
}

/** 获取设备列表 */
export const deleteDeviceGroupRelation = async (params: object) => {
  return await request.delete2<Api.BaseApi.Data>(`/device/group/relation`, params)
}

/** 获取设备连接信息 */
export const getDeviceConnectInfo = async (params: object) => {
  return await request.get<Record<string, unknown>>(`/device/connect/info`, {
    params
  })
}

/** 获取设备连接信息 */
export const getPlugininfoByService = async (params: object) => {
  return await request.get<Api.BaseApi.Data>(`/service/plugin/info`, {
    params
  })
}

/** 获取设备配置列表 */
export const getDeviceConfigList = async (params: object) => {
  return await request.get<DeviceManagement.ConfigDatas>(`/device_config`, {
    params
  })
}

/** 更新设备凭证 */
export const updateDeviceVoucher = async (params: object) => {
  return await request.post(`/device/update/voucher`, params)
}
export const deviceAdd = async (params: object) => {
  return await request.post(`/device`, params)
}

export const deviceConnectForm = async (params: object) => {
  return await request.get(`/device/connect/form`, { params })
}

export const checkDevice = async (deviceNumber: string) => {
  const url = `/device/check/${encodeURIComponent(deviceNumber)}`
  return await request.get(url)
}
export const deleteDevice = async (params: { id: string }) => {
  return await request.delete<Api.BaseApi.Data>(`/device/${params.id}`)
}

export const setDeviceScriptEnable = async (params: object) => {
  return await request.put(`/data_script/enable`, params)
}

/** 获取数据处理列表 */
export const getDataScriptList = async (params: object) => {
  return await request.get<DeviceManagement.DataScriptDatas>(`/data_script`, {
    params
  })
}

/** 创建数据处理 */
export const dataScriptAdd = async (params: object) => {
  return await request.post<Api.BaseApi.Data>(`/data_script`, params)
}

/** 更新数据处理 */
export const dataScriptEdit = async (params: object) => {
  return await request.put<Api.BaseApi.Data>(`/data_script`, params)
}

/** 调试数据处理 */
export const dataScriptQuiz = async (params: object) => {
  return await request.post<Api.BaseApi.Data>(`/data_script/quiz`, params, { needMessage: true })
}
/** 删除数据处理 */
export const dataScriptDel = async (params: { id: string }) => {
  return await request.delete<Api.BaseApi.Data>(`/data_script/${params.id}`)
}

/** 新增期望消息 */
export const expectMessageAdd = async (params: object) => {
  return await request.post(`/expected/data`, params)
}
/** 期望消息列表 */
export const expectMessageList = async (params: object) => {
  return await request.get(`/expected/data/list`, { params })
}

/** 期望消息删除 */
export const expectMessageDelete = async (params: string | number) => {
  return await request.delete(`/expected/data/${params}`)
}

/** 设备影子消息列表（可按 status 过滤） */
export const deviceShadowList = async (deviceId: string, params?: object) => {
  return await request.get(`/device/shadow/${deviceId}`, { params })
}
/** 设置设备影子消息：设备在线直接下发，离线写入缓存队列 */
export const deviceShadowSet = async (deviceId: string, params: object) => {
  return await request.post(`/device/shadow/${deviceId}`, params)
}
/** 取消待投递的影子消息 */
export const deviceShadowCancel = async (deviceId: string, msgId: string) => {
  return await request.delete(`/device/shadow/${deviceId}/${msgId}`)
}

/** 读取设备 Modbus 点表 */
export const getModbusProfile = async (deviceId: string) => {
  return await request.get(`/device/modbus/profile/${deviceId}`)
}
/** 保存设备 Modbus 点表（body 为 {profile} 包装） */
export const saveModbusProfile = async (deviceId: string, profile: object) => {
  return await request.put(`/device/modbus/profile/${deviceId}`, { profile })
}
/** 属性集查询：载荷契约固定为 { device_id }；历史上曾因裸字符串调用实际请求 /attribute/datas/undefined。 */
export const getAttributeDataSet = async (
  params: { device_id: string | number },
  requestConfig: CustomAxiosRequestConfig = {}
) => {
  return await request.get(`/attribute/datas/${params.device_id}`, requestConfig)
}

export const deleteAttributeDataSet = async (params: string | number) => {
  return await request.delete(`/attribute/datas/${params}`)
}

/** 属性下发记录查询（分页） */
export const getAttributeDataSetLogs = async (params: object) => {
  return await request.get(`/attribute/datas/set/logs`, { params })
}

/** 下发属性 */
export const attributeDataPub = async (params: object) => {
  return await request.post(`/attribute/datas/pub`, params)
}

/**
 * @param params {device_id:string,key:string}
 * @returns
 */
export const getAttributeDatasKey = async (params: { device_id: string; key: string }) => {
  return await request.get('/attribute/datas/key', { params })
}

/** 属性下发记录查询（分页） */
export const getEventDataSet = async (params: object) => {
  return await request.get(`/event/datas`, { params })
}

/** 属性下发记录查询（分页） */
export const getCommandDataSetLogs = async (params: object) => {
  return await request.get(`/command/datas/set/logs`, { params })
}

export type CommandDeliveryDiagnostics = {
  device_id: string
  evaluated_at: string
  is_online: boolean
  device_status: number
  latest_log?: {
    id: string
    message_id: string
    identify: string
    status: string
    status_label: string
    data?: string
    response_data?: string
    error_message?: string
    created_at: string
  }
  recent_logs: Array<{
    id: string
    message_id: string
    identify: string
    status: string
    status_label: string
    error_message?: string
    created_at: string
  }>
  confirmation_channels: Array<{
    code: string
    label: string
    description: string
  }>
  conclusion: {
    level: string
    code: string
    summary: string
    next_actions: string[]
    evidence?: string[]
  }
}

/** 命令下发确认诊断：只读聚合最近日志、在线态和下一步 */
export const getCommandDeliveryDiagnostics = async (deviceId: string, params?: { limit?: number }) => {
  return await request.get<CommandDeliveryDiagnostics>(`/command/datas/delivery/diagnostics/${deviceId}`, { params })
}

/** 下发命令 */
export const commandDataPub = async (params: object) => {
  return await request.post(`/command/datas/pub`, params)
}

export type DirectMethodCommandRequest = {
  device_id: string
  identify: string
  value?: string | null
  timeout_seconds?: number
}

export type DirectMethodResult = {
  message_id: string
  device_id: string
  identify: string
  status: string
  outcome: 'awaiting_response' | 'device_succeeded' | 'device_failed' | 'delivery_failed' | 'timeout'
  published: boolean
  log_recorded: boolean
  device_responded: boolean
  device_succeeded: boolean
  timed_out: boolean
  response_payload?: string
  error_message?: string
  timeout_seconds: number
  elapsed_ms: number
}

/** 即时在线命令：发布后短暂等待同一 message_id 的设备响应。 */
export const invokeDirectMethod = async (params: DirectMethodCommandRequest) => {
  return await request.post<DirectMethodResult>(`/command/datas/direct-method`, params)
}

/** 命令标识符下拉菜单 */
export const commandDataById = async (id: string) => {
  const url = `/command/datas/${id}`
  return await request.get<DeviceManagement.telemetryCurrent>(url)
}

/** 有图表的设备list */
export const deviceTemplateSelect = async () => {
  const url = `/device/template/chart/select`
  return await request.get(url)
}

export const deviceUpdateConfig = async (params: object) => {
  return await updateDeviceConfigBinding(params)
}

export const deviceConfigMenu = async (params: object) => {
  return await request.get(`/device/template/menu`, { params })
}

// 保存设备位置
export const deviceLocation = async (params: object) => {
  return await request.put(`/device`, params)
}
/** 修改设备名称 */
export const deviceUpdate = async (params: object) => {
  return await request.put<Api.BaseApi.Data>('/device', params)
}
/** 网关下子设备列表 */
export const childDeviceTableList = async (params: { id: string; page?: number; page_size?: number }) => {
  return await request.get(`/device/sub-list/${params.id}`, {
    params
  })
}
/** 添加子设备选择列表 */
export const childDeviceSelectList = async () => {
  return await request.get(`/device/list`, {})
}
/** 添加子设备 */
export const addChildDevice = async (params: object) => {
  return await request.post(`/device/son/add`, params)
}
/** 移除子设备 */
export const removeChildDevice = async (params: object) => {
  return await request.put(`/device/sub-remove`, params)
}
// 根据设备id查自定义命令列表
export const deviceCustomCommandsIdList = async (paramsId: string) => {
  return await request.get(`/device/model/custom/commands/${paramsId}`)
}

export const deviceProtocolServiceList = async (params: object) => {
  return await request.get(`/service/plugin/select`, { params })
}

/** 获取设备状态历史记录 */
export const deviceStatusHistory = async (params: {
  device_id: string
  page: number
  page_size: number
  start_time?: number
  end_time?: number
  status?: number
}) => {
  return await request.get(`/device/status/history`, { params })
}

/** 获取设备诊断信息 */
export const deviceDiagnostics = async (deviceId: string) => {
  return await request.get(`/devices/${deviceId}/diagnostics`)
}

/** 获取设备接入向导使用的只读连接诊断聚合 */
export interface TopicMappingPayload {
  device_config_id: string
  name: string
  direction: 'up' | 'down'
  source_topic: string
  target_topic: string
  data_identifier?: string
  description?: string
  priority?: number
  enabled?: boolean
}

export interface TopicMappingDryRunPayload {
  device_config_id: string
  direction: 'up' | 'down'
  source_topic: string
  target_topic: string
  test_topic?: string
  sample_topic: string
  data_identifier?: string
}

export interface TopicMappingDryRunDiagnostic {
  severity: 'success' | 'info' | 'warning' | 'error'
  scope: string
  message: string
}

export interface TopicMappingDryRunResult {
  matched: boolean
  direction: 'up' | 'down'
  source_topic: string
  target_topic: string
  test_topic?: string
  sample_topic: string
  resolved_topic: string
  data_identifier?: string
  diagnostics: TopicMappingDryRunDiagnostic[]
  next_steps: string[]
}

export const getTopicMappingList = async (params: { device_config_id: string; page?: number; page_size?: number }) => {
  return await request.get(`/device/topic-mappings`, { params })
}

export const createTopicMapping = async (data: TopicMappingPayload) => {
  return await request.post(`/device/topic-mappings`, data)
}

export const dryRunTopicMapping = async (data: TopicMappingDryRunPayload) => {
  return await request.post<TopicMappingDryRunResult>(`/device/topic-mappings/dry-run`, data, { silentError: true })
}

export const updateTopicMapping = async (id: string | number, data: Partial<TopicMappingPayload>) => {
  return await request.put(`/device/topic-mappings/${id}`, data)
}

export const deleteTopicMapping = async (id: string | number) => {
  return await request.delete(`/device/topic-mappings/${id}`)
}

/** 设备调试日志开关状态查询 */

/** 设备在线状态查询 */
export const getDeviceOnlineStatus = async (deviceId: string) => {
  return await request.get(`/device/online/status/${deviceId}`)
}

/** 设备调试日志开关 */

/** 设备调试日志查询 */
