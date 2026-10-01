/**
 * 文件用途: 设备域 API 聚合入口，统一对外导出设备列表、配置、接入控制、共享/RDI、命令任务、
 * 接入向导调试、遥测双胞胎等子域 wrapper，保持历史导入路径 `@/service/api/device` 兼容。
 * 核心逻辑: 本文件自身不再直接实现任何请求，只做 re-export；具体实现按域拆分到同目录下的
 * device-list-api / device-config-api / device-access-api / device-share-rdi-api 等模块。
 * 关键注意事项: 新增设备相关接口时，请放入对应领域文件而不是本文件；本文件的导出名/类型名
 * 不可随意变更，大量页面通过 `@/service/api` 或 `@/service/api/device` 直接导入这些符号。
 */
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

export {
  addChildDevice,
  checkDevice,
  childDeviceSelectList,
  childDeviceTableList,
  deleteDevice,
  deleteDeviceGroupRelation,
  deviceAdd,
  deviceConnectForm,
  deviceCustomCommandsIdList,
  deviceDetail,
  deviceDiagnostics,
  deviceDictProtocolService,
  deviceDictProtocolServiceFirstLevel,
  deviceDictProtocolServiceSecondLevel,
  deviceGroup,
  deviceGroupDetail,
  deviceGroupRelation,
  deviceGroupTree,
  deviceList,
  deviceListByGroup,
  deviceLocation,
  deviceProtocolServiceList,
  deviceStatusHistory,
  deviceUpdate,
  getDeviceConnectInfo,
  getDeviceGroup,
  getDeviceGroupRelation,
  getDeviceOnlineStatus,
  getPlugininfoByService,
  putDeviceActive,
  putDeviceGroup,
  removeChildDevice,
  deleteDeviceGroup
} from './device-list-api'

export {
  attributeDataPub,
  commandDataById,
  commandDataPub,
  createTopicMapping,
  dataScriptAdd,
  dataScriptDel,
  dataScriptEdit,
  dataScriptQuiz,
  deleteAttributeDataSet,
  deleteTopicMapping,
  detachDeviceFromConfig,
  deviceAlarmHistory,
  deviceAlarmHistoryPut,
  deviceAlarmList,
  deviceAlarmStatus,
  deviceConfig,
  deviceConfigAdd,
  deviceConfigBatch,
  deviceConfigDel,
  deviceConfigEdit,
  deviceConfigInfo,
  deviceConfigMenu,
  deviceConfigVoucherType,
  deviceTemplate,
  deviceTemplateAdd,
  deviceTemplateDetail,
  deviceTemplateSelect,
  deviceUpdateConfig,
  dryRunTopicMapping,
  expectMessageAdd,
  expectMessageDelete,
  expectMessageList,
  getAttributeDataSet,
  getAttributeDataSetLogs,
  getAttributeDatasKey,
  getCommandDataSetLogs,
  getCommandDeliveryDiagnostics,
  getDataScriptList,
  getDeviceConfigList,
  getEventDataSet,
  getModbusProfile,
  getServiceList,
  getTopicMappingList,
  invokeDirectMethod,
  protocolPluginConfigForm,
  saveModbusProfile,
  setDeviceScriptEnable,
  updateDeviceVoucher,
  updateTopicMapping
} from './device-config-api'
export type {
  CommandDeliveryDiagnostics,
  DirectMethodCommandRequest,
  DirectMethodResult,
  TopicMappingDryRunDiagnostic,
  TopicMappingDryRunPayload,
  TopicMappingDryRunResult,
  TopicMappingPayload
} from './device-config-api'

export {
  issueDeviceClaimToken,
  listDeviceClaimTokens,
  redeemDeviceClaim,
  revokeDeviceClaimToken
} from './device-access-api'

export { deviceShadowCancel, deviceShadowList, deviceShadowSet } from './device-share-rdi-api'
export type { DeviceShadowCommandParams } from './device-share-rdi-api'
