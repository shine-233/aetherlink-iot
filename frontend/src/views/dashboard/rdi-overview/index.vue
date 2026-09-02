<!--
  文件用途：承载 frontend/src/views/dashboard/rdi-overview/index.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NEmpty, NTag } from 'naive-ui'
import dayjs from 'dayjs'
import { acknowledgeAlarmHistory, resetAlarmHistory } from '@/service/api/alarm'
import { useAuthStore } from '@/store/modules/auth'
import { isSysAdminUser } from '@/utils/thingsvis/space'
import { $t } from '@/locales'
import {
  RDI_SNAPSHOT_LIMIT,
  alarmStatusLabel as resolveAlarmStatusLabel,
  alarmTagType as resolveAlarmTagType,
  alarmTypeLabel as resolveAlarmTypeLabel,
  buildOperationsFocus,
  formatSwitch as resolveSwitchLabel,
  formatTemperature as resolveTemperatureLabel,
  isAcknowledgedAlarm,
  isRowOnline as resolveRowOnline,
  normalizeDeviceRows as resolveDeviceRows,
  normalizeTelemetry as resolveTelemetry,
  parseAlarmRemark as resolveAlarmRemark,
  rowText as resolveRowText,
  type AlarmRecord,
  type DeviceSnapshot,
  type TemperatureUnit
} from './rdiOverviewState'
import { buildAlarmTrendChartOptions, buildAlarmTrendYearOptions } from './rdiTrendChart'
import { useRdiOverviewData } from './useRdiOverviewData'
import { useRdiSnapshotFilters } from './useRdiSnapshotFilters'
import { useRdiDeviceSnapshots } from './useRdiDeviceSnapshots'

const props = withDefaults(
  defineProps<{
    activeSystemsOnly?: boolean
  }>(),
  {
    activeSystemsOnly: false
  }
)

const ChartComponent = defineAsyncComponent(() => import('@/components/custom/ChartComponent.vue'))
const router = useRouter()
const authStore = useAuthStore()
const isMasterAccount = computed(() => isSysAdminUser(authStore.userInfo))
const temperatureUnit = ref<TemperatureUnit>(readTemperatureUnitPreference())

const {
  loading,
  alarmLoading,
  alarmTrendLoading,
  alarms,
  alarmTrendPoints,
  alarmTrendYear,
  alarmDeviceTotal,
  stats,
  queryParams,
  alarmPagination,
  fetchDevices,
  fetchCounts,
  fetchActiveAlarmCounts,
  refreshAlarmSummaryCounts,
  fetchAlarms,
  searchAlarms,
  fetchAlarmTrend,
  resetAlarmFilter
} = useRdiOverviewData({ isMasterAccount })

const {
  snapshotLoading,
  deviceSnapshots,
  snapshotPage,
  snapshotTotal,
  fetchDeviceSnapshots,
  changeSnapshotPage,
  scheduleDeviceSnapshotsRefresh,
  cancelScheduledDeviceSnapshots,
  dispose: disposeDeviceSnapshots
} = useRdiDeviceSnapshots({
  activeSystemsOnly: () => props.activeSystemsOnly,
  isMasterAccount
})

const {
  snapshotFilterKeyword,
  snapshotFilterStatus,
  snapshotFilterAlarmLevel,
  snapshotFilterGroupId,
  snapshotFilterAdvancedVisible,
  snapshotGroupOptions,
  snapshotStatusOptions,
  snapshotAlarmLevelOptions,
  snapshotHasActiveFilters,
  snapshotActiveFilterChips,
  visibleDeviceSnapshots,
  resetSnapshotFilters,
  removeSnapshotFilter,
  toggleSnapshotAdvancedFilters,
  loadSnapshotGroupOptions
} = useRdiSnapshotFilters({ deviceSnapshots })

function readTemperatureUnitPreference(): TemperatureUnit {
  try {
    return typeof window !== 'undefined' && window.localStorage.getItem('rdi-temperature-unit') === 'F' ? 'F' : 'C'
  } catch {
    return 'C'
  }
}

