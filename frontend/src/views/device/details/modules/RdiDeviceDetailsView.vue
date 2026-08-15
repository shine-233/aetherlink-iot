<!--
  RDI 客户只读设备详情视图。
  该视图查询 RDI 平台已保存配置和实时遥测，展示设备、安装、联系人、参数摘要及当前传感器状态，不提供保存或下发入口。
  promoted system_info 字段优先读取顶层值，同时兼容旧记录仍保存在 extra_fields 的形态。
-->
<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useAppStore } from '@/store/modules/app'
import { rdiDeviceConfig } from '@/service/api/rdi'
import type { RDIConfig, RDIDeviceConfigResponse, RDISystemInfo } from '@/service/api/rdi'
import { labels } from './rdi/constants/rdi-labels'
import type { LabelKey } from './rdi/constants/rdi-labels'
import { useRdiTelemetry } from './rdi/composables/useRdiTelemetry'
import RdiTelemetrySummary from './rdi/RdiTelemetrySummary.vue'
// REQ-48 / REQ-53：按 REQ-07 的四 Tab 约束，这两块内容嵌在详细信息页内，
// 不再各占一个顶层 Tab。两者都是只读面板，只需要设备 ID。
import RdiFieldSettingsPanel from './rdi/RdiFieldSettingsPanel.vue'
import RdiDevicePowerConsumptionView from './RdiDevicePowerConsumptionView.vue'

defineOptions({
  name: 'RdiDeviceDetailsView',
  inheritAttrs: false
})

const props = defineProps<{
  id: string
  online?: number
  onlineUpdatedAt?: string
  deviceData?: Record<string, any>
}>()

type DetailItem = {
  key: string
  label: LabelKey
  value: unknown
}

const appStore = useAppStore()
const loading = ref(false)
const snapshot = ref<RDIDeviceConfigResponse | null>(null)
let requestSequence = 0
let viewActive = true

const text = computed(() => labels[appStore.locale] || labels['en-US'])
const t = (key: string) => text.value[key as LabelKey] ?? key
const systemInfo = computed<Partial<RDISystemInfo>>(() => snapshot.value?.system_info || {})
const config = computed<Partial<RDIConfig>>(() => snapshot.value?.config || {})
const deviceId = () => String(props.id || '').trim()
const {
  temperatureUnit,
  telemetryRows,
  deviceOnlineText,
  formatTemperatureValue,
  loadRealtimeState,
  startTelemetryRefresh
} = useRdiTelemetry(deviceId, () => props.online, () => props.deviceData, t)
const temperatureUnitOptions = computed(() => [
  { label: `${t('celsius')} (C)`, value: 'C' },
  { label: `${t('fahrenheit')} (F)`, value: 'F' }
])

function firstPresent(...values: unknown[]) {
  return values.find((value) => value !== undefined && value !== null && String(value).trim() !== '')
}

function formatValue(value: unknown) {
  const resolved = firstPresent(value)
  if (resolved === undefined) return '--'
  if (typeof resolved === 'object') return JSON.stringify(resolved)
  return String(resolved)
}

function readSystemInfo(key: keyof RDISystemInfo) {
  return firstPresent(systemInfo.value[key], systemInfo.value.extra_fields?.[key])
}

function settingPart(label: LabelKey, value: unknown, formatter: (value: unknown) => string = formatValue) {
  const resolved = firstPresent(value)
  if (resolved === undefined) return undefined
  return `${t(label)}: ${formatter(resolved)}`
}

function joinSettingParts(parts: Array<string | undefined>) {
  const visibleParts = parts.filter((part): part is string => Boolean(part))
  return visibleParts.length ? visibleParts.join(' | ') : '--'
}

function formatEnabled(value: unknown) {
  if (value === true || value === 1 || value === '1' || value === 'true') return t('enabled')
  if (value === false || value === 0 || value === '0' || value === 'false') return t('disabled')
  return formatValue(value)
}

