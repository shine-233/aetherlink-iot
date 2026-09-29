<!--
  文件用途：RDI 运营概览（设备统计、运营焦点、系统快照、告警列表与年度趋势）。
  核心逻辑：数据加载在 useRdiOverviewData / useRdiDeviceSnapshots，快照筛选在 useRdiSnapshotFilters，
    告警列在 rdiAlarmColumns，快照卡与筛选面板为子组件；本文件只做编排。
  关键注意事项：页面测试通过 setupState 读取若干纯函数别名（parseAlarmRemark、formatSwitch 等），
    这些别名直接指向 rdiOverviewState / rdiSnapshotPresentation 的实现，不要再在此处重写逻辑。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { acknowledgeAlarmHistory, resetAlarmHistory } from '@/service/api/alarm'
import { useAuthStore } from '@/store/modules/auth'
import { isSysAdminUser } from '@/utils/thingsvis/space'
import { $t } from '@/locales'
import {
  RDI_SNAPSHOT_LIMIT,
  alarmStatusLabel as resolveAlarmStatusLabel,
  alarmTypeLabel as resolveAlarmTypeLabel,
  buildOperationsFocus,
  formatSwitch as resolveSwitchLabel,
  formatTemperature as resolveTemperatureLabel,
  type AlarmRecord,
  type TemperatureUnit
} from './rdiOverviewState'
import { buildAlarmTrendChartOptions, buildAlarmTrendYearOptions } from './rdiTrendChart'
import { createRdiAlarmColumns } from './rdiAlarmColumns'
import { useRdiOverviewData } from './useRdiOverviewData'
import { useRdiSnapshotFilters } from './useRdiSnapshotFilters'
import { useRdiDeviceSnapshots } from './useRdiDeviceSnapshots'
import RdiAlarmTrendCard from './RdiAlarmTrendCard.vue'
import RdiSnapshotCard from './RdiSnapshotCard.vue'
import RdiSnapshotFilterPanel from './RdiSnapshotFilterPanel.vue'

const props = withDefaults(defineProps<{ activeSystemsOnly?: boolean }>(), { activeSystemsOnly: false })

const TEMPERATURE_UNIT_STORAGE_KEY = 'rdi-temperature-unit'
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
  dispose: disposeDeviceSnapshots
} = useRdiDeviceSnapshots({ activeSystemsOnly: () => props.activeSystemsOnly, isMasterAccount })

const snapshotFilters = useRdiSnapshotFilters({ deviceSnapshots })
const { visibleDeviceSnapshots } = snapshotFilters

function readTemperatureUnitPreference(): TemperatureUnit {
  try {
    return typeof window !== 'undefined' && window.localStorage.getItem(TEMPERATURE_UNIT_STORAGE_KEY) === 'F'
      ? 'F'
      : 'C'
  } catch {
    return 'C'
  }
}