const operationsFocus = computed(() => buildOperationsFocus(stats, alarmDeviceTotal.value))
type OperationsAlertType = 'default' | 'info' | 'success' | 'warning' | 'error'
type OperationsTagType = OperationsAlertType | 'primary'
const operationsFocusAlertType = computed<OperationsAlertType>(() => operationsFocus.value.type as OperationsAlertType)
const operationsFocusTags = computed(() =>
  operationsFocus.value.tags.map(tag => ({ ...tag, type: tag.type as OperationsTagType }))
)
const systemsCardTitleKey = computed(() =>
  props.activeSystemsOnly ? 'rdi.overview.activeSystems' : 'rdi.overview.allSystems'
)
const systemsEmptyTitleKey = computed(() =>
  props.activeSystemsOnly ? 'rdi.overview.noActiveSystemsTitle' : 'rdi.overview.noSnapshotTitle'
)
const systemsEmptyDescriptionKey = computed(() =>
  props.activeSystemsOnly ? 'rdi.overview.noActiveSystemsDesc' : 'rdi.overview.noSnapshotDesc'
)

const temperatureUnitOptions = computed(() => [
  { label: 'Celsius (C)', value: 'C' },
  { label: 'Fahrenheit (F)', value: 'F' }
])

const alarmTrendYearOptions = computed(() => buildAlarmTrendYearOptions(dayjs().year()))

function parseAlarmRemark(raw: unknown) {
  return resolveAlarmRemark(raw)
}

function isAcknowledged(row: AlarmRecord) {
  return isAcknowledgedAlarm(row)
}

function formatTime(value?: string) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function alarmStatusLabel(status?: string) {
  return resolveAlarmStatusLabel(status, $t)
}

function alarmTagType(status?: string) {
  return resolveAlarmTagType(status)
}

function alarmTypeLabel(row: AlarmRecord) {
  return resolveAlarmTypeLabel(row, $t)
}

function goDevice(deviceId?: string) {
  if (!deviceId) return
  router.push({ name: 'device_details', query: { d_id: deviceId, tab: 'message' } })
}

function openDeviceManage() {
  router.push('/device/manage')
}

function openServiceAccess() {
  router.push('/device/service-access')
}

function normalizeDeviceRows(payload: any): Record<string, unknown>[] {
  return resolveDeviceRows(payload)
}

function normalizeTelemetry(payload: unknown) {
  return resolveTelemetry(payload)
}

function rowText(row: Record<string, unknown>, keys: string[], fallback = '--') {
  return resolveRowText(row, keys, fallback)
}

function isRowOnline(row: Record<string, unknown>) {
  return resolveRowOnline(row)
}

function snapshotStatusLabel(device: DeviceSnapshot) {
  if (device.alarm === true) return $t('custom.devicePage.alarmed')
  if (!device.online) return $t('rdi.overview.offline')
  if (device.alarm === false) return $t('rdi.overview.normal')
  return $t('rdi.overview.online')
}

function snapshotStatusTagType(device: DeviceSnapshot) {
  if (device.alarm === true) return 'error'
  if (!device.online) return 'default'
  if (device.alarm === false) return 'success'
  return 'info'
}

function formatTemperature(value: unknown) {
  return resolveTemperatureLabel(value, temperatureUnit.value)
}

function formatSwitch(value: unknown) {
  return resolveSwitchLabel(value, $t)
}

function hasInstallationInfo(device: DeviceSnapshot) {
  return [
    device.serialNumber,
    device.installLocation,
    device.installAddress,
    device.installDate,
    device.installerName,
    device.installerContact,
    device.adminName
  ].some((value) => value && value !== '--')
}

function goBack() {
  router.back()
}

async function acknowledgeAlarm(row: AlarmRecord) {
  await acknowledgeAlarmHistory(row.id)
  window.$message?.success($t('rdi.overview.alarmAcknowledged'))
  await fetchAlarms()
}

async function resetAlarm(row: AlarmRecord) {
  window.$dialog?.warning({
    title: $t('rdi.overview.resetAlarm'),
    content: row.name || row.content || row.id,
    positiveText: $t('common.reset'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      await resetAlarmHistory(row.id)
      window.$message?.success($t('rdi.overview.alarmReset'))
      await fetchAlarms()
      await refreshAlarmSummaryCounts()
      if (props.activeSystemsOnly) {
        snapshotPage.value = 1
      }
      scheduleDeviceSnapshotsRefresh()
    }
  })
}

