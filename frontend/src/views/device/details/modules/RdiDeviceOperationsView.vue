<!--
  文件用途: RDI 设备详情操作视图，集中展示和操作 RDI 配置、遥测、历史、命令、分享和温度告警。
  核心逻辑: 组合多个 RDI composable，把“设备切换 -> 配置装载 -> 实时状态刷新 -> 能耗统计、命令/分享动作”收敛在单一详情视图中。
  关键链路:
  1. 挂载时并行启动配置读取、实时状态刷新和遥测轮询；OTA 包列表等到用户进入 OTA 操作区再按需加载。
  2. 用户在各 tab 中修改配置、执行命令、导出历史或生成分享链接时，统一复用 composable 暴露的动作函数。
  3. 设备 ID 变化时重置分享状态、清空在线缓存，并重新触发整套 RDI 数据装载。
  关键注意事项: 字段名、命令 payload、分享语义、在线状态限制和温度单位切换必须与 backend RDI API 保持一致。
  静态审查建议:
  - 当前页面聚合了配置、遥测、历史、命令、分享五类高副作用能力，后续新增逻辑时要优先判断是否还能继续下沉到 composable。
  - 设备切换后会重复启动轮询与加载链路，建议后续重点审查轮询清理、重复请求与旧状态残留风险。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { useRdiConfig } from './rdi/composables/useRdiConfig'
import { useRdiTelemetry } from './rdi/composables/useRdiTelemetry'
import { useRdiHistory } from './rdi/composables/useRdiHistory'
import { useRdiCommands } from './rdi/composables/useRdiCommands'
import { useRdiShare } from './rdi/composables/useRdiShare'
import { useRdiDeviceBasicInfo } from './rdi/composables/useRdiDeviceBasicInfo'
import { useRdiOnDemandLoads } from './rdi/composables/useRdiOnDemandLoads'
import RdiTelemetrySummary from './rdi/RdiTelemetrySummary.vue'
import RdiOperationsView from './RdiOperationsView.vue'

const ChartComponent = defineAsyncComponent(() => import('./telemetry/modules/ChartComponent.vue'))
const RdiTemperatureAlarmAxis = defineAsyncComponent(() => import('./RdiTemperatureAlarmAxis.vue'))

const props = defineProps<{
  id: string
  online?: number
  onlineUpdatedAt?: string
  deviceData?: Record<string, any>
}>()

const emit = defineEmits<{
  change: []
}>()

const deviceId = () => props.id
const onlineGetter = () => props.online
const deviceDataGetter = () => props.deviceData

const {
  loading,
  applyToDevice,
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
} = useRdiConfig(deviceId, () => emit('change'))

const {
  telemetry,
  liveOnlineStatus,
  temperatureUnit,
  telemetryRows,
  deviceOnlineText,
  deviceDescriptionText,
  formatSwitch,
  toAxisValue,
  loadRealtimeState,
  startTelemetryRefresh
} = useRdiTelemetry(deviceId, onlineGetter, deviceDataGetter, t)

const {
  RDI_DURATION_MAX_SECONDS,
  energyLoading,
  historyExportLoading,
  energyRange,
  energyCustomRange,
  historyChartSeriesKeys,
  historyExportKey,
  historyExportFormat,
  energyStats,
  historyChartOptions,
  energyRangeOptions,
  historyChartSeriesOptions,
  historyExportKeyOptions,
  historyExportFormatOptions,
  formatDurationLabel,
  formatEnergyValue,
  loadEnergyStatistics,
  exportHistoryData
} = useRdiHistory(deviceId, () => temperatureUnit.value, t)

const {
  commandLoading,
  dryCommandDelay,
  dryTestDuration,
  commandTrackingSummary,
  otaPackageLoading,
  otaPackageId,
  latestFirmwareLoading,
  latestFirmwarePackage,
  otaCommand,
  otaPackageOptions,
  otaMissingFieldLabels,
  canSendOtaUpgrade,
  setDryContact,
  testDryContact,
  sendFieldSetting,
  sendOtaUpgrade,
  sendUnbindDevice,
  sendFactoryReset,
  loadOtaPackages,
  applyLatestFirmwarePackage,
  checkLatestFirmware
} = useRdiCommands(deviceId, config, t)

