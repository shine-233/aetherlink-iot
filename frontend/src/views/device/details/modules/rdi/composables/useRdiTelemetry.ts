/**
 * 文件用途: RDI 操作视图实时遥测 composable。
 * 核心逻辑: 按设备 ID 轮询当前遥测和在线状态，计算温度单位、字段展示和刷新生命周期。
 * 关键注意事项: 轮询间隔、组件卸载清理、温度单位偏好和在线态字段会影响详情页实时性。
 * 重构建议: 抽出温度转换和遥测 normalize helper，并补轮询清理、离线状态和接口失败测试。
 */
import { computed, onUnmounted, ref, watch } from 'vue'
import { telemetryDataCurrentKeys, getDeviceOnlineStatus } from '@/service/api'
import type { LabelKey } from '../constants/rdi-labels'
import { useRdiTemperatureUnit } from './useRdiTemperatureUnit'

type TemperatureUnit = 'C' | 'F'

const RDI_TELEMETRY_REFRESH_MS = 10_000
const RDI_TELEMETRY_KEYS = [
  'temperature_1',
  'temperature_2',
  'switch_1',
  'switch_2',
  'dry_contact_output',
  'electricity_consumption',
  'led_status'
]

type TranslateRdiLabel = (key: LabelKey) => string

function isBlankValue(value: unknown) {
  return value === undefined || value === null || value === ''
}

function toFallbackText(value: unknown) {
  return isBlankValue(value) ? '--' : String(value)
}

function normalizeTelemetryPayload(payload: unknown) {
  const result: Record<string, unknown> = {}
  if (Array.isArray(payload)) {
    payload.forEach((item) => {
      if (item && typeof item === 'object') {
        const row = item as Record<string, unknown>
        const key = String(row.key || '')
        if (key) result[key] = row.value
      }
    })
    return result
  }
  if (payload && typeof payload === 'object') return payload as Record<string, unknown>
  return result
}

function formatTelemetryValue(value: unknown) {
  const numeric = Number(value)
  if (Number.isFinite(numeric)) return numeric.toFixed(2)
  return toFallbackText(value)
}

function formatTemperatureTelemetryValue(value: unknown, temperatureUnit: TemperatureUnit) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return toFallbackText(value)
  const converted = temperatureUnit === 'F' ? (numeric * 9) / 5 + 32 : numeric
  return converted.toFixed(2)
}

function formatSwitchTelemetryValue(value: unknown, t: TranslateRdiLabel) {
  if (value === true || value === 1 || value === '1' || value === 'true') return t('high')
  if (value === false || value === 0 || value === '0' || value === 'false') return t('low')
  return toFallbackText(value)
}

function formatLedTelemetryValue(value: unknown, t: TranslateRdiLabel) {
  const raw = String(value ?? '')
    .trim()
    .toLowerCase()
  if (!raw) return '--'
  switch (raw) {
    case 'off':
      return t('ledOff')
    case 'solid':
      return t('ledSolid')
    case 'slow_blink':
    case 'slowblink':
      return t('ledSlowBlink')
    case 'fast_blink':
    case 'fastblink':
      return t('ledFastBlink')
    case 'error':
      return t('ledError')
    default:
      return String(value)
  }
}

function toAxisValue(value: unknown): string | number | null | undefined {
  if (value === undefined || value === null) return value
  if (typeof value === 'string' || typeof value === 'number') return value
  return String(value)
}

function deriveOnlineStatus(
  liveOnlineStatus: number | null,
  onlineStatus: number | undefined,
  deviceData: Record<string, any> | undefined
) {
  const status = liveOnlineStatus ?? onlineStatus ?? deviceData?.is_online
  return status === 1 || status === true
}

function deriveDeviceDescription(deviceData: Record<string, any> | undefined) {
  const description = deviceData?.description ?? deviceData?.device_description ?? deviceData?.Description
  if (description === null || description === undefined) return '--'
  const descText = String(description).trim()
  return descText || '--'
}

function normalizeOnlineStatus(data: unknown) {
  if (!data || typeof data !== 'object') return null
  const row = data as Record<string, unknown>
  const rawStatus = row.device_status ?? row.is_online
  const normalizedStatus = Number(rawStatus)
  return Number.isFinite(normalizedStatus) ? normalizedStatus : null
}

function hasExternalOnlineStatus(onlineStatus: number | undefined, deviceData: Record<string, any> | undefined) {
  return onlineStatus !== undefined || deviceData?.is_online !== undefined
}

async function fetchTelemetrySnapshot(deviceId: string) {
  const { error, data } = await telemetryDataCurrentKeys({ device_id: deviceId, keys: RDI_TELEMETRY_KEYS })
  return error ? null : normalizeTelemetryPayload(data)
}

async function fetchOnlineStatusSnapshot(deviceId: string) {
  const { error, data } = await getDeviceOnlineStatus(deviceId)
  if (error || !data) return null
  return normalizeOnlineStatus(data)
}

