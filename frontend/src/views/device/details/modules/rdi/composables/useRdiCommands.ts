/**
 * 文件用途: RDI 操作视图命令下发 composable。
 * 核心逻辑: 构造命令 payload、发送 RDI command、维护 OTA 选择状态并统一处理命令错误。
 * 关键注意事项: public API 被 RdiDeviceOperationsView.vue 直接解构使用，字段名和 payload identifier 需与后端 RDI command 契约保持一致。
 * 重构建议: 保持 payload helper、发送 helper、状态 helper 和错误 helper 分离，避免面板层感知命令细节。
 */
import { computed, reactive, ref, watch } from 'vue'
import { rdiLatestFirmware, sendRdiCommand } from '@/service/api'
import type { RDICommandReq, RDICommandTracking, RDIConfig } from '@/service/api/rdi'
import { getOtaPackageList } from '@/service/product/update-package'
import { message } from '@/utils/common/discrete'
import { getBaseServerUrl } from '@/utils/common/tool'
import type { LabelKey } from '../constants/rdi-labels'

type TranslateRdiLabel = (key: LabelKey) => string
type DryContactLevel = 'high' | 'low'

type OtaCommandState = {
  firmware_url: string
  version: string
  size: number | null
  md5: string
}

type OtaPackageRecord = Record<string, unknown> & {
  id: string
  name?: string
  version?: string
  package_url?: string
  url?: string
  firmware_url?: string
  size?: number | string
  package_size?: number | string
  file_size?: number | string
  signature?: string
  md5?: string
  checksum?: string
  additional_info?: unknown
}

type LoadingRef = {
  value: boolean
}

type CommandSendResult = {
  success: boolean
  tracking: RDICommandTracking | null
}

function createOtaCommandState(): OtaCommandState {
  return {
    firmware_url: '',
    version: '',
    size: null,
    md5: ''
  }
}

function readPayloadList(payload: unknown): unknown[] {
  if (Array.isArray(payload)) return payload
  if (payload && typeof payload === 'object' && Array.isArray((payload as Record<string, unknown>).list)) {
    return (payload as { list: unknown[] }).list
  }
  return []
}

function normalizeOtaPackages(payload: unknown): OtaPackageRecord[] {
  return readPayloadList(payload)
    .filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === 'object')
    .map((item) => ({ ...item, id: String(item.id ?? '') }))
    .filter((item): item is OtaPackageRecord => Boolean(item.id))
}