const { shareLoading, shareExpiresIn, shareLink, shareExpiryOptions, shareExpiresAt, shareActions, resetShareState } =
  useRdiShare(deviceId, t)

const switchModeOptions = computed(() => [
  { label: t('poweredOn'), value: 'powered_on' },
  { label: t('poweredOff'), value: 'powered_off' },
  { label: t('disabled'), value: 'disabled' }
])

const temperatureUnitOptions = computed(() => [
  { label: `${t('celsius')} (C)`, value: 'C' },
  { label: `${t('fahrenheit')} (F)`, value: 'F' }
])

const levelOptions = computed(() => [
  { label: t('high'), value: 'high' },
  { label: t('low'), value: 'low' }
])

const { isDeviceOnline, basicInfoColumns } = useRdiDeviceBasicInfo({
  deviceId,
  online: () => props.online,
  onlineUpdatedAt: () => props.onlineUpdatedAt,
  deviceData: () => props.deviceData,
  liveOnlineStatus,
  deviceOnlineText,
  deviceDescriptionText,
  t
})

const dryContactHasDistinctRecoveryDelay = computed(
  () => config.dry_contact_alarm_delay !== config.dry_contact_normal_delay
)

const dryContactTriggerEffectiveTime = computed({
  get: () => config.dry_contact_alarm_delay,
  set: value => {
    config.dry_contact_alarm_delay = value
    config.dry_contact_normal_delay = value
  }
})

const {
  hasLoadedEnergyStatistics,
  loadEnergyStatisticsOnDemand,
  ensureOtaPackagesLoaded,
  reloadOtaPackages,
  loadConfigAndRefresh
} = useRdiOnDemandLoads({
  deviceId,
  loadConfig,
  loadRealtimeState,
  loadEnergyStatistics,
  loadOtaPackages,
  otaPackageLoading,
  resetShareState,
  liveOnlineStatus,
  startTelemetryRefresh
})
</script>