type OperationsAlertType = 'default' | 'info' | 'success' | 'warning' | 'error'
type OperationsTagType = OperationsAlertType | 'primary'
const operationsFocus = computed(() => buildOperationsFocus(stats, alarmDeviceTotal.value))
const operationsFocusAlertType = computed(() => operationsFocus.value.type as OperationsAlertType)
const operationsFocusTags = computed(() =>
  operationsFocus.value.tags.map((tag) => ({ ...tag, type: tag.type as OperationsTagType }))
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
const metricCards = computed(() => [
  { key: 'devices', label: 'rdi.overview.devices', value: stats.totalDevices },
  { key: 'online', label: 'rdi.overview.online', value: stats.onlineDevices },
  { key: 'offline', label: 'rdi.overview.offline', value: stats.offlineDevices },
  { key: 'alarmDevices', label: 'rdi.overview.alarmDevices', value: alarmDeviceTotal.value },
  { key: 'activeAlarms', label: 'rdi.overview.activeAlarms', value: stats.activeAlarms },
  { key: 'history', label: 'rdi.overview.alarmHistoryTotal', value: stats.alarmHistoryTotal }
])

const temperatureUnitOptions = [
  { label: 'Celsius (C)', value: 'C' },
  { label: 'Fahrenheit (F)', value: 'F' }
]
const alarmTrendYearOptions = computed(() => buildAlarmTrendYearOptions(dayjs().year()))
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

// 绑定当前单位 / 语言的展示别名（模板与页面测试共用）。
const alarmStatusLabel = (status?: string) => resolveAlarmStatusLabel(status, $t)
const alarmTypeLabel = (row: AlarmRecord) => resolveAlarmTypeLabel(row, $t)
const formatTemperature = (value: unknown) => resolveTemperatureLabel(value, temperatureUnit.value)
const formatSwitch = (value: unknown) => resolveSwitchLabel(value, $t)

function goDevice(deviceId?: string) {
  if (!deviceId) return
  router.push({ name: 'device_details', query: { d_id: deviceId, tab: 'message' } })
}
const openDeviceManage = () => router.push('/device/manage')
const openServiceAccess = () => router.push('/device/service-access')
const goBack = () => router.back()

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
      if (props.activeSystemsOnly) snapshotPage.value = 1
      scheduleDeviceSnapshotsRefresh()
    }
  })
}

const alarmColumns = createRdiAlarmColumns({
  isMasterAccount,
  onOpenDevice: goDevice,
  onAcknowledge: acknowledgeAlarm,
  onReset: resetAlarm
})

async function refreshAll() {
  // 趋势卡是独立面板：它的后端失败（例如缺少 IANA 时区库）不能阻断主快照请求。
  await Promise.allSettled([fetchDevices(), refreshAlarmSummaryCounts(), fetchAlarms(), fetchAlarmTrend()])
  scheduleDeviceSnapshotsRefresh()
}

onMounted(() => {
  refreshAll()
  void snapshotFilters.loadSnapshotGroupOptions()
})

watch(temperatureUnit, (value) => {
  try {
    if (typeof window !== 'undefined') window.localStorage.setItem(TEMPERATURE_UNIT_STORAGE_KEY, value)
  } catch {
    // localStorage 不可用时当前页面仍按所选单位展示。
  }
})

onBeforeUnmount(disposeDeviceSnapshots)
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
        <NCard v-for="metric in metricCards" :key="metric.key" :bordered="false">
          <NStatistic :label="$t(metric.label)" :value="metric.value" />
        </NCard>
      </div>

      <NAlert :type="operationsFocusAlertType" :show-icon="false">
        <div class="operations-focus">
          <div>
            <strong>{{ $t(operationsFocus.titleKey) }}</strong>
            <p>{{ $t(operationsFocus.descKey) }}</p>
          </div>
          <NSpace>
            <NTag v-for="tag in operationsFocusTags" :key="tag.labelKey" :type="tag.type">
              {{ $t(tag.labelKey) }}: {{ tag.value }}
            </NTag>
          </NSpace>
        </div>
      </NAlert>

      <NCard :title="$t(systemsCardTitleKey)" :bordered="false">
        <NSpin :show="snapshotLoading">
          <RdiSnapshotFilterPanel :filters="snapshotFilters" />
          <template v-if="visibleDeviceSnapshots.length">
            <div class="snapshot-grid">
              <RdiSnapshotCard
                v-for="device in visibleDeviceSnapshots"
                :key="device.id"
                :device="device"
                :temperature-unit="temperatureUnit"
                :show-tenant="isMasterAccount"
                @open="goDevice"
              />
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

      <RdiAlarmTrendCard
        v-model:year="alarmTrendYear"
        :loading="alarmTrendLoading"
        :year-options="alarmTrendYearOptions"
        :chart-options="alarmTrendChartOptions"
        @change="fetchAlarmTrend"
      />
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

.status-filter {
  width: 180px;
}

.action-row {
  display: flex;
  gap: 8px;
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
}
</style>
