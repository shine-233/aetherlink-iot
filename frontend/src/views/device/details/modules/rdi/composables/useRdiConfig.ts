/**
 * 文件用途: RDI 操作视图配置状态 composable。
 * 核心逻辑: 加载/保存 RDI 设备配置，维护系统信息、温度范围、字段标签和额外字段读写。
 * 关键注意事项: 配置字段默认值、读写权限和保存 payload 必须与 backend RDI 配置模型一致。
 * 重构建议: 将默认配置、字段 normalize 和保存 payload 拆成纯函数，并覆盖缺字段/保存失败测试。
 */
import { computed, reactive, ref } from 'vue'
import { rdiDeviceConfig, updateRdiDeviceConfig } from '@/service/api'
import type { RDICommandTracking, RDIConfig, RDISystemInfo } from '@/service/api/rdi'
import { message } from '@/utils/common/discrete'
import { useAppStore } from '@/store/modules/app'
import { labels, systemExtraFieldLabels, systemExtraFieldDefinitions } from '../constants/rdi-labels'
import type { LabelKey, SystemExtraFieldKey } from '../constants/rdi-labels'

type FieldRecord = Record<string, unknown>

const promotedSystemInfoExtraKeys = [
  'address',
  'installation_date',
  'installer_company',
  'installer_contact',
  'installer_name',
  'installer_phone',
  'installer_email',
  'controller_serial_number'
] as const

export function defaultConfig(): RDIConfig {
  return {
    data_collection_interval: 60,
    alarm_sensor_1_enabled: true,
    alarm_sensor_2_enabled: true,
    sensor_1_upper: 80,
    sensor_1_lower: -10,
    sensor_2_upper: 80,
    sensor_2_lower: -10,
    sensor_1_duration: 30,
    sensor_2_duration: 30,
    switch_1_alarm_mode: 'disabled',
    switch_2_alarm_mode: 'disabled',
    switch_1_alarm_duration: 30,
    switch_2_alarm_duration: 30,
    dry_contact_alarm_level: 'high',
    dry_contact_normal_level: 'low',
    dry_contact_alarm_delay: 0,
    dry_contact_normal_delay: 0,
    notification_enabled: false,
    notification_temperature_alarm: true,
    notification_switch_alarm: true,
    notification_warranty_alarm: true,
    sensor_alarm_emails: '',
    switch_alarm_emails: '',
    warranty_alarm_emails: '',
    sensor_1_alarm_emails: '',
    sensor_2_alarm_emails: '',
    switch_1_alarm_emails: '',
    switch_2_alarm_emails: '',
    field_setting: {}
  }
}

export function defaultSystemInfo(): RDISystemInfo {
  return {
    installation_location: '',
    address: '',
    installation_date: '',
    installer_company: '',
    installer_contact: '',
    installer_name: '',
    installer_phone: '',
    installer_email: '',
    controller_serial_number: '',
    maintenance_technician: '',
    customer_name: '',
    contact_email: '',
    contact_phone: '',
    warranty_status: '',
    extra_fields: {}
  }
}

function normalizeFieldRecord(value: unknown): FieldRecord {
  return value && typeof value === 'object' && !Array.isArray(value) ? { ...(value as FieldRecord) } : {}
}

function normalizeCollectionInterval(value: unknown) {
  const interval = Number(value)
  return Number.isFinite(interval) && interval >= 45 && interval <= 60 ? interval : defaultConfig().data_collection_interval
}

function normalizeConfig(config?: Partial<RDIConfig>): RDIConfig {
  const next = {
    ...defaultConfig(),
    ...(config || {}),
    field_setting: normalizeFieldRecord(config?.field_setting)
  }
  next.data_collection_interval = normalizeCollectionInterval(next.data_collection_interval)
  return next
}

function normalizeSystemInfo(systemInfo?: Partial<RDISystemInfo>): RDISystemInfo {
  const extraFields = normalizeFieldRecord(systemInfo?.extra_fields)
  const promotedValues = promotedSystemInfoExtraKeys.reduce<Partial<RDISystemInfo>>((acc, key) => {
    const value = systemInfo?.[key] || extraFields[key]
    if (typeof value === 'string' && value.trim()) {
      acc[key] = value.trim()
    }
    return acc
  }, {})

  return {
    ...defaultSystemInfo(),
    ...(systemInfo || {}),
    ...promotedValues,
    extra_fields: extraFields
  }
}

function withoutPromotedSystemInfoExtraFields(extraFields?: Record<string, unknown>) {
  const next = normalizeFieldRecord(extraFields)
  promotedSystemInfoExtraKeys.forEach((key) => {
    delete next[key]
  })
  return next
}

function applyConfigSnapshot(
  config: RDIConfig,
  systemInfo: RDISystemInfo,
  snapshot?: { config?: Partial<RDIConfig>; system_info?: Partial<RDISystemInfo> }
) {
  Object.assign(config, normalizeConfig(snapshot?.config))
  Object.assign(systemInfo, normalizeSystemInfo(snapshot?.system_info))
}