function clearTelemetryRefresh(timer: { value: number | null }) {
  if (timer.value === null) return
  if (typeof window !== 'undefined') {
    window.clearTimeout(timer.value)
  }
  timer.value = null
}

function scheduleTelemetryRefresh(
  timer: { value: number | null },
  refresh: () => Promise<void>,
  isActive: () => boolean
) {
  if (typeof window === 'undefined' || timer.value !== null || !isActive()) return
  timer.value = window.setTimeout(() => {
    timer.value = null
    if (!isActive()) return
    void refresh().finally(() => {
      if (isActive()) {
        scheduleTelemetryRefresh(timer, refresh, isActive)
      }
    })
  }, RDI_TELEMETRY_REFRESH_MS)
}

export function useRdiTelemetry(
  deviceId: () => string,
  online: () => number | undefined,
  deviceData: () => Record<string, any> | undefined,
  t: (key: LabelKey) => string
) {
  const telemetry = ref<Record<string, unknown>>({})
  const liveOnlineStatus = ref<number | null>(null)
  const temperatureUnit = useRdiTemperatureUnit()
  const telemetryRefreshTimer = ref<number | null>(null)
  const telemetryRefreshActive = ref(false)
  let telemetryRequestSequence = 0
  let onlineStatusRequestSequence = 0

  function formatValue(value: unknown) {
    return formatTelemetryValue(value)
  }

  function formatTemperatureValue(value: unknown) {
    return formatTemperatureTelemetryValue(value, temperatureUnit.value)
  }

  function formatSwitch(value: unknown) {
    return formatSwitchTelemetryValue(value, t)
  }

  function formatLedStatus(value: unknown) {
    return formatLedTelemetryValue(value, t)
  }

  const telemetryRows = computed(() => [
    { label: 'T1', value: formatTemperatureValue(telemetry.value.temperature_1), unit: temperatureUnit.value },
    { label: 'T2', value: formatTemperatureValue(telemetry.value.temperature_2), unit: temperatureUnit.value },
    { label: t('switch1'), value: formatSwitch(telemetry.value.switch_1), unit: '' },
    { label: t('switch2'), value: formatSwitch(telemetry.value.switch_2), unit: '' },
    { label: t('dryContact'), value: formatSwitch(telemetry.value.dry_contact_output), unit: '' },
    { label: 'kWh', value: formatValue(telemetry.value.electricity_consumption), unit: 'kWh' },
    { label: 'LED1', value: formatLedStatus(telemetry.value.led_status), unit: '' }
  ])

  const deviceOnlineText = computed(() => {
    return deriveOnlineStatus(liveOnlineStatus.value, online(), deviceData()) ? t('online') : t('offline')
  })

  const deviceDescriptionText = computed(() => {
    return deriveDeviceDescription(deviceData())
  })

  async function loadTelemetry() {
    const id = deviceId()
    if (!id) return
    const sequence = ++telemetryRequestSequence
    const snapshot = await fetchTelemetrySnapshot(id)
    if (sequence !== telemetryRequestSequence || id !== deviceId()) return
    if (snapshot) telemetry.value = snapshot
  }

  async function loadOnlineStatus() {
    const id = deviceId()
    if (!id) return
    const sequence = ++onlineStatusRequestSequence
    if (hasExternalOnlineStatus(online(), deviceData())) {
      liveOnlineStatus.value = null
      return
    }

    const status = await fetchOnlineStatusSnapshot(id)
    if (sequence !== onlineStatusRequestSequence || id !== deviceId()) return
    if (status !== null) liveOnlineStatus.value = status
  }

  async function loadRealtimeState() {
    await Promise.all([loadTelemetry(), loadOnlineStatus()])
  }

  function stopTelemetryRefresh() {
    telemetryRefreshActive.value = false
    clearTelemetryRefresh(telemetryRefreshTimer)
  }

  function startTelemetryRefresh() {
    stopTelemetryRefresh()
    if (typeof window === 'undefined' || !deviceId()) return
    telemetryRefreshActive.value = true
    scheduleTelemetryRefresh(telemetryRefreshTimer, loadRealtimeState, () => telemetryRefreshActive.value)
  }

  watch(
    () => [online(), deviceData()?.is_online],
    () => {
      if (hasExternalOnlineStatus(online(), deviceData())) {
        onlineStatusRequestSequence += 1
        liveOnlineStatus.value = null
      }
    }
  )

  watch(deviceId, () => {
    telemetryRequestSequence += 1
    onlineStatusRequestSequence += 1
    telemetry.value = {}
    liveOnlineStatus.value = null
  })

  onUnmounted(() => {
    stopTelemetryRefresh()
  })

  return {
    telemetry,
    liveOnlineStatus,
    temperatureUnit,
    telemetryRows,
    deviceOnlineText,
    deviceDescriptionText,
    formatTemperatureValue,
    formatSwitch,
    formatLedStatus,
    formatValue,
    toAxisValue,
    loadTelemetry,
    loadOnlineStatus,
    loadRealtimeState,
    startTelemetryRefresh,
    stopTelemetryRefresh
  }
}