const alarmColumns: DataTableColumns<AlarmRecord> = [
  {
    key: 'create_at',
    title: () => $t('common.time'),
    minWidth: 170,
    render: (row) => formatTime(row.create_at)
  },
  {
    key: 'name',
    title: () => $t('rdi.overview.alarm'),
    minWidth: 180,
    render: (row) => row.name || row.content || '-'
  },
  {
    key: 'alarm_status',
    title: () => $t('common.alarm_level'),
    width: 120,
    render: (row) =>
      h(NTag, { type: alarmTagType(row.alarm_status) }, { default: () => alarmStatusLabel(row.alarm_status) })
  },
  {
    key: 'alarm_type',
    title: () => $t('rdi.overview.alarmType'),
    minWidth: 160,
    render: (row) => alarmTypeLabel(row)
  },
  {
    key: 'devices',
    title: () => $t('rdi.overview.device'),
    minWidth: 180,
    render: (row) => {
      const firstDevice = row.alarm_device_list?.[0]
      if (!firstDevice) return '-'
      const tenantId = rowText(row as unknown as Record<string, unknown>, ['tenant_id', 'TenantID'], '')
      const deviceLabel = firstDevice.name || firstDevice.id
      const scopedDeviceLabel = isMasterAccount.value && tenantId ? `${deviceLabel} · ${tenantId}` : deviceLabel
      return h(
        NButton,
        { text: true, type: 'primary', onClick: () => goDevice(firstDevice.id) },
        { default: () => scopedDeviceLabel }
      )
    }
  },
  {
    key: 'description',
    title: () => $t('rdi.overview.description'),
    minWidth: 200,
    ellipsis: { tooltip: true },
    render: (row) => row.description || '-'
  },
  {
    key: 'actions',
    title: () => $t('common.actions'),
    width: 180,
    render: (row) =>
      h('div', { class: 'action-row' }, [
        h(
          NButton,
          { size: 'small', type: 'success', disabled: isAcknowledged(row), onClick: () => acknowledgeAlarm(row) },
          { default: () => $t('rdi.overview.acknowledgeAlarm') }
        ),
        h(
          NButton,
          { size: 'small', type: 'error', disabled: row.alarm_status === 'N', onClick: () => resetAlarm(row) },
          { default: () => $t('rdi.overview.resetAlarm') }
        )
      ])
  }
]

const alarmStatusOptions = computed(() => [
  { label: $t('rdi.overview.active'), value: 'ACTIVE' },
  { label: $t('rdi.overview.all'), value: '' },
  { label: $t('rdi.overview.high'), value: 'H' },
  { label: $t('rdi.overview.medium'), value: 'M' },
  { label: $t('rdi.overview.low'), value: 'L' },
  { label: $t('rdi.overview.normal'), value: 'N' }
])

const alarmTrendChartOptions = computed(() =>
  buildAlarmTrendChartOptions(alarmTrendPoints.value, $t('rdi.overview.alarmTrendSeries'))
)

async function refreshAll() {
  // The trend card is an independent panel. A backend failure there (for
  // example, a missing runtime IANA timezone database) must not prevent the
  // device snapshot request that powers the primary overview content.
  await Promise.allSettled([fetchDevices(), refreshAlarmSummaryCounts(), fetchAlarms(), fetchAlarmTrend()])
  scheduleDeviceSnapshotsRefresh()
}

onMounted(() => {
  refreshAll()
  void loadSnapshotGroupOptions()
})

watch(temperatureUnit, (value) => {
  try {
    if (typeof window !== 'undefined') window.localStorage.setItem('rdi-temperature-unit', value)
  } catch {
    // The current page can still use the selected unit even if localStorage is unavailable.
  }
})

onBeforeUnmount(() => {
  disposeDeviceSnapshots()
})
</script>