function buildSavePayload(config: RDIConfig, systemInfo: RDISystemInfo, applyToDevice: boolean) {
  return {
    config: { ...config, field_setting: normalizeFieldRecord(config.field_setting) },
    system_info: { ...systemInfo, extra_fields: withoutPromotedSystemInfoExtraFields(systemInfo.extra_fields) },
    apply_to_device: applyToDevice
  }
}

function parseFieldValue(key: string, value: string): unknown {
  const trimmed = value.trim()
  if (key.startsWith('n')) {
    return trimmed
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
  }

  if (key.startsWith('sw')) {
    try {
      const parsed = JSON.parse(trimmed)
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : { label: trimmed }
    } catch {
      return { label: trimmed }
    }
  }

  return trimmed
}

function writeTextField(
  record: FieldRecord | undefined,
  key: string,
  value: string,
  parser = parseFieldValue
): FieldRecord {
  const next = { ...(record || {}) }
  const trimmed = value.trim()
  if (!trimmed) {
    delete next[key]
    return next
  }

  next[key] = parser(key, trimmed)
  return next
}

function readFieldValue(record: FieldRecord | undefined, key: string) {
  const value = record?.[key]
  if (value === undefined || value === null) return ''
  if (Array.isArray(value)) return value.join(',')
  if (typeof value === 'object') {
    const label = (value as FieldRecord).label
    return label === undefined || label === null ? JSON.stringify(value) : String(label)
  }
  return String(value)
}

function readPlainExtraField(record: FieldRecord | undefined, key: string) {
  const value = record?.[key]
  return value === undefined || value === null ? '' : String(value)
}

function createSensorRange(
  config: RDIConfig,
  lowerKey: 'sensor_1_lower' | 'sensor_2_lower',
  upperKey: 'sensor_1_upper' | 'sensor_2_upper'
) {
  return computed<[number, number]>({
    get: () => [config[lowerKey], config[upperKey]] as [number, number],
    set: (value) => {
      config[lowerKey] = value[0]
      config[upperKey] = value[1]
    }
  })
}

function createDerivedState(config: RDIConfig, locale: () => App.I18n.LangType) {
  const text = computed(() => labels[locale()] || labels['en-US'])
  const systemExtraText = computed(() => systemExtraFieldLabels[locale()] || systemExtraFieldLabels['en-US'])

  return {
    sensor1Range: createSensorRange(config, 'sensor_1_lower', 'sensor_1_upper'),
    sensor2Range: createSensorRange(config, 'sensor_2_lower', 'sensor_2_upper'),
    fieldEntries: computed(() => Object.entries(config.field_setting || {})),
    t: (key: LabelKey) => text.value[key],
    systemExtraFieldLabel: (key: SystemExtraFieldKey) => systemExtraText.value[key] || key
  }
}

export function useRdiConfig(deviceId: () => string, onChange: () => void) {
  const appStore = useAppStore()
  const loading = ref(false)
  const applyToDevice = ref(true)
  const lastConfigCommandTracking = ref<RDICommandTracking | null>(null)

  const config = reactive<RDIConfig>(defaultConfig())
  const systemInfo = reactive<RDISystemInfo>(defaultSystemInfo())

  const { sensor1Range, sensor2Range, fieldEntries, t, systemExtraFieldLabel } = createDerivedState(
    config,
    () => appStore.locale
  )
  const configCommandTrackingSummary = computed(() => {
    const tracking = lastConfigCommandTracking.value
    if (!tracking) return ''
    const logState = tracking.log_recorded ? 'log recorded' : 'log not recorded'
    return `set_alarm_config message_id=${tracking.message_id || '--'} status=${tracking.status || '--'} (${logState})`
  })

  function setFieldValue(key: string, value: string) {
    config.field_setting = writeTextField(config.field_setting, key, value)
  }

  function getFieldValue(key: string) {
    return readFieldValue(config.field_setting, key)
  }

  function setSystemExtraField(key: string, value: string) {
    systemInfo.extra_fields = writeTextField(systemInfo.extra_fields, key, value, (_key, trimmed) => trimmed)
  }

  function getSystemExtraField(key: string) {
    return readPlainExtraField(systemInfo.extra_fields, key)
  }

  async function loadConfig() {
    const id = deviceId()
    if (!id) return
    loading.value = true
    try {
      const { error, data } = await rdiDeviceConfig(id)
      if (!error && data) {
        lastConfigCommandTracking.value = null
        applyConfigSnapshot(config, systemInfo, data)
      }
    } finally {
      loading.value = false
    }
  }

  async function saveConfig() {
    const id = deviceId()
    loading.value = true
    try {
      const { error, data } = await updateRdiDeviceConfig(id, buildSavePayload(config, systemInfo, applyToDevice.value))
      if (!error && data) {
        lastConfigCommandTracking.value = data.command_tracking || null
        applyConfigSnapshot(config, systemInfo, data)
        message.success(configCommandTrackingSummary.value || t('saved'))
        onChange()
      }
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    applyToDevice,
    lastConfigCommandTracking,
    configCommandTrackingSummary,
    config,
    systemInfo,
    sensor1Range,
    sensor2Range,
    fieldEntries,
    t,
    systemExtraFieldLabel,
    systemExtraFieldDefinitions,
    setFieldValue,
    getFieldValue,
    setSystemExtraField,
    getSystemExtraField,
    loadConfig,
    saveConfig
  }
}
