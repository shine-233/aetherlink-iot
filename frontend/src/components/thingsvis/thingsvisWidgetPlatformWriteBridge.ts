/**
 * 文件说明：
 * - 承接 ThingsVisWidget 中 `tv:platform-write` 的宿主桥接逻辑。
 * - 负责可信消息解析、目标设备约束、字段类型归一化后的写入发布，以及写回结果回传。
 * 维护提示：
 * - 这里属于 iframe guest 与 AetherLink 平台 API 的跨系统契约层，`message type` 和回包结构要保持稳定。
 * - 任何新的写入类型都应优先扩展这里，而不是重新塞回 `ThingsVisWidget.vue` 主文件。
 */
import { parseTrustedGuestMessage, type TrustedGuestMessage } from '@/components/thingsvis/hostBridge'

type PlatformWriteMessagePayload = {
  dataSourceId?: string
  data?: unknown
  deviceId?: string
}

type PlatformWriteRequest = {
  requestId?: string
  dataSourceId: string
  data: unknown
  deviceId?: string
}

type PlatformWritePayload = {
  normalizedData: unknown
  fieldId?: string
  fieldType?: string
  valueStr: string
}

type PlatformWritePublishResult = { success: true; result: any } | { success: false; error: string }

type PostGuestMessage = (message: Record<string, unknown>) => void

type PlatformWriteBridgeOptions = {
  isTrustedMessageEvent: (event: MessageEvent) => boolean
  postMessageToGuest: PostGuestMessage
  getPreviewDeviceId: () => string | undefined
  getFieldDataTypeMap: () => Record<string, string>
  normalizeWriteData: (dataSourceId: string, data: unknown) => unknown
  publishAttributeData: (payload: { device_id: string; value: string }) => Promise<any>
  publishCommandData: (payload: { device_id: string; identify: string; value: string }) => Promise<any>
  publishTelemetryData: (payload: { device_id: string; value: string }) => Promise<any>
}

type TrustedThingsVisWidgetMessage<TPayload extends Record<string, unknown> = Record<string, unknown>> =
  TrustedGuestMessage<TPayload>

const postPlatformWriteResult = (
  requestId: string | undefined,
  payload: Record<string, unknown>,
  postMessageToGuest: PostGuestMessage
) => {
  if (!requestId) return

  postMessageToGuest({
    type: 'tv:platform-write-result',
    requestId,
    ...payload
  })
}

const postPlatformWriteSuccess = (
  requestId: string | undefined,
  echo: unknown,
  postMessageToGuest: PostGuestMessage
) => {
  postPlatformWriteResult(
    requestId,
    {
      success: true,
      echo
    },
    postMessageToGuest
  )
}

const postPlatformWriteError = (
  requestId: string | undefined,
  error: string,
  postMessageToGuest: PostGuestMessage
) => {
  postPlatformWriteResult(
    requestId,
    {
      success: false,
      error
    },
    postMessageToGuest
  )
}

const resolveWritableDeviceId = (
  getPreviewDeviceId: () => string | undefined,
  requestedDeviceId?: string
) => {
  const previewDeviceId = getPreviewDeviceId()
  const normalizedRequestedDeviceId = typeof requestedDeviceId === 'string' ? requestedDeviceId.trim() : ''

  if (!previewDeviceId) {
    return { error: 'Missing deviceId' } as const
  }

  if (normalizedRequestedDeviceId && normalizedRequestedDeviceId !== previewDeviceId) {
    return { error: 'Device mismatch' } as const
  }

  return { deviceId: previewDeviceId } as const
}

const normalizePlatformWriteRequest = (
  message: TrustedThingsVisWidgetMessage<PlatformWriteMessagePayload>
): PlatformWriteRequest | { requestId?: string; error: string } => {
  const { requestId, payload: writePayload } = message
  const { dataSourceId, data, deviceId } = writePayload ?? {}

  if (!dataSourceId || data === undefined) {
    return {
      requestId,
      error: 'Missing dataSourceId or data'
    }
  }

  return {
    requestId,
    dataSourceId,
    data,
    deviceId
  }
}