<template>
  <div class="rdi-overview">
    <NSpace vertical size="large">
      <NCard :bordered="false">
        <div class="overview-header">
          <div>
            <div class="overview-title">{{ $t('rdi.overview.title') }}</div>
            <div class="overview-subtitle">{{ $t('rdi.overview.subtitle') }}</div>
          </div>
          <NSpace>
            <NSelect
              v-model:value="temperatureUnit"
              :options="temperatureUnitOptions"
              class="temperature-unit-select"
            />
            <NButton @click="goBack">{{ $t('rdi.overview.back') }}</NButton>
            <NButton type="primary" :loading="loading || alarmLoading || snapshotLoading" @click="refreshAll">
              {{ $t('rdi.overview.refresh') }}
            </NButton>
          </NSpace>
        </div>
      </NCard>

      <div class="metric-grid">
        <NCard :bordered="false">
          <NStatistic :label="$t('rdi.overview.devices')" :value="stats.totalDevices" />
        </NCard>
        <NCard :bordered="false">
          <NStatistic :label="$t('rdi.overview.online')" :value="stats.onlineDevices" />
        </NCard>
        <NCard :bordered="false">
          <NStatistic :label="$t('rdi.overview.offline')" :value="stats.offlineDevices" />
        </NCard>
        <NCard :bordered="false">
          <NStatistic :label="$t('rdi.overview.alarmDevices')" :value="alarmDeviceTotal" />
        </NCard>
        <NCard :bordered="false">
          <NStatistic :label="$t('rdi.overview.activeAlarms')" :value="stats.activeAlarms" />
        </NCard>
        <NCard :bordered="false">
          <NStatistic :label="$t('rdi.overview.alarmHistoryTotal')" :value="stats.alarmHistoryTotal" />
        </NCard>
      </div>

      <NAlert :type="operationsFocusAlertType" :show-icon="false">
        <div class="operations-focus">
          <div>
            <strong>{{ $t(operationsFocus.titleKey) }}</strong>
            <p>{{ $t(operationsFocus.descKey) }}</p>
          </div>
          <NSpace>
            <NTag
              v-for="tag in operationsFocusTags"
              :key="tag.labelKey"
              :type="tag.type"
            >
              {{ $t(tag.labelKey) }}: {{ tag.value }}
            </NTag>
          </NSpace>
        </div>
      </NAlert>

      <NCard :title="$t(systemsCardTitleKey)" :bordered="false">
        <NSpin :show="snapshotLoading">
          <div class="snapshot-filter-panel">
            <NSpace align="center" :wrap="true" class="snapshot-filter-bar">
              <NInput
                v-model:value="snapshotFilterKeyword"
                :placeholder="$t('rdi.overview.snapshotFilterPlaceholder')"
                clearable
                class="snapshot-filter-keyword"
              />
              <NSelect
                v-model:value="snapshotFilterStatus"
                :options="snapshotStatusOptions"
                :placeholder="$t('rdi.overview.snapshotFilterStatus')"
                class="snapshot-filter-status"
              />
              <NButton size="small" tertiary @click="toggleSnapshotAdvancedFilters">
                {{
                  snapshotFilterAdvancedVisible
                    ? $t('rdi.overview.snapshotFilterHideAdvanced')
                    : $t('rdi.overview.snapshotFilterShowAdvanced')
                }}
              </NButton>
              <NButton
                v-if="snapshotHasActiveFilters"
                size="small"
                type="warning"
                secondary
                @click="resetSnapshotFilters"
              >
                {{ $t('rdi.overview.snapshotFilterClear') }}
              </NButton>
            </NSpace>
            <div v-if="snapshotFilterAdvancedVisible" class="snapshot-filter-advanced">
              <div class="snapshot-filter-advanced-item">
                <span class="snapshot-filter-advanced-label">{{ $t('common.alarm_level') }}</span>
                <NSelect
                  v-model:value="snapshotFilterAlarmLevel"
                  :options="snapshotAlarmLevelOptions"
                  class="snapshot-filter-advanced-control"
                />
              </div>
              <div class="snapshot-filter-advanced-item">
                <span class="snapshot-filter-advanced-label">{{ $t('rdi.overview.snapshotFilterGroup') }}</span>
                <NTreeSelect
                  v-model:value="snapshotFilterGroupId"
                  :options="snapshotGroupOptions"
                  :placeholder="$t('rdi.overview.snapshotFilterGroupPlaceholder')"
                  clearable
                  filterable
                  class="snapshot-filter-advanced-control"
                />
              </div>
            </div>
            <div v-if="snapshotActiveFilterChips.length" class="snapshot-filter-chips">
              <NTag
                v-for="chip in snapshotActiveFilterChips"
                :key="chip.key"
                closable
                type="info"
                size="small"
                @close="removeSnapshotFilter(chip.key)"
              >
                {{ chip.label }}
              </NTag>
            </div>
          </div>
          <template v-if="visibleDeviceSnapshots.length">
            <div class="snapshot-grid">
              <button
                v-for="device in visibleDeviceSnapshots"
                :key="device.id"
                type="button"
                class="snapshot-card"
                @click="goDevice(device.id)"
              >
                <div class="snapshot-head">
                  <div class="snapshot-title">
                    <strong>{{ device.name }}</strong>
                    <span>{{ $t('rdi.overview.pid') }}: {{ device.pid }}</span>
                  </div>
                  <NTag :type="snapshotStatusTagType(device)">
                    {{ snapshotStatusLabel(device) }}
                  </NTag>
                </div>
                <div class="snapshot-values">
                  <span>
                    T1
                    <strong>{{ formatTemperature(device.telemetry.temperature_1) }}</strong>
                  </span>
                  <span>
                    T2
                    <strong>{{ formatTemperature(device.telemetry.temperature_2) }}</strong>
                  </span>
                  <span>
                    {{ $t('rdi.overview.energy') }}
                    <strong>{{ device.telemetry.electricity_consumption ?? '--' }}</strong>
                  </span>
                </div>
                <div class="snapshot-status">
                  <span>SW1 {{ formatSwitch(device.telemetry.switch_1) }}</span>
                  <span>SW2 {{ formatSwitch(device.telemetry.switch_2) }}</span>
                  <span>DO {{ formatSwitch(device.telemetry.dry_contact_output) }}</span>
                </div>
                <div class="snapshot-foot">{{ $t('rdi.overview.firmware') }}: {{ device.firmware }}</div>
                <div v-if="isMasterAccount && device.tenantId !== '--'" class="snapshot-foot">
                  {{ $t('rdi.overview.tenantScope') }}: {{ device.tenantId }}
                </div>
                <div v-if="hasInstallationInfo(device)" class="snapshot-installation">
                  <span v-if="device.serialNumber !== '--'">{{ $t('rdi.overview.serialNumber') }} {{ device.serialNumber }}</span>
                  <span v-if="device.installDate !== '--'">{{ $t('rdi.overview.installedAt') }} {{ device.installDate }}</span>
                  <span v-if="device.installLocation !== '--'">{{ $t('rdi.overview.installLocation') }} {{ device.installLocation }}</span>
                  <span v-if="device.installAddress !== '--'">{{ $t('rdi.overview.installAddress') }} {{ device.installAddress }}</span>
                  <span v-if="device.installerName !== '--'">{{ $t('rdi.overview.installer') }} {{ device.installerName }}</span>
                  <span v-if="device.installerContact !== '--'">{{ $t('rdi.overview.installerContact') }} {{ device.installerContact }}</span>
                  <span v-if="device.adminName !== '--'">{{ $t('rdi.overview.administrator') }} {{ device.adminName }}</span>
                </div>
              </button>
            </div>
            <NPagination
              v-if="snapshotTotal > RDI_SNAPSHOT_LIMIT"
              :page="snapshotPage"
              :page-size="RDI_SNAPSHOT_LIMIT"
              :item-count="snapshotTotal"
              class="snapshot-pagination"
              @update:page="changeSnapshotPage"
            />
          </template>
          <div v-else class="snapshot-empty">
            <strong>{{ $t(systemsEmptyTitleKey) }}</strong>
            <span>{{ $t(systemsEmptyDescriptionKey) }}</span>
            <NSpace :size="[8, 8]">
              <NButton size="small" type="primary" @click="openDeviceManage">
                {{ $t('rdi.overview.openDeviceManage') }}
              </NButton>
              <NButton size="small" secondary @click="openServiceAccess">
                {{ $t('rdi.overview.openServiceAccess') }}
              </NButton>
              <NButton size="small" secondary :loading="snapshotLoading" @click="() => fetchDeviceSnapshots()">
                {{ $t('common.refresh') }}
              </NButton>
            </NSpace>
          </div>
        </NSpin>
      </NCard>

      <NCard :title="$t('rdi.overview.alarmOverview')" :bordered="false">
        <NSpace vertical size="medium">
          <NSpace align="center" :wrap="true">
            <NSelect v-model:value="queryParams.alarm_status" class="status-filter" :options="alarmStatusOptions" />
            <NButton type="primary" @click="searchAlarms">{{ $t('common.search') }}</NButton>
            <NButton @click="resetAlarmFilter">{{ $t('common.reset') }}</NButton>
          </NSpace>
          <NDataTable
            :columns="alarmColumns"
            :data="alarms"
            :loading="alarmLoading"
            :pagination="alarmPagination"
            :scroll-x="980"
          >
            <template #empty>
              <NEmpty :description="$t('common.noData')" class="py-24px" />
            </template>
          </NDataTable>
        </NSpace>
      </NCard>

      <NCard :title="$t('rdi.overview.alarmTrendTitle')" :bordered="false">
        <NSpin :show="alarmTrendLoading">
          <div class="alarm-trend-card">
            <div class="alarm-trend-summary">
              <div class="alarm-trend-year-control">
                <strong>{{ $t('rdi.overview.alarmTrendYear') }}</strong>
                <NSelect
                  v-model:value="alarmTrendYear"
                  :options="alarmTrendYearOptions"
                  class="alarm-trend-year-select"
                  @update:value="fetchAlarmTrend"
                />
              </div>
              <span>{{ $t('rdi.overview.alarmTrendDesc') }}</span>
            </div>
            <div class="alarm-trend-chart">
              <ChartComponent :initial-options="alarmTrendChartOptions" />
            </div>
          </div>
        </NSpin>
      </NCard>
    </NSpace>
  </div>