function formatMode(value: unknown) {
  if (value === 'powered_on') return t('poweredOn')
  if (value === 'powered_off') return t('poweredOff')
  if (value === 'disabled') return t('disabled')
  return formatValue(value)
}

function formatLevel(value: unknown) {
  if (value === 'high') return t('high')
  if (value === 'low') return t('low')
  return formatValue(value)
}

function formatSeconds(value: unknown) {
  const resolved = firstPresent(value)
  if (resolved === undefined) return '--'
  const numeric = Number(resolved)
  return Number.isFinite(numeric) ? `${numeric} s` : '--'
}

function formatConfigTemperature(value: unknown) {
  const resolved = firstPresent(value)
  if (resolved === undefined) return '--'
  const numeric = Number(resolved)
  return Number.isFinite(numeric) ? formatTemperatureValue(numeric) : '--'
}

function temperatureRangePart(lower: unknown, upper: unknown) {
  if (firstPresent(lower, upper) === undefined) return undefined
  return `${t('alarmRange')}: ${formatConfigTemperature(lower)} - ${formatConfigTemperature(upper)} ${temperatureUnit.value}`
}

function formatSensorSettings(sensor: 1 | 2) {
  const prefix = sensor === 1 ? 'sensor_1' : 'sensor_2'
  const enabledKey = sensor === 1 ? 'alarm_sensor_1_enabled' : 'alarm_sensor_2_enabled'
  const lower = config.value[`${prefix}_lower` as keyof RDIConfig]
  const upper = config.value[`${prefix}_upper` as keyof RDIConfig]
  const duration = config.value[`${prefix}_duration` as keyof RDIConfig]
  return joinSettingParts([
    settingPart('statusLabel', config.value[enabledKey], formatEnabled),
    temperatureRangePart(lower, upper),
    settingPart('triggerEffectiveTime', duration, formatSeconds)
  ])
}

function formatSwitchSettings(node: 1 | 2) {
  const prefix = node === 1 ? 'switch_1' : 'switch_2'
  return joinSettingParts([
    settingPart('mode', config.value[`${prefix}_alarm_mode` as keyof RDIConfig], formatMode),
    settingPart(
      'triggerEffectiveTime',
      config.value[`${prefix}_alarm_duration` as keyof RDIConfig],
      formatSeconds
    )
  ])
}

function formatDryContactSettings() {
  const alarmDelay = firstPresent(config.value.dry_contact_alarm_delay)
  const normalDelay = firstPresent(config.value.dry_contact_normal_delay)
  const sharedDelay =
    alarmDelay !== undefined && normalDelay !== undefined && String(alarmDelay) === String(normalDelay)
      ? settingPart('triggerEffectiveTime', alarmDelay, formatSeconds)
      : undefined
  return joinSettingParts([
    settingPart('alarmLevel', config.value.dry_contact_alarm_level, formatLevel),
    settingPart('normalLevel', config.value.dry_contact_normal_level, formatLevel),
    sharedDelay || settingPart('alarmDelay', alarmDelay, formatSeconds),
    sharedDelay ? undefined : settingPart('normalDelay', normalDelay, formatSeconds)
  ])
}

function formatNotificationSettings() {
  return joinSettingParts([
    settingPart('statusLabel', config.value.notification_enabled, formatEnabled),
    settingPart('temperatureAlarmNotice', config.value.notification_temperature_alarm, formatEnabled),
    settingPart('switchAlarmNotice', config.value.notification_switch_alarm, formatEnabled),
    settingPart('warrantyAlarmNotice', config.value.notification_warranty_alarm, formatEnabled)
  ])
}