const resolveCommandWrite = (data: unknown, fieldId?: string): { identify: string; value: string } | null => {
  if (!fieldId || data === null || typeof data !== 'object' || Array.isArray(data)) return null
  const params = (data as Record<string, unknown>)[fieldId]
  return {
    identify: fieldId,
    value: JSON.stringify(params ?? {})
  }
}

const buildPlatformWritePayload = (
  dataSourceId: string,
  data: unknown,
  getFieldDataTypeMap: () => Record<string, string>,
  normalizeWriteData: (dataSourceId: string, data: unknown) => unknown
): PlatformWritePayload => {
  const normalizedData = normalizeWriteData(dataSourceId, data)
  const dataObject =
    normalizedData !== null && typeof normalizedData === 'object' ? (normalizedData as Record<string, unknown>) : null
  const fieldEntries = dataObject ? Object.entries(dataObject) : []
  const fieldId = fieldEntries.length === 1 ? fieldEntries[0]?.[0] : undefined
  const fieldTypeMap = getFieldDataTypeMap()
  const fieldType = fieldId ? fieldTypeMap[fieldId] : undefined
  // 平台写接口最终都吃字符串，因此这里统一完成序列化。
  const valueStr = typeof normalizedData === 'string' ? normalizedData : JSON.stringify(normalizedData)

  return {
    normalizedData,
    fieldId,
    fieldType,
    valueStr
  }
}

const publishPlatformWrite = async (
  deviceId: string,
  payload: PlatformWritePayload,
  options: Pick<
    PlatformWriteBridgeOptions,
    'publishAttributeData' | 'publishCommandData' | 'publishTelemetryData'
  >
): Promise<PlatformWritePublishResult> => {
  if (payload.fieldType === 'attribute') {
    return {
      success: true,
      result: await options.publishAttributeData({ device_id: deviceId, value: payload.valueStr })
    }
  }

  if (payload.fieldType === 'command') {
    const commandWrite = resolveCommandWrite(payload.normalizedData, payload.fieldId)
    if (!commandWrite) {
      return {
        success: false,
        error: 'Command write requires a single command field payload'
      }
    }

    return {
      success: true,
      result: await options.publishCommandData({
        device_id: deviceId,
        identify: commandWrite.identify,
        value: commandWrite.value
      })
    }
  }

  return {
    success: true,
    result: await options.publishTelemetryData({ device_id: deviceId, value: payload.valueStr })
  }
}

export const createThingsVisWidgetPlatformWriteHandler = (options: PlatformWriteBridgeOptions) => {
  return async (event: MessageEvent) => {
    const message = parseTrustedGuestMessage<PlatformWriteMessagePayload>(
      event,
      {
        isTrusted: options.isTrustedMessageEvent
      },
      'tv:platform-write'
    )
    if (!message) return

    const request = normalizePlatformWriteRequest(message)
    if ('error' in request) {
      postPlatformWriteError(request.requestId, request.error, options.postMessageToGuest)
      return
    }

    // 写回请求只允许命中当前 widget 绑定的设备，避免 guest 越权写别的设备。
    const targetDevice = resolveWritableDeviceId(options.getPreviewDeviceId, request.deviceId)
    if ('error' in targetDevice) {
      if (targetDevice.error === 'Missing deviceId') {
        console.warn('[ThingsVisWidget] tv:platform-write received but deviceId prop is not set')
      }
      postPlatformWriteError(request.requestId, targetDevice.error as string, options.postMessageToGuest)
      return
    }

    try {
      const writePayload = buildPlatformWritePayload(
        request.dataSourceId,
        request.data,
        options.getFieldDataTypeMap,
        options.normalizeWriteData
      )
      const published = await publishPlatformWrite(targetDevice.deviceId, writePayload, options)
      if (!published.success) {
        postPlatformWriteError(request.requestId, published.error, options.postMessageToGuest)
        return
      }

      postPlatformWriteSuccess(
        request.requestId,
        published.result?.data ?? writePayload.normalizedData,
        options.postMessageToGuest
      )
    } catch (error) {
      console.error('[ThingsVisWidget] telemetryDataPub failed for tv:platform-write:', error)
      const message = error instanceof Error ? error.message : String(error || 'Platform write failed')
      postPlatformWriteError(request.requestId, message, options.postMessageToGuest)
    }
  }
}
