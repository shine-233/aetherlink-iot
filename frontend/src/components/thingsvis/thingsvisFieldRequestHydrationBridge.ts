/**
 * 文件说明：
 * - 编排 ThingsVis guest 发起的字段读取请求，并复用平台字段补水逻辑回推 `tv:platform-data`。
 * - 将字段请求解析、设备绑定解析、当前值/告警/RDI 元数据读取和瞬时错误静默策略从 AppFrame 中拆出。
 * 维护提示：
 * - `resolvePlatformFieldReadRequest` 的 `null` / `undefined` 语义不能改：`null` 表示拒绝请求，`undefined` 表示走默认设备。
 * - 本模块不直接持有 iframe/window/origin，真实 postMessage 由 AppFrame 注入，避免放宽跨域通信边界。
 */
import { resolvePlatformFieldReadRequest } from '@/components/thingsvis/fieldReadRequestBridge'
import { buildRequestedFieldData } from '@/components/thingsvis/thingsvisFieldDataBridge'
import { hydratePlatformSourceDescriptorGroup } from '@/components/thingsvis/thingsvisFieldHydrationBridge'
import { buildThingsVisHostErrorPayload } from '@/components/thingsvis/thingsvisHostErrorPayload'

const DEVICE_ALARM_STATUS_FIELD_IDS = new Set([
  'device_alarm_active',
  'device_alarm_count',
  'device_alarm_highest_level',
  'latest_device_alarm_title',
  'latest_device_alarm_level',
  'latest_device_alarm_time'
])
const RDI_DEVICE_META_FIELD_IDS = new Set(['pid_number', 'firmware_version', 'description', 'shared_status'])
const SILENT_REQUEST_CONFIG = { silentError: true } as const

export type FieldRequestBinding = {
  deviceId?: string | null
  dataSourceId?: string
}

export type FieldRequestHydrationDependencies = {
  loadTelemetryCurrent: (deviceId: string, requestConfig: unknown) => Promise<any>
  loadAttributeDataSet: (params: { device_id: string }, requestConfig: unknown) => Promise<any>
  loadRdiDeviceConfig: (deviceId: string, requestConfig: unknown) => Promise<any>
  loadDeviceAlarmStatus: (params: { device_id: string; page: number; page_size: number }) => Promise<any>
}

export type ThingsVisFieldRequestHydrationBridgeOptions = {
  isFrameReady: () => boolean
  resolveBindingFromPayload: (payload: Record<string, unknown> | undefined) => FieldRequestBinding
  ensureDevice: (deviceId?: string) => void
  ensureDeviceWs: (deviceId?: string) => void
  ensureDeviceStatusWs: (deviceId?: string) => void
  postPlatformData: (fields: Record<string, unknown>, deviceId?: string, dataSourceId?: string) => void
  postToThingsVis?: (type: string, payload: Record<string, unknown>) => void
  dependencies: FieldRequestHydrationDependencies
  onAlarmError?: (deviceId: string, error: unknown) => void
}

export function createThingsVisFieldRequestHydrationBridge(options: ThingsVisFieldRequestHydrationBridgeOptions) {
  async function loadRequestedFieldData(
    fieldIds: unknown[],
    deviceId?: string
  ): Promise<Record<string, unknown>> {
    return buildRequestedFieldData({
      fieldIds,
      deviceId,
      alarmStatusFieldIds: DEVICE_ALARM_STATUS_FIELD_IDS,
      rdiMetaFieldIds: RDI_DEVICE_META_FIELD_IDS,
      historyFieldSuffix: '__history',
      silentRequestConfig: SILENT_REQUEST_CONFIG,
      loadTelemetryCurrent: options.dependencies.loadTelemetryCurrent,
      loadAttributeDataSet: options.dependencies.loadAttributeDataSet,
      loadRdiDeviceConfig: options.dependencies.loadRdiDeviceConfig,
      loadDeviceAlarmStatus: options.dependencies.loadDeviceAlarmStatus,
      onAlarmError: options.onAlarmError
    })
  }

  async function handleFieldDataRequest(payload: Record<string, unknown>) {
    if (!options.isFrameReady()) return

    const binding = options.resolveBindingFromPayload(payload)
    const { dataSourceId } = binding
    let deviceId = binding.deviceId || undefined

    try {
      const request = resolvePlatformFieldReadRequest({
        payload,
        resolveTargetDeviceId: (nextPayload) => options.resolveBindingFromPayload(nextPayload).deviceId
      })
      if (!request) return

      const { fieldIds, targetDeviceId } = request
      deviceId = targetDeviceId

      options.ensureDevice(deviceId)
      await hydratePlatformSourceDescriptorGroup({
        deviceId: deviceId || '',
        group: {
          descriptors: [{ id: dataSourceId || '', deviceId, requestedFields: fieldIds }],
          requestedFields: new Set(fieldIds)
        },
        ensureDeviceWs: options.ensureDeviceWs,
        ensureDeviceStatusWs: options.ensureDeviceStatusWs,
        loadRequestedFieldData: (requestedFieldIds, targetDeviceId) =>
          loadRequestedFieldData(requestedFieldIds, targetDeviceId),
        postPlatformData: options.postPlatformData
      })
    } catch (error) {
      options.postToThingsVis?.('tv:platform-data', {
        dataSourceId,
        deviceId,
        fields: {},
        ...buildThingsVisHostErrorPayload('field_data', error)
      })
    }
  }

  return {
    loadRequestedFieldData,
    handleFieldDataRequest
  }
}