<template>
  <div class="rdi-device-operations-view">
    <NSpin :show="loading">
      <div class="rdi-header">
        <div>
          <div class="rdi-title">{{ t('rdiSettings') }}</div>
          <div class="rdi-meta">
            <span>{{ t('pid') }}: {{ props.deviceData?.device_number || '--' }}</span>
            <span>{{ t('firmware') }}: {{ props.deviceData?.current_version || '--' }}</span>
            <span>{{ t('connection') }}: {{ props.deviceData?.protocol || '--' }}</span>
            <span>{{ deviceOnlineText }}</span>
            <span class="rdi-meta-description">{{ t('description') }}: {{ deviceDescriptionText }}</span>
          </div>
        </div>
        <NButton @click="loadConfigAndRefresh">{{ t('refresh') }}</NButton>
      </div>

      <section class="rdi-section rdi-basic-info-section">
        <div class="rdi-basic-info-header">
          <div class="rdi-section-title">{{ t('basicInfo') }}</div>
        </div>
        <div class="rdi-basic-info-layout">
          <div
            v-for="(column, columnIndex) in basicInfoColumns"
            :key="`basic-info-column-${columnIndex}`"
            class="rdi-basic-info-column"
          >
            <div
              v-for="item in column"
              :key="item.key"
              class="rdi-basic-info-row"
              :class="`rdi-basic-info-row--${item.key}`"
            >
              <span class="rdi-basic-info-label">{{ item.label }}</span>
              <div class="rdi-basic-info-value" :class="item.kind ? `rdi-basic-info-value--${item.kind}` : ''">
                <template v-if="item.kind === 'status'">
                  <span
                    class="rdi-basic-info-status-dot"
                    :class="{ 'rdi-basic-info-status-dot--online': isDeviceOnline }"
                    aria-hidden="true"
                  />
                  <strong>{{ item.value }}</strong>
                </template>
                <span v-else-if="item.kind === 'chip'" class="rdi-basic-info-chip">{{ item.value }}</span>
                <strong v-else>{{ item.value }}</strong>
              </div>
            </div>
          </div>
        </div>
      </section>

      <RdiTelemetrySummary
        v-model:temperature-unit="temperatureUnit"
        :rows="telemetryRows"
        :temperature-unit-options="temperatureUnitOptions"
        :t="t"
      />

      <section class="rdi-section rdi-feature-tabs-section">
        <NTabs type="line" animated class="rdi-feature-tabs">
          <NTabPane name="electricity-statistics" :tab="t('energy')">
            <div class="rdi-tab-pane">
              <div class="rdi-energy-toolbar">
                <NSelect v-model:value="energyRange" :options="energyRangeOptions" class="rdi-select" />
                <NSelect
                  v-model:value="historyChartSeriesKeys"
                  multiple
                  :options="historyChartSeriesOptions"
                  :placeholder="t('historyKey')"
                  class="rdi-select rdi-series-select"
                  max-tag-count="responsive"
                />
                <NSelect v-model:value="historyExportKey" :options="historyExportKeyOptions" class="rdi-select" />
                <NSelect
                  v-model:value="historyExportFormat"
                  :options="historyExportFormatOptions"
                  :aria-label="t('exportFormat')"
                  class="rdi-select"
                />
                <NDatePicker
                  v-if="energyRange === 'custom'"
                  v-model:value="energyCustomRange"
                  type="datetimerange"
                  class="rdi-date-range"
                />
                <NButton :loading="energyLoading" @click="loadEnergyStatisticsOnDemand">{{ t('load') }}</NButton>
                <NButton :loading="historyExportLoading" @click="exportHistoryData">{{ t('exportData') }}</NButton>
              </div>
              <div class="rdi-telemetry-grid">
                <div class="rdi-telemetry-cell">
                  <span>{{ t('latest') }}</span>
                  <strong>{{ formatEnergyValue(energyStats.latest) }}</strong>
                </div>
                <div class="rdi-telemetry-cell">
                  <span>{{ t('delta') }}</span>
                  <strong>{{ formatEnergyValue(energyStats.delta) }}</strong>
                </div>
                <div class="rdi-telemetry-cell">
                  <span>{{ t('minMax') }}</span>
                  <strong>{{ formatEnergyValue(energyStats.min) }} / {{ formatEnergyValue(energyStats.max) }}</strong>
                </div>
                <div class="rdi-telemetry-cell">
                  <span>{{ t('dataPoints') }}</span>
                  <strong>{{ energyStats.sample_count }}</strong>
                </div>
              </div>
              <NSpin :show="energyLoading">
                <div class="rdi-history-chart">
                  <ChartComponent v-if="hasLoadedEnergyStatistics" :initial-options="historyChartOptions" />
                  <NEmpty v-else :description="t('empty')" />
                </div>
              </NSpin>
            </div>
          </NTabPane>
          <NTabPane name="field-setting" :tab="t('field')">
            <div class="rdi-tab-pane">
              <div class="rdi-grid rdi-grid--two">
                <div class="rdi-fieldset">
                  <div class="rdi-fieldset-title">{{ t('nFields') }}</div>
                  <div class="rdi-field-grid">
                    <NInput
                      v-for="index in 8"
                      :key="`n${index - 1}`"
                      :value="getFieldValue(`n0${index - 1}`)"
                      :placeholder="`n0${index - 1}`"
                      @update:value="(value) => setFieldValue(`n0${index - 1}`, value)"
                    />
                  </div>
                </div>
                <div class="rdi-fieldset">
                  <div class="rdi-fieldset-title">{{ t('swFields') }}</div>
                  <div class="rdi-field-grid">
                    <NInput
                      v-for="index in 4"
                      :key="`sw${index}`"
                      :value="getFieldValue(`sw${index}`)"
                      :placeholder="`sw${index}`"
                      @update:value="(value) => setFieldValue(`sw${index}`, value)"
                    />
                  </div>
                </div>
              </div>
              <div v-if="fieldEntries.length" class="rdi-tags">
                <NTag v-for="[key, value] in fieldEntries" :key="key" size="small">{{ key }}={{ value }}</NTag>
              </div>
              <div class="rdi-section-actions">
                <NButton :loading="commandLoading" @click="sendFieldSetting">{{ t('sendField') }}</NButton>
              </div>
            </div>
          </NTabPane>
        </NTabs>
      </section>

      <section class="rdi-section">
        <div class="rdi-section-title">{{ t('alarm') }}</div>
        <div class="rdi-grid rdi-grid--two">
          <div class="rdi-fieldset">
            <div class="rdi-fieldset-title">{{ t('sensor1') }}</div>
            <NFormItem :label="t('enabled')">
              <NSwitch v-model:value="config.alarm_sensor_1_enabled" />
            </NFormItem>
            <NSlider v-model:value="sensor1Range" range :min="-40" :max="125" />
            <RdiTemperatureAlarmAxis
              v-if="config.alarm_sensor_1_enabled"
              v-model:lower="config.sensor_1_lower"
              v-model:upper="config.sensor_1_upper"
              :current="toAxisValue(telemetry.temperature_1)"
              :unit="temperatureUnit"
              :lower-label="t('lower')"
              :upper-label="t('upper')"
              :current-label="t('currentValue')"
            />
            <div class="rdi-inline">
              <NFormItem :label="t('lower')">
                <NInputNumber v-model:value="config.sensor_1_lower" :min="-40" :max="125" />
              </NFormItem>
              <NFormItem :label="t('upper')">
                <NInputNumber v-model:value="config.sensor_1_upper" :min="-40" :max="125" />
              </NFormItem>
              <NFormItem :label="`${t('duration')} (s)`" class="rdi-duration-field">
                <div class="rdi-duration-control">
                  <div class="rdi-duration-value">{{ formatDurationLabel(config.sensor_1_duration) }}</div>
                  <NSlider
                    v-model:value="config.sensor_1_duration"
                    :min="0"
                    :max="RDI_DURATION_MAX_SECONDS"
                    :step="60"
                    :format-tooltip="formatDurationLabel"
                  />
                  <div class="rdi-duration-labels">
                    <span>0H</span>
                    <span>24H</span>
                  </div>
                  <NInputNumber v-model:value="config.sensor_1_duration" :min="0" :max="RDI_DURATION_MAX_SECONDS" />
                </div>
              </NFormItem>
            </div>
          </div>

          <div class="rdi-fieldset">
            <div class="rdi-fieldset-title">{{ t('sensor2') }}</div>
            <NFormItem :label="t('enabled')">
              <NSwitch v-model:value="config.alarm_sensor_2_enabled" />
            </NFormItem>
            <NSlider v-model:value="sensor2Range" range :min="-40" :max="125" />
            <RdiTemperatureAlarmAxis
              v-if="config.alarm_sensor_2_enabled"
              v-model:lower="config.sensor_2_lower"
              v-model:upper="config.sensor_2_upper"
              :current="toAxisValue(telemetry.temperature_2)"
              :unit="temperatureUnit"
              :lower-label="t('lower')"
              :upper-label="t('upper')"
              :current-label="t('currentValue')"
            />
            <div class="rdi-inline">
              <NFormItem :label="t('lower')">
                <NInputNumber v-model:value="config.sensor_2_lower" :min="-40" :max="125" />
              </NFormItem>
              <NFormItem :label="t('upper')">
                <NInputNumber v-model:value="config.sensor_2_upper" :min="-40" :max="125" />
              </NFormItem>
              <NFormItem :label="`${t('duration')} (s)`" class="rdi-duration-field">
                <div class="rdi-duration-control">
                  <div class="rdi-duration-value">{{ formatDurationLabel(config.sensor_2_duration) }}</div>
                  <NSlider
                    v-model:value="config.sensor_2_duration"
                    :min="0"
                    :max="RDI_DURATION_MAX_SECONDS"
                    :step="60"
                    :format-tooltip="formatDurationLabel"
                  />
                  <div class="rdi-duration-labels">
                    <span>0H</span>
                    <span>24H</span>
                  </div>
                  <NInputNumber v-model:value="config.sensor_2_duration" :min="0" :max="RDI_DURATION_MAX_SECONDS" />
                </div>
              </NFormItem>
            </div>
          </div>

          <div class="rdi-fieldset">
            <div class="rdi-fieldset-title">{{ t('switch1') }}</div>
            <NFormItem :label="t('switchAlarmLevel')">
              <NSelect v-model:value="config.switch_1_alarm_mode" :options="switchModeOptions" />
            </NFormItem>
            <NFormItem :label="`${t('triggerEffectiveTime')} (s)`" class="rdi-duration-field">
              <div class="rdi-duration-control">
                <div class="rdi-duration-value">{{ formatDurationLabel(config.switch_1_alarm_duration) }}</div>
                <NSlider
                  v-model:value="config.switch_1_alarm_duration"
                  :min="0"
                  :max="RDI_DURATION_MAX_SECONDS"
                  :step="60"
                  :format-tooltip="formatDurationLabel"
                />
                <div class="rdi-duration-labels">
                  <span>0H</span>
                  <span>24H</span>
                </div>
                <NInputNumber v-model:value="config.switch_1_alarm_duration" :min="0" :max="RDI_DURATION_MAX_SECONDS" />
              </div>
            </NFormItem>
            <NFormItem :label="t('currentStatus')">
              <div class="rdi-status-value">{{ formatSwitch(telemetry.switch_1) }}</div>
            </NFormItem>
          </div>

          <div class="rdi-fieldset">
            <div class="rdi-fieldset-title">{{ t('switch2') }}</div>
            <NFormItem :label="t('switchAlarmLevel')">
              <NSelect v-model:value="config.switch_2_alarm_mode" :options="switchModeOptions" />
            </NFormItem>
            <NFormItem :label="`${t('triggerEffectiveTime')} (s)`" class="rdi-duration-field">
              <div class="rdi-duration-control">
                <div class="rdi-duration-value">{{ formatDurationLabel(config.switch_2_alarm_duration) }}</div>
                <NSlider
                  v-model:value="config.switch_2_alarm_duration"
                  :min="0"
                  :max="RDI_DURATION_MAX_SECONDS"
                  :step="60"
                  :format-tooltip="formatDurationLabel"
                />
                <div class="rdi-duration-labels">
                  <span>0H</span>
                  <span>24H</span>
                </div>
                <NInputNumber v-model:value="config.switch_2_alarm_duration" :min="0" :max="RDI_DURATION_MAX_SECONDS" />
              </div>
            </NFormItem>
            <NFormItem :label="t('currentStatus')">
              <div class="rdi-status-value">{{ formatSwitch(telemetry.switch_2) }}</div>
            </NFormItem>
          </div>
        </div>
      </section>

      <section class="rdi-section">
        <div class="rdi-section-title">{{ t('dryContact') }}</div>
        <div class="rdi-grid rdi-grid--four">
          <NFormItem :label="t('alarmLevel')">
            <NSelect v-model:value="config.dry_contact_alarm_level" :options="levelOptions" />
          </NFormItem>
          <NFormItem :label="t('normalLevel')">
            <NSelect v-model:value="config.dry_contact_normal_level" :options="levelOptions" />
          </NFormItem>
          <NFormItem
            v-if="!dryContactHasDistinctRecoveryDelay"
            :label="`${t('triggerEffectiveTime')} (s)`"
            class="rdi-duration-field"
          >
            <div class="rdi-duration-control">
              <div class="rdi-duration-value">{{ formatDurationLabel(dryContactTriggerEffectiveTime) }}</div>
              <NSlider
                v-model:value="dryContactTriggerEffectiveTime"
                :min="0"
                :max="RDI_DURATION_MAX_SECONDS"
                :step="60"
                :format-tooltip="formatDurationLabel"
              />
              <div class="rdi-duration-labels">
                <span>0H</span>
                <span>24H</span>
              </div>
              <NInputNumber v-model:value="dryContactTriggerEffectiveTime" :min="0" :max="RDI_DURATION_MAX_SECONDS" />
            </div>
          </NFormItem>
          <NFormItem v-else :label="`${t('alarmDelay')} (s)`" class="rdi-duration-field">
            <div class="rdi-duration-control">
              <div class="rdi-duration-value">{{ formatDurationLabel(config.dry_contact_alarm_delay) }}</div>
              <NSlider
                v-model:value="config.dry_contact_alarm_delay"
                :min="0"
                :max="RDI_DURATION_MAX_SECONDS"
                :step="60"
                :format-tooltip="formatDurationLabel"
              />
              <div class="rdi-duration-labels">
                <span>0H</span>
                <span>24H</span>
              </div>
              <NInputNumber v-model:value="config.dry_contact_alarm_delay" :min="0" :max="RDI_DURATION_MAX_SECONDS" />
            </div>
          </NFormItem>
          <NFormItem v-if="dryContactHasDistinctRecoveryDelay" :label="`${t('normalDelay')} (s)`" class="rdi-duration-field">
            <div class="rdi-duration-control">
              <div class="rdi-duration-value">{{ formatDurationLabel(config.dry_contact_normal_delay) }}</div>
              <NSlider
                v-model:value="config.dry_contact_normal_delay"
                :min="0"
                :max="RDI_DURATION_MAX_SECONDS"
                :step="60"
                :format-tooltip="formatDurationLabel"
              />
              <div class="rdi-duration-labels">
                <span>0H</span>
                <span>24H</span>
              </div>
              <NInputNumber v-model:value="config.dry_contact_normal_delay" :min="0" :max="RDI_DURATION_MAX_SECONDS" />
            </div>
          </NFormItem>
          <NFormItem :label="t('currentStatus')">
            <div class="rdi-status-value">{{ formatSwitch(telemetry.dry_contact_output) }}</div>
          </NFormItem>
        </div>
      </section>

      <section class="rdi-section">
        <div class="rdi-section-title">{{ t('notification') }}</div>
        <NAlert type="info" class="rdi-notification-hint" :show-icon="false">
          {{ t('notificationFallbackHint') }}
        </NAlert>
        <div class="rdi-grid rdi-grid--two">
          <NFormItem :label="t('enabled')">
            <NSwitch v-model:value="config.notification_enabled" />
          </NFormItem>
          <NFormItem :label="t('temperatureAlarmNotice')">
            <NSwitch v-model:value="config.notification_temperature_alarm" />
          </NFormItem>
          <NFormItem :label="t('switchAlarmNotice')">
            <NSwitch v-model:value="config.notification_switch_alarm" />
          </NFormItem>
          <NFormItem :label="t('warrantyAlarmNotice')">
            <NSwitch v-model:value="config.notification_warranty_alarm" />
          </NFormItem>
          <NFormItem :label="t('interval')">
            <NInputNumber v-model:value="config.data_collection_interval" :min="45" :max="60" />
          </NFormItem>
          <NFormItem :label="t('temperatureMail')">
            <NInput v-model:value="config.sensor_alarm_emails" />
          </NFormItem>
          <NFormItem :label="t('switchMail')">
            <NInput v-model:value="config.switch_alarm_emails" />
          </NFormItem>
          <NFormItem :label="t('warrantyMail')">
            <NInput v-model:value="config.warranty_alarm_emails" />
          </NFormItem>
          <NFormItem :label="t('sensor1Mail')">
            <NInput v-model:value="config.sensor_1_alarm_emails" />
          </NFormItem>
          <NFormItem :label="t('sensor2Mail')">
            <NInput v-model:value="config.sensor_2_alarm_emails" />
          </NFormItem>
          <NFormItem :label="t('switch1Mail')">
            <NInput v-model:value="config.switch_1_alarm_emails" />
          </NFormItem>
          <NFormItem :label="t('switch2Mail')">
            <NInput v-model:value="config.switch_2_alarm_emails" />
          </NFormItem>
        </div>
      </section>

      <section class="rdi-section">
        <div class="rdi-section-title">{{ t('system') }}</div>
        <div class="rdi-grid rdi-grid--three">
          <NFormItem :label="t('location')">
            <NInput v-model:value="systemInfo.installation_location" />
          </NFormItem>
          <NFormItem :label="systemExtraFieldLabel('address')">
            <NInput v-model:value="systemInfo.address" />
          </NFormItem>
          <NFormItem :label="systemExtraFieldLabel('installation_date')">
            <NInput v-model:value="systemInfo.installation_date" />
          </NFormItem>
          <NFormItem :label="systemExtraFieldLabel('installer_company')">
            <NInput v-model:value="systemInfo.installer_company" />
          </NFormItem>
          <NFormItem :label="systemExtraFieldLabel('installer_contact')">
            <NInput v-model:value="systemInfo.installer_contact" />
          </NFormItem>
          <NFormItem :label="systemExtraFieldLabel('installer_name')">
            <NInput v-model:value="systemInfo.installer_name" />
          </NFormItem>
          <NFormItem :label="systemExtraFieldLabel('installer_phone')">
            <NInput v-model:value="systemInfo.installer_phone" />
          </NFormItem>
          <NFormItem :label="systemExtraFieldLabel('installer_email')">
            <NInput v-model:value="systemInfo.installer_email" />
          </NFormItem>
          <NFormItem :label="systemExtraFieldLabel('controller_serial_number')">
            <NInput v-model:value="systemInfo.controller_serial_number" />
          </NFormItem>
          <NFormItem :label="t('technician')">
            <NInput v-model:value="systemInfo.maintenance_technician" />
          </NFormItem>
          <NFormItem :label="t('customer')">
            <NInput v-model:value="systemInfo.customer_name" />
          </NFormItem>
          <NFormItem :label="t('email')">
            <NInput v-model:value="systemInfo.contact_email" />
          </NFormItem>
          <NFormItem :label="t('phone')">
            <NInput v-model:value="systemInfo.contact_phone" />
          </NFormItem>
          <NFormItem :label="t('warranty')">
            <NInput v-model:value="systemInfo.warranty_status" />
          </NFormItem>
        </div>
        <div class="rdi-fieldset rdi-system-extra">
          <div class="rdi-fieldset-title">{{ t('extendedFields') }}</div>
          <div class="rdi-grid rdi-grid--three">
            <NFormItem
              v-for="field in systemExtraFieldDefinitions"
              :key="field.key"
              :label="systemExtraFieldLabel(field.key)"
            >
              <NInput
                :value="getSystemExtraField(field.key)"
                @update:value="(value) => setSystemExtraField(field.key, value)"
              />
            </NFormItem>
          </div>
        </div>
      </section>

      <RdiOperationsView
        :command-loading="commandLoading"
        :command-tracking-summary="commandTrackingSummary"
        :dry-command-delay="dryCommandDelay"
        :dry-test-duration="dryTestDuration"
        :latest-firmware-loading="latestFirmwareLoading"
        :latest-firmware-package="latestFirmwarePackage"
        :can-send-ota-upgrade="canSendOtaUpgrade"
        :ota-command="otaCommand"
        :ota-missing-field-labels="otaMissingFieldLabels"
        :ota-package-id="otaPackageId"
        :ota-package-loading="otaPackageLoading"
        :ota-package-options="otaPackageOptions"
        :share-expires-at="shareExpiresAt"
        :share-expires-in="shareExpiresIn"
        :share-expiry-options="shareExpiryOptions"
        :share-link="shareLink"
        :share-loading="shareLoading"
        :t="t"
        @apply-latest-firmware="applyLatestFirmwarePackage"
        @check-latest-firmware="checkLatestFirmware"
        @copy-share="shareActions.copy"
        @create-share="shareActions.create"
        @ensure-ota-packages="ensureOtaPackagesLoaded"
        @load-ota-packages="reloadOtaPackages"
        @send-factory-reset="sendFactoryReset"
        @send-ota-upgrade="sendOtaUpgrade"
        @send-unbind-device="sendUnbindDevice"
        @set-dry-contact="setDryContact"
        @test-dry-contact="testDryContact"
        @update:dry-command-delay="dryCommandDelay = $event"
        @update:dry-test-duration="dryTestDuration = $event"
        @update:ota-package-id="otaPackageId = $event"
        @update:share-expires-in="shareExpiresIn = $event"
      />

      <div class="rdi-footer">
        <NCheckbox v-model:checked="applyToDevice">{{ t('apply') }}</NCheckbox>
        <NButton type="primary" :loading="loading" @click="saveConfig">{{ t('save') }}</NButton>
      </div>
      <NAlert v-if="configCommandTrackingSummary" type="info" class="rdi-command-tracking" :show-icon="false">
        {{ configCommandTrackingSummary }}
      </NAlert>
    </NSpin>
  </div>