function parsePackageAdditionalInfo(value: unknown): Record<string, unknown> {
  if (!value) return {}
  if (typeof value === 'object' && !Array.isArray(value)) return value as Record<string, unknown>
  if (typeof value !== 'string') return {}
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function parsePackageSize(...values: unknown[]): number | null {
  for (const value of values) {
    const numeric = typeof value === 'string' && value.trim() ? Number(value) : value
    if (typeof numeric === 'number' && Number.isFinite(numeric) && numeric > 0) return numeric
  }
  return null
}

function resolvePackageUrl(rawUrl: unknown) {
  const url = String(rawUrl ?? '').trim()
  if (!url) return ''
  if (/^https?:\/\//i.test(url)) return url
  const baseUrlWithoutApi = getBaseServerUrl().replace('/api/v1', '/')
  return `${baseUrlWithoutApi.replace(/\/+$/, '')}/${url.replace(/^\/+/, '')}`
}

function readPackageVersion(pkg: OtaPackageRecord, additionalInfo: Record<string, unknown>) {
  return String(pkg.version ?? additionalInfo.version ?? '')
}

function readPackageChecksum(pkg: OtaPackageRecord, additionalInfo: Record<string, unknown>) {
  return String(pkg.signature ?? pkg.md5 ?? pkg.checksum ?? additionalInfo.md5 ?? additionalInfo.signature ?? '')
}

function readPackageUrl(pkg: OtaPackageRecord) {
  return pkg.package_url ?? pkg.firmware_url ?? pkg.url ?? ''
}

function buildOtaPackageLabel(pkg: OtaPackageRecord) {
  const name = String(pkg.name ?? pkg.id)
  return pkg.version ? `${name} (${pkg.version})` : name
}

function getOtaMissingFieldLabels(otaCommand: OtaCommandState, t: TranslateRdiLabel) {
  const size = Number(otaCommand.size)
  const missing: LabelKey[] = []

  if (!otaCommand.firmware_url.trim()) missing.push('firmwareUrl')
  if (!otaCommand.version.trim()) missing.push('version')
  if (!Number.isFinite(size) || size <= 0) missing.push('size')
  if (!otaCommand.md5.trim()) missing.push('md5')

  return missing.map((key) => t(key))
}

function applyOtaPackageToState(pkg: OtaPackageRecord, otaCommand: OtaCommandState) {
  const additionalInfo = parsePackageAdditionalInfo(pkg.additional_info)
  otaCommand.firmware_url = resolvePackageUrl(readPackageUrl(pkg))
  otaCommand.version = readPackageVersion(pkg, additionalInfo)
  otaCommand.size = parsePackageSize(pkg.size, pkg.package_size, pkg.file_size, additionalInfo.size)
  otaCommand.md5 = readPackageChecksum(pkg, additionalInfo)
}

function normalizeFieldSettingValue(key: string, value: unknown) {
  const normalizedKey = key.toLowerCase()
  if (normalizedKey.startsWith('n')) {
    if (Array.isArray(value)) return value
    if (typeof value === 'string') {
      return value
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
    }
  }

  if (normalizedKey.startsWith('sw')) {
    if (value && typeof value === 'object' && !Array.isArray(value)) return value
    if (typeof value === 'string') return { label: value }
  }

  return value
}

function normalizeFieldSetting(fieldSetting: RDIConfig['field_setting']) {
  return Object.entries(fieldSetting || {}).reduce<Record<string, unknown>>((params, [key, value]) => {
    params[key] = normalizeFieldSettingValue(key, value)
    return params
  }, {})
}

function buildDryContactCommandPayload(level: DryContactLevel, delaySeconds: number): RDICommandReq {
  return {
    identifier: 'set_dry_contact',
    params: {
      level,
      delay_seconds: delaySeconds
    }
  }
}

function buildDryContactTestCommandPayload(config: RDIConfig, durationSeconds: number): RDICommandReq {
  return {
    identifier: 'test_dry_contact',
    params: {
      level: config.dry_contact_alarm_level,
      duration_seconds: durationSeconds
    }
  }
}

function buildFieldSettingCommandPayload(config: RDIConfig, t: TranslateRdiLabel): RDICommandReq | null {
  const params = normalizeFieldSetting(config.field_setting)
  if (!Object.keys(params).length) {
    message.error(t('empty'))
    return null
  }
  return {
    identifier: 'set_field_setting',
    params
  }
}

function buildOtaUpgradeCommandPayload(otaCommand: OtaCommandState, t: TranslateRdiLabel): RDICommandReq | null {
  const size = Number(otaCommand.size)
  const missingLabels = getOtaMissingFieldLabels(otaCommand, t)
  if (missingLabels.length) {
    message.error(`${t('otaMissingFields')}: ${missingLabels.join(', ')}`)
    return null
  }
  return {
    identifier: 'ota_upgrade',
    params: {
      firmware_url: otaCommand.firmware_url.trim(),
      version: otaCommand.version.trim(),
      size,
      md5: otaCommand.md5.trim()
    }
  }
}

function buildSimpleCommandPayload(
  identifier: Extract<RDICommandReq['identifier'], 'unbind_device' | 'factory_reset'>
): RDICommandReq {
  return {
    identifier,
    params: {}
  }
}

function getErrorText(error: unknown, fallback: string) {
  if (typeof error === 'string') return error
  if (error && typeof error === 'object' && 'message' in error)
    return String((error as { message?: unknown }).message || fallback)
  return fallback
}

function handleCommandError(error: unknown, t: TranslateRdiLabel) {
  message.error(getErrorText(error, t('ota')))
}

function handleOtaPackageError(error: unknown, t: TranslateRdiLabel) {
  message.error(getErrorText(error, t('empty')))
}

function handleLatestFirmwareResponse(
  data: any,
  latestFirmwarePackage: { value: OtaPackageRecord | null },
  otaCommand: OtaCommandState,
  t: TranslateRdiLabel
) {
  if (!data?.update_available || !data.package) {
    latestFirmwarePackage.value = null
    message.success(t('alreadyLatest'))
    return
  }

  const pkg = normalizeOtaPackages([data.package])[0]
  if (!pkg) {
    latestFirmwarePackage.value = null
    message.error(t('empty'))
    return
  }

  latestFirmwarePackage.value = pkg
  applyOtaPackageToState(pkg, otaCommand)
  message.success(
    `${t('updateAvailable')}: ${readPackageVersion(pkg, parsePackageAdditionalInfo(pkg.additional_info))}`
  )
}

function normalizeCommandTracking(payload: any): RDICommandTracking | null {
  if (payload?.command_tracking && typeof payload.command_tracking === 'object') {
    return payload.command_tracking as RDICommandTracking
  }
  if (!payload?.message_id) return null
  return {
    message_id: String(payload.message_id),
    status: String(payload.tracking_status ?? payload.status ?? ''),
    device_id: String(payload.device_id ?? ''),
    identifier: String(payload.identifier ?? ''),
    operation_type: String(payload.operation_type ?? ''),
    log_recorded: payload.log_recorded !== false
  }
}

function buildCommandTrackingSummary(tracking: RDICommandTracking | null) {
  if (!tracking) return ''
  const logState = tracking.log_recorded ? 'log recorded' : 'log not recorded'
  return `${tracking.identifier || 'command'} message_id=${tracking.message_id || '--'} status=${tracking.status || '--'} (${logState})`
}

async function sendCommandPayload(
  deviceId: string,
  payload: RDICommandReq,
  loading: LoadingRef,
  t: TranslateRdiLabel
): Promise<CommandSendResult> {
  loading.value = true
  try {
    const { error, data } = await sendRdiCommand(deviceId, payload)
    if (error) {
      handleCommandError(error, t)
      return { success: false, tracking: null }
    }
    return { success: true, tracking: normalizeCommandTracking(data) }
  } catch (error) {
    handleCommandError(error, t)
    return { success: false, tracking: null }
  } finally {
    loading.value = false
  }
}

export function useRdiCommands(deviceId: () => string, config: RDIConfig, t: TranslateRdiLabel) {
  const commandLoading = ref(false)
  const dryCommandDelay = ref(0)
  const dryTestDuration = ref(1)
  const otaPackageLoading = ref(false)
  const otaPackageId = ref('')
  const latestFirmwareLoading = ref(false)
  const latestFirmwarePackage = ref<OtaPackageRecord | null>(null)
  const lastCommandTracking = ref<RDICommandTracking | null>(null)
  const otaPackages = ref<OtaPackageRecord[]>([])
  const otaCommand = reactive(createOtaCommandState())

  const otaPackageOptions = computed(() =>
    otaPackages.value.map((pkg) => ({
      label: buildOtaPackageLabel(pkg),
      value: pkg.id
    }))
  )
  const otaMissingFieldLabels = computed(() => getOtaMissingFieldLabels(otaCommand, t))
  const canSendOtaUpgrade = computed(() => otaMissingFieldLabels.value.length === 0)
  const commandTrackingSummary = computed(() => buildCommandTrackingSummary(lastCommandTracking.value))

  function applySelectedOtaPackage(id: string) {
    const selectedPackage = otaPackages.value.find((pkg) => pkg.id === id)
    if (selectedPackage) applyOtaPackageToState(selectedPackage, otaCommand)
  }

  async function sendPayload(payload: RDICommandReq | null) {
    if (!payload) return false
    const id = deviceId()
    if (!id) return false
    const result = await sendCommandPayload(id, payload, commandLoading, t)
    if (result.success) {
      lastCommandTracking.value = result.tracking
      if (result.tracking?.message_id) {
        message.success(buildCommandTrackingSummary(result.tracking))
      } else {
        message.success(t('sent'))
      }
    }
    return result.success
  }

  async function setDryContact(level: DryContactLevel) {
    return sendPayload(buildDryContactCommandPayload(level, dryCommandDelay.value))
  }

  async function testDryContact() {
    return sendPayload(buildDryContactTestCommandPayload(config, dryTestDuration.value))
  }

  async function sendFieldSetting() {
    return sendPayload(buildFieldSettingCommandPayload(config, t))
  }

  async function sendOtaUpgrade() {
    return sendPayload(buildOtaUpgradeCommandPayload(otaCommand, t))
  }

  async function sendUnbindDevice() {
    return sendPayload(buildSimpleCommandPayload('unbind_device'))
  }

  async function sendFactoryReset() {
    return sendPayload(buildSimpleCommandPayload('factory_reset'))
  }

  async function loadOtaPackages() {
    otaPackageLoading.value = true
    try {
      const { error, data } = await getOtaPackageList({ page: 1, page_size: 100 })
      if (error) {
        handleOtaPackageError(error, t)
        return
      }
      otaPackages.value = normalizeOtaPackages(data)
      applySelectedOtaPackage(otaPackageId.value)
    } catch (error) {
      handleOtaPackageError(error, t)
    } finally {
      otaPackageLoading.value = false
    }
  }

  function applyLatestFirmwarePackage() {
    if (!latestFirmwarePackage.value) {
      message.error(t('empty'))
      return
    }
    applyOtaPackageToState(latestFirmwarePackage.value, otaCommand)
  }

  async function checkLatestFirmware() {
    const id = deviceId()
    if (!id) return
    latestFirmwareLoading.value = true
    try {
      const { error, data } = await rdiLatestFirmware(id)
      if (error) {
        handleOtaPackageError(error, t)
        return
      }
      handleLatestFirmwareResponse(data, latestFirmwarePackage, otaCommand, t)
    } catch (error) {
      handleOtaPackageError(error, t)
    } finally {
      latestFirmwareLoading.value = false
    }
  }

  watch(otaPackageId, (value) => {
    applySelectedOtaPackage(value)
  })

  return {
    commandLoading,
    dryCommandDelay,
    dryTestDuration,
    otaPackageLoading,
    otaPackageId,
    latestFirmwareLoading,
    latestFirmwarePackage,
    lastCommandTracking,
    otaCommand,
    otaPackages,
    otaPackageOptions,
    otaMissingFieldLabels,
    canSendOtaUpgrade,
    commandTrackingSummary,
    setDryContact,
    testDryContact,
    sendFieldSetting,
    sendOtaUpgrade,
    sendUnbindDevice,
    sendFactoryReset,
    loadOtaPackages,
    applyLatestFirmwarePackage,
    checkLatestFirmware
  }
}