const basicInfoItems = computed<DetailItem[]>(() => [
  {
    key: 'device-name',
    label: 'deviceName',
    value: firstPresent(snapshot.value?.device_name, props.deviceData?.name)
  },
  {
    key: 'device-id',
    label: 'deviceId',
    value: firstPresent(snapshot.value?.device_id, props.deviceData?.id, props.id)
  },
  {
    key: 'pid',
    label: 'pid',
    value: firstPresent(
      snapshot.value?.pid_number,
      props.deviceData?.pid_number,
      props.deviceData?.device_number
    )
  },
  {
    key: 'firmware',
    label: 'firmware',
    value: firstPresent(
      snapshot.value?.firmware_version,
      props.deviceData?.firmware_version,
      props.deviceData?.current_version
    )
  },
  {
    key: 'status',
    label: 'statusLabel',
    value: deviceOnlineText.value
  },
  {
    key: 'connection',
    label: 'connection',
    value: firstPresent(snapshot.value?.connection_type, props.deviceData?.connection_type, props.deviceData?.protocol)
  },
  {
    key: 'description',
    label: 'description',
    value: props.deviceData?.description
  },
  {
    key: 'last-heartbeat',
    label: 'lastHeartbeat',
    value: props.onlineUpdatedAt
  }
])

const systemInfoItems = computed<DetailItem[]>(() => [
  { key: 'installation-location', label: 'location', value: readSystemInfo('installation_location') },
  { key: 'address', label: 'installationAddress', value: readSystemInfo('address') },
  { key: 'installation-date', label: 'installationDate', value: readSystemInfo('installation_date') },
  { key: 'installer-company', label: 'installerCompany', value: readSystemInfo('installer_company') },
  { key: 'installer-contact', label: 'installerContact', value: readSystemInfo('installer_contact') },
  { key: 'installer-name', label: 'installerName', value: readSystemInfo('installer_name') },
  { key: 'installer-phone', label: 'installerPhone', value: readSystemInfo('installer_phone') },
  { key: 'installer-email', label: 'installerEmail', value: readSystemInfo('installer_email') },
  {
    key: 'controller-serial-number',
    label: 'controllerSerialNumber',
    value: readSystemInfo('controller_serial_number')
  },
  { key: 'customer', label: 'customer', value: readSystemInfo('customer_name') },
  { key: 'technician', label: 'technician', value: readSystemInfo('maintenance_technician') },
  { key: 'contact-phone', label: 'phone', value: readSystemInfo('contact_phone') },
  { key: 'contact-email', label: 'email', value: readSystemInfo('contact_email') },
  { key: 'warranty', label: 'warranty', value: readSystemInfo('warranty_status') }
])

const promotedSystemInfoKeys = new Set<string>([
  'installation_location',
  'address',
  'installation_date',
  'installer_company',
  'installer_contact',
  'installer_name',
  'installer_phone',
  'installer_email',
  'controller_serial_number',
  'customer_name',
  'maintenance_technician',
  'contact_phone',
  'contact_email',
  'warranty_status'
])

function humanizeExtraFieldKey(key: string) {
  const cleaned = key.replace(/[_-]+/g, ' ').trim()
  if (!cleaned) return key
  return cleaned.replace(/\b\w/g, (char) => char.toUpperCase())
}

// 渲染 extra_fields 中未被提升为具名字段的自定义键(方案 REQ-50 的"扩展JSON字段"读侧闭环):
// 写侧 withoutPromotedSystemInfoExtraFields 会保留这些键,此前详情视图只渲染 14 个具名字段,
// 客户自定义字段被持久化却从不显示。这里把剩余键按稳定顺序动态展示,标签取键的人性化形式。
const customSystemInfoItems = computed<Array<{ key: string; label: string; value: unknown }>>(() => {
  const extraFields = systemInfo.value.extra_fields
  if (!extraFields || typeof extraFields !== 'object') return []
  return Object.keys(extraFields)
    .filter((key) => !promotedSystemInfoKeys.has(key))
    .filter((key) => firstPresent(extraFields[key]) !== undefined)
    .sort()
    .map((key) => ({
      key: `extra-${key}`,
      label: humanizeExtraFieldKey(key),
      value: extraFields[key]
    }))
})