</template>

<style scoped>
.rdi-device-operations-view {
  width: 100%;
}

.rdi-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.rdi-title {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.3;
}

.rdi-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  margin-top: 6px;
  color: #667085;
}

.rdi-meta-description {
  min-width: min(100%, 260px);
  overflow-wrap: anywhere;
}

.rdi-section {
  border-top: 1px solid #e5e7eb;
  padding: 16px 0;
}

.rdi-basic-info-section {
  padding-top: 12px;
}

.rdi-basic-info-header {
  margin-bottom: 16px;
  padding-bottom: 10px;
  border-bottom: 2px solid #3b82f6;
}

.rdi-section-title {
  margin-bottom: 12px;
  font-size: 15px;
  font-weight: 600;
}

.rdi-basic-info-header .rdi-section-title {
  margin-bottom: 0;
  font-size: 16px;
}

.rdi-basic-info-layout {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 18px 48px;
}

.rdi-basic-info-column {
  display: grid;
  gap: 18px;
}

.rdi-basic-info-row {
  display: grid;
  gap: 6px;
  min-height: 56px;
}

.rdi-basic-info-label {
  color: #667085;
  font-size: 12px;
  font-weight: 500;
}

.rdi-basic-info-value {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
}

.rdi-basic-info-value strong {
  color: #0f172a;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.rdi-basic-info-value--status {
  gap: 10px;
}