</template>

<style scoped>
.rdi-overview {
  padding: 16px;
}

.overview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.overview-title {
  font-size: 22px;
  font-weight: 700;
}

.overview-subtitle {
  margin-top: 4px;
  color: var(--text-color-3);
}

.temperature-unit-select {
  width: 150px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 16px;
}

.operations-focus {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.operations-focus strong {
  color: #111827;
  font-size: 15px;
}

.operations-focus p {
  margin: 4px 0 0;
  color: #475569;
}

.snapshot-filter-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 16px;
}

.snapshot-filter-keyword {
  width: 260px;
  max-width: 100%;
}

.snapshot-filter-status {
  width: 160px;
}

.snapshot-filter-advanced {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding: 12px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f9fafb;
}

.snapshot-filter-advanced-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.snapshot-filter-advanced-label {
  color: #6b7280;
  font-size: 13px;
  white-space: nowrap;
}

.snapshot-filter-advanced-control {
  width: 200px;
}

.snapshot-filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.snapshot-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
}

.snapshot-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.snapshot-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 168px;
  padding: 14px;
  text-align: left;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  cursor: pointer;
}

.snapshot-card:hover {
  border-color: #2563eb;
  box-shadow: 0 8px 22px rgb(15 23 42 / 8%);
}

.snapshot-empty {
  display: flex;
  min-height: 160px;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 10px;
  border: 1px dashed #bfdbfe;
  border-radius: 8px;
  background: #eff6ff;
  padding: 20px;
}

