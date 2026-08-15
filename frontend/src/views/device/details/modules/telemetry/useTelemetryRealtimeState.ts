import { onUnmounted, ref, toValue, type MaybeRefOrGetter } from 'vue'
import { useWebSocket } from '@vueuse/core'
import { telemetryDataCurrent } from '@/service/api'
import { getWebsocketServerUrl, isJSON } from '@/utils/common/tool'
import { localStg } from '@/utils/storage'
import { mergeRealtimeTelemetry } from './telemetryRealtimeMerge'

export type TelemetryLoadStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'

type InFlightTelemetryRequest = {
  requestId: number
  promise: Promise<void>
}

const TELEMETRY_PAYLOAD_CONTAINER_KEYS = ['data', 'list', 'value'] as const
const TELEMETRY_REALTIME_FLUSH_MS = 120

function createTelemetryWsUrl() {
  return `${getWebsocketServerUrl()}/telemetry/datas/current/ws`
}

export function useTelemetryRealtimeState(deviceId: MaybeRefOrGetter<string>) {
  const telemetryData = ref<DeviceManagement.telemetryData[]>([])
  const initTelemetryData = ref<any>()
  const telemetryLoadError = ref('')
  const telemetryLoadStatus = ref<TelemetryLoadStatus>('idle')
  const token = localStg.get('token')
  const inFlightRequests = new Map<string, InFlightTelemetryRequest>()
  let latestRequestId = 0
  let appliedDeviceId = ''
  let realtimeFlushTimer: number | null = null
  let pendingRealtimePayloads: Record<string, any>[] = []

  const resolveDeviceId = () => toValue(deviceId)

  const collectPayloadDeviceIds = (payload: unknown, ids = new Set<string>()) => {
    if (!payload || typeof payload !== 'object') return ids

    if (Array.isArray(payload)) {
      payload.forEach((item) => collectPayloadDeviceIds(item, ids))
      return ids
    }

    const record = payload as Record<string, unknown>
    const directDeviceId = record.device_id || record.deviceId
    if (typeof directDeviceId === 'string' && directDeviceId) {
      ids.add(directDeviceId)
    }

    TELEMETRY_PAYLOAD_CONTAINER_KEYS.forEach((key) => {
      collectPayloadDeviceIds(record[key], ids)
    })

    return ids
  }

  const telemetryPayloadMatchesActiveDevice = (payload: unknown) => {
    const currentDeviceId = resolveDeviceId()
    const payloadDeviceIds = collectPayloadDeviceIds(payload)

    if (payloadDeviceIds.size === 0) {
      return Boolean(currentDeviceId) && appliedDeviceId === currentDeviceId
    }

    return payloadDeviceIds.size === 1 && payloadDeviceIds.has(currentDeviceId)
  }

  const clearPendingRealtimeTelemetry = () => {
    if (realtimeFlushTimer !== null) {
      window.clearTimeout(realtimeFlushTimer)
      realtimeFlushTimer = null
    }
    pendingRealtimePayloads = []
  }

  const flushRealtimeTelemetry = () => {
    realtimeFlushTimer = null
    if (!pendingRealtimePayloads.length) return

    const payloads = pendingRealtimePayloads
    pendingRealtimePayloads = []
    let mergedTelemetry = telemetryData.value
    payloads.forEach((payload) => {
      mergedTelemetry = mergeRealtimeTelemetry(mergedTelemetry, payload, initTelemetryData.value)
    })
    telemetryData.value = mergedTelemetry
  }

  const queueRealtimeTelemetryPayload = (payload: Record<string, any>) => {
    pendingRealtimePayloads.push(payload)
    if (realtimeFlushTimer !== null) return

    realtimeFlushTimer = window.setTimeout(flushRealtimeTelemetry, TELEMETRY_REALTIME_FLUSH_MS)
  }

  const handleRealtimeTelemetryMessage = (messageData: string) => {
    if (!messageData || messageData === 'pong' || !isJSON(messageData)) return
    const payload = JSON.parse(messageData)
    if (!telemetryPayloadMatchesActiveDevice(payload)) return
    queueRealtimeTelemetryPayload(payload)
  }

  const { status, send, close } = useWebSocket(createTelemetryWsUrl(), {
    heartbeat: {
      message: 'ping',
      interval: 8000,
      pongTimeout: 3000
    },
    onMessage(_ws: WebSocket, event: MessageEvent) {
      handleRealtimeTelemetryMessage(event.data)
    }
  })

  const cacheTelemetryTemplate = (data: DeviceManagement.telemetryData[], requestDeviceId: string) => {
    initTelemetryData.value = {
      ...(data[0] || {}),
      device_id: requestDeviceId
    }
  }

  const subscribeTelemetryStream = (requestDeviceId: string) => {
    send(
      JSON.stringify({
        device_id: requestDeviceId,
        token
      })
    )
  }

  const requestStillOwnsTelemetryState = (requestDeviceId: string, requestId: number) =>
    latestRequestId === requestId && resolveDeviceId() === requestDeviceId

  const fetchTelemetry = async () => {
    const requestDeviceId = resolveDeviceId()
    if (!requestDeviceId) {
      latestRequestId += 1
      appliedDeviceId = ''
      clearPendingRealtimeTelemetry()
      telemetryData.value = []
      initTelemetryData.value = undefined
      telemetryLoadStatus.value = 'idle'
      telemetryLoadError.value = ''
      return
    }

    const activeRequest = inFlightRequests.get(requestDeviceId)
    if (activeRequest && activeRequest.requestId === latestRequestId) {
      return activeRequest.promise
    }

    const requestId = latestRequestId + 1
    latestRequestId = requestId
    telemetryLoadStatus.value = 'loading'
    telemetryLoadError.value = ''

    if (appliedDeviceId && appliedDeviceId !== requestDeviceId) {
      clearPendingRealtimeTelemetry()
      telemetryData.value = []
      initTelemetryData.value = undefined
    }

    const requestPromise = (async () => {
      try {
        const { data, error } = await telemetryDataCurrent(requestDeviceId)
        if (!requestStillOwnsTelemetryState(requestDeviceId, requestId)) return

        if (error) {
          telemetryLoadStatus.value = 'error'
          telemetryLoadError.value = error instanceof Error ? error.message : String((error as any)?.message || error)
          return
        }

        if (!data) {
          telemetryLoadStatus.value = 'empty'
          telemetryData.value = []
          appliedDeviceId = requestDeviceId
          return
        }

        telemetryData.value = data
        telemetryLoadStatus.value = data.length > 0 ? 'ready' : 'empty'
        appliedDeviceId = requestDeviceId
        cacheTelemetryTemplate(data, requestDeviceId)
        subscribeTelemetryStream(requestDeviceId)
      } catch (error) {
        if (!requestStillOwnsTelemetryState(requestDeviceId, requestId)) return
        telemetryLoadStatus.value = 'error'
        telemetryLoadError.value = error instanceof Error ? error.message : String(error)
      } finally {
        const currentRequest = inFlightRequests.get(requestDeviceId)
        if (currentRequest?.requestId === requestId) {
          inFlightRequests.delete(requestDeviceId)
        }
      }
    })()

    inFlightRequests.set(requestDeviceId, {
      requestId,
      promise: requestPromise
    })

    return requestPromise
  }

  const closeTelemetrySocket = () => {
    if (status.value === 'OPEN') {
      close()
    }
  }

  const refreshTelemetry = () => fetchTelemetry()

  onUnmounted(() => {
    clearPendingRealtimeTelemetry()
    closeTelemetrySocket()
  })

  return {
    telemetryData,
    telemetryLoadError,
    telemetryLoadStatus,
    fetchTelemetry,
    refreshTelemetry,
    closeTelemetrySocket
  }
}
