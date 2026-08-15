/**
 * 文件说明：
 * - 承接 ThingsVisWidget 中 `thingsvis:requestFieldData` 的宿主桥接逻辑。
 * - 负责可信消息解析、目标设备约束、字段响应拼装与 `tv:platform-data` 回推前的数据装配。
 * 维护提示：
 * - 这里是 widget guest 拉取平台字段的主入口，任何 payload 兼容、设备绑定或空响应策略调整都要谨慎。
 * - 该模块依赖 `widgetFieldDataBridge.ts` 与历史字段桥提供的能力，本身不直接维护历史扫描或写回逻辑。
 */
import { resolvePlatformFieldReadRequest } from '@/components/thingsvis/fieldReadRequestBridge'
import { parseTrustedGuestMessage } from '@/components/thingsvis/hostBridge'
import {
  buildWidgetFieldDataResponseFields,
  type FieldDataRequestPayload,
  type HistoryRequestConfig,
  type ResolvedFieldDataRequest
} from '@/components/thingsvis/widgetFieldDataBridge'

type TelemetryHistoryRow = {
  value: unknown
  ts: number
}

type ThingsVisWidgetFieldRequestBridgeOptions = {
  isTrustedMessageEvent: (event: MessageEvent) => boolean
  getPreviewDeviceId: () => string | undefined
  historyFieldSuffix: string
  templateDeviceId: string
  alarmStatusFieldIds: Set<string>
  getCurrentData: () => Record<string, unknown> | undefined
  collectConfiguredHistoryFields: (dataSourceId?: string) => Map<string, string>
  shouldPrefillHistoryForDataSource: (dataSourceId?: string) => boolean
  fetchTelemetryHistoryField: (
    deviceId: string,
    fieldId: string,
    config?: HistoryRequestConfig
  ) => Promise<TelemetryHistoryRow[]>
  pushPlatformFieldHistory: (fieldId: string, history: TelemetryHistoryRow[], deviceId?: string) => void
  loadAlarmStatus: (deviceId: string) => Promise<any>
  pushPlatformFieldData: (fields: Record<string, unknown>, deviceId?: string) => void
}

const resolveFieldDataDeviceScope = (
  getPreviewDeviceId: () => string | undefined,
  payload?: FieldDataRequestPayload
) => {
  const previewDeviceId = getPreviewDeviceId()
  if (payload?.deviceId && previewDeviceId && payload.deviceId !== previewDeviceId) return null
  return payload?.deviceId || previewDeviceId
}

const resolveFieldDataRequest = (
  getPreviewDeviceId: () => string | undefined,
  payload?: FieldDataRequestPayload
): ResolvedFieldDataRequest | null => {
  const resolved = resolvePlatformFieldReadRequest({
    payload,
    resolveTargetDeviceId: (currentPayload) => resolveFieldDataDeviceScope(getPreviewDeviceId, currentPayload)
  })
  if (!resolved) return null
  return {
    payload: resolved.payload,
    fieldIds: resolved.fieldIds,
    targetDeviceId: resolved.targetDeviceId
  }
}

const buildFieldDataResponseFields = async (
  request: ResolvedFieldDataRequest,
  options: Omit<
    ThingsVisWidgetFieldRequestBridgeOptions,
    'isTrustedMessageEvent' | 'getPreviewDeviceId' | 'pushPlatformFieldData'
  >
) => {
  // 字段读取既可能返回实时值，也可能顺带补告警派生字段与历史序列。
  return buildWidgetFieldDataResponseFields(request, {
    historyFieldSuffix: options.historyFieldSuffix,
    templateDeviceId: options.templateDeviceId,
    alarmStatusFieldIds: options.alarmStatusFieldIds,
    currentData: options.getCurrentData(),
    collectConfiguredHistoryFields: options.collectConfiguredHistoryFields,
    shouldPrefillHistoryForDataSource: options.shouldPrefillHistoryForDataSource,
    fetchTelemetryHistoryField: options.fetchTelemetryHistoryField,
    pushPlatformFieldHistory: options.pushPlatformFieldHistory,
    loadAlarmStatus: options.loadAlarmStatus
  })
}

export const createThingsVisWidgetFieldRequestHandler = (options: ThingsVisWidgetFieldRequestBridgeOptions) => {
  return async (event: MessageEvent) => {
    const message = parseTrustedGuestMessage<FieldDataRequestPayload>(
      event,
      {
        isTrusted: options.isTrustedMessageEvent
      },
      'thingsvis:requestFieldData'
    )
    if (!message) return

    const request = resolveFieldDataRequest(options.getPreviewDeviceId, message.payload)
    if (!request) return

    const fields = await buildFieldDataResponseFields(request, options)
    if (Object.keys(fields).length === 0) return

    options.pushPlatformFieldData(fields, request.targetDeviceId)
  }
}