.rdi-basic-info-status-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  flex: 0 0 auto;
  background: #ff6b72;
  box-shadow: 0 0 0 3px rgb(255 107 114 / 16%);
}

.rdi-basic-info-status-dot--online {
  background: #16a34a;
  box-shadow: 0 0 0 3px rgb(22 163 74 / 16%);
}

.rdi-basic-info-chip {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  max-width: 100%;
  padding: 4px 12px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #0f172a;
  font-size: 14px;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.rdi-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.rdi-section-header .rdi-section-title {
  margin-bottom: 0;
}

.rdi-feature-tabs-section {
  padding-bottom: 12px;
}

.rdi-feature-tabs {
  --n-tab-gap: 18px;
}

.rdi-tab-pane {
  padding-top: 4px;
}

.rdi-telemetry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}

.rdi-telemetry-cell {
  display: flex;
  min-height: 56px;
  flex-direction: column;
  justify-content: center;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 8px 10px;
  background: #f8fafc;
}

.rdi-telemetry-cell span {
  color: #667085;
  font-size: 12px;
}

.rdi-telemetry-cell strong {
  margin-top: 4px;
  font-size: 16px;
}

.rdi-grid {
  display: grid;
  gap: 14px;
}

.rdi-grid--two {
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
}

.rdi-grid--three {
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.rdi-grid--four {
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
}

.rdi-fieldset {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 12px;
}

.rdi-fieldset-title {
  margin-bottom: 10px;
  font-weight: 600;
}

.rdi-system-extra {
  margin-top: 12px;
}

.rdi-inline {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
  gap: 10px;
  margin-top: 10px;
}

.rdi-field-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(88px, 1fr));
  gap: 8px;
}