const currentParameterItems = computed<DetailItem[]>(() => [
  {
    key: 'collection-interval',
    label: 'interval',
    value: formatSeconds(config.value.data_collection_interval)
  },
  { key: 'sensor-1-settings', label: 'sensor1', value: formatSensorSettings(1) },
  { key: 'sensor-2-settings', label: 'sensor2', value: formatSensorSettings(2) },
  { key: 'switch-1-settings', label: 'switch1', value: formatSwitchSettings(1) },
  { key: 'switch-2-settings', label: 'switch2', value: formatSwitchSettings(2) },
  { key: 'dry-contact-settings', label: 'dryContact', value: formatDryContactSettings() },
  { key: 'notification-settings', label: 'notification', value: formatNotificationSettings() }
])

async function loadDetails() {
  const currentDeviceId = deviceId()
  const sequence = ++requestSequence

  if (!currentDeviceId) {
    snapshot.value = null
    loading.value = false
    return
  }

  loading.value = true
  try {
    const { error, data } = await rdiDeviceConfig(currentDeviceId)
    if (sequence !== requestSequence) return
    snapshot.value = !error && data ? data : null
  } catch {
    if (sequence === requestSequence) snapshot.value = null
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

watch(
  () => props.id,
  async () => {
    const requestedDeviceId = deviceId()
    await Promise.all([loadDetails(), loadRealtimeState()])
    if (viewActive && requestedDeviceId && requestedDeviceId === deviceId()) {
      startTelemetryRefresh()
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  viewActive = false
})
</script>

<template>
  <NSpin :show="loading">
    <div class="rdi-device-details">
      <NCard :title="t('basicInfo')" size="small">
        <NDescriptions bordered :column="2" label-placement="left" size="small">
          <NDescriptionsItem v-for="item in basicInfoItems" :key="item.key" :label="t(item.label)">
            {{ formatValue(item.value) }}
          </NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <NCard :title="t('system')" size="small">
        <NDescriptions bordered :column="2" label-placement="left" size="small">
          <NDescriptionsItem v-for="item in systemInfoItems" :key="item.key" :label="t(item.label)">
            {{ formatValue(item.value) }}
          </NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <NCard
        v-if="customSystemInfoItems.length"
        :title="t('extendedFields')"
        size="small"
        data-testid="rdi-additional-fields"
      >
        <NDescriptions bordered :column="2" label-placement="left" size="small">
          <NDescriptionsItem v-for="item in customSystemInfoItems" :key="item.key" :label="item.label">
            {{ formatValue(item.value) }}
          </NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <NCard :title="t('configuredParameters')" size="small" data-testid="rdi-current-parameter-summary">
        <NDescriptions bordered :column="2" label-placement="left" size="small">
          <NDescriptionsItem v-for="item in currentParameterItems" :key="item.key" :label="t(item.label)">
            {{ formatValue(item.value) }}
          </NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <NCard size="small">
        <RdiTelemetrySummary
          v-model:temperature-unit="temperatureUnit"
          :rows="telemetryRows"
          :temperature-unit-options="temperatureUnitOptions"
          :t="t"
        />
      </NCard>

      <!--
        REQ-48 / REQ-53：按 REQ-07 的四 Tab 约束，用电量统计与 Field Setting
        不再各占一个顶层 Tab，改为本页内的两个只读区块。
      -->
      <NCard :title="t('powerUsage')" size="small" data-testid="rdi-power-usage-section">
        <RdiDevicePowerConsumptionView :id="props.id" />
      </NCard>

      <NCard size="small" data-testid="rdi-field-settings-section">
        <RdiFieldSettingsPanel :id="props.id" />
      </NCard>
    </div>
  </NSpin>
</template>

<style scoped>
.rdi-device-details {
  display: grid;
  gap: 16px;
}

@media (max-width: 720px) {
  .rdi-device-details :deep(.n-descriptions-table) {
    table-layout: fixed;
  }
}
</style>