.snapshot-empty strong {
  color: #1e3a8a;
  font-size: 15px;
}

.snapshot-empty span {
  max-width: 640px;
  color: #1d4ed8;
  font-size: 13px;
  line-height: 1.6;
}

.snapshot-head,
.snapshot-values,
.snapshot-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.snapshot-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.snapshot-title strong {
  overflow: hidden;
  color: #111827;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.snapshot-title span,
.snapshot-foot {
  color: #6b7280;
  font-size: 12px;
}

.snapshot-installation {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 8px;
  color: #4b5563;
  font-size: 12px;
}

.snapshot-installation span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.snapshot-values,
.snapshot-status {
  flex-wrap: wrap;
  color: #374151;
  font-size: 13px;
}

.snapshot-values span,
.snapshot-status span {
  min-width: 72px;
}

.snapshot-values strong {
  display: block;
  margin-top: 2px;
  color: #111827;
  font-size: 18px;
}

.status-filter {
  width: 180px;
}

.action-row {
  display: flex;
  gap: 8px;
}

.alarm-trend-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.alarm-trend-summary {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.alarm-trend-year-control {
  display: flex;
  align-items: center;
  gap: 10px;
}

.alarm-trend-year-select {
  width: 120px;
}

.alarm-trend-summary strong {
  color: #111827;
  font-size: 15px;
}

.alarm-trend-summary span {
  color: #6b7280;
  font-size: 13px;
}

.alarm-trend-chart {
  height: 280px;
}

@media (max-width: 900px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .metric-grid {
    grid-template-columns: 1fr;
  }

  .overview-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .temperature-unit-select {
    width: 100%;
  }

  .operations-focus {
    align-items: flex-start;
    flex-direction: column;
  }

  .alarm-trend-summary {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