.rdi-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.rdi-section-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.rdi-energy-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.rdi-history-chart {
  width: 100%;
  height: 360px;
  min-height: 320px;
  margin-top: 16px;
}

.rdi-select {
  width: 168px;
}

.rdi-date-range {
  width: 320px;
}

.rdi-duration-field {
  min-width: 260px;
}

.rdi-duration-control {
  display: grid;
  width: 100%;
  gap: 8px;
}

.rdi-duration-value {
  color: #344054;
  font-size: 12px;
  font-weight: 600;
}

.rdi-duration-labels {
  display: flex;
  justify-content: space-between;
  color: #98a2b3;
  font-size: 11px;
  line-height: 1.2;
}

.rdi-duration-control :deep(.n-input-number) {
  width: 100%;
}

.rdi-status-value {
  color: #344054;
  font-size: 14px;
  font-weight: 600;
  line-height: 34px;
}

.rdi-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 14px;
  border-top: 1px solid #e5e7eb;
  padding-top: 16px;
}

.rdi-command-tracking {
  margin-top: 12px;
  overflow-wrap: anywhere;
}

:deep(.n-form-item) {
  margin-bottom: 0;
}

@media (max-width: 900px) {
  .rdi-date-range,
  .rdi-select {
    width: 100%;
  }
}
</style>
