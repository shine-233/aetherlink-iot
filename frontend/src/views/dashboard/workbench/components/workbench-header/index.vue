<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { listFleetCommandJobs } from '@/service/api/device'
import { getAlarmCount, sumData } from '@/service/api/system-data'
import { useAuthStore } from '@/store/modules/auth'
import { $t } from '@/locales'

defineOptions({ name: 'DashboardWorkbenchHeader' })

const auth = useAuthStore()

interface StatisticData {
  id: string
  label: string
  value: string
  detail: string
}

type DeviceSummaryPayload = {
  device_total?: unknown
  device_on?: unknown
  device_offline?: unknown
}

type AlarmSummaryPayload = {
  active_alarm_total?: unknown
  alarm_device_total?: unknown
}

type CommandJobSummaryPayload = {
  total?: unknown
  attention_counts?: {
    needs_operator_action_count?: unknown
  }
}

type MetricSnapshot = {
  deviceTotal: number | null
  deviceOnline: number | null
  deviceOffline: number | null
  activeAlarmTotal: number | null
  alarmDeviceTotal: number | null
  commandJobTotal: number | null
  commandJobsNeedingAttention: number | null
}

const unavailableText = $t('custom.dashboardWorkbench.metricUnavailable')

const metrics = ref<MetricSnapshot>({
  deviceTotal: null,
  deviceOnline: null,
  deviceOffline: null,
  activeAlarmTotal: null,
  alarmDeviceTotal: null,
  commandJobTotal: null,
  commandJobsNeedingAttention: null
})

const unwrapApiData = <T,>(payload: unknown): T | null => {
  if (payload && typeof payload === 'object' && 'data' in (payload as Record<string, unknown>)) {
    return ((payload as Record<string, unknown>).data as T | null) ?? null
  }

  return (payload as T | null) ?? null
}

const toNumber = (value: unknown): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

const formatMetricValue = (value: number | null) => {
  if (value === null) return '--'
  return new Intl.NumberFormat().format(value)
}

const statisticData = computed<StatisticData[]>(() => [
  {
    id: 'devices',
    label: $t('custom.dashboardWorkbench.deviceSummary'),
    value: formatMetricValue(metrics.value.deviceTotal),
    detail:
      metrics.value.deviceOnline === null || metrics.value.deviceOffline === null
        ? unavailableText
        : $t('custom.dashboardWorkbench.deviceSummaryDetail', {
            online: formatMetricValue(metrics.value.deviceOnline),
            offline: formatMetricValue(metrics.value.deviceOffline)
          })
  },
  {
    id: 'alarms',
    label: $t('custom.dashboardWorkbench.activeAlarmSummary'),
    value: formatMetricValue(metrics.value.activeAlarmTotal),
    detail:
      metrics.value.alarmDeviceTotal === null
        ? unavailableText
        : $t('custom.dashboardWorkbench.activeAlarmSummaryDetail', {
            devices: formatMetricValue(metrics.value.alarmDeviceTotal)
          })
  },
  {
    id: 'command-jobs',
    label: $t('custom.dashboardWorkbench.commandJobSummary'),
    value: formatMetricValue(metrics.value.commandJobTotal),
    detail:
      metrics.value.commandJobsNeedingAttention === null
        ? unavailableText
        : $t('custom.dashboardWorkbench.commandJobSummaryDetail', {
            attention: formatMetricValue(metrics.value.commandJobsNeedingAttention)
          })
  }
])

const loadSummaryMetrics = async () => {
  const [deviceResult, alarmResult, commandJobResult] = await Promise.allSettled([
    sumData(),
    getAlarmCount(),
    listFleetCommandJobs({ page: 1, page_size: 1 })
  ])

  if (deviceResult.status === 'fulfilled') {
    const deviceData = unwrapApiData<DeviceSummaryPayload>(deviceResult.value)
    if (deviceData) {
      metrics.value.deviceTotal = toNumber(deviceData.device_total)
      metrics.value.deviceOnline = toNumber(deviceData.device_on)
      metrics.value.deviceOffline = toNumber(deviceData.device_offline)
    }
  }

  if (alarmResult.status === 'fulfilled') {
    const alarmData = unwrapApiData<AlarmSummaryPayload>(alarmResult.value)
    if (alarmData) {
      metrics.value.activeAlarmTotal = toNumber(alarmData.active_alarm_total)
      metrics.value.alarmDeviceTotal = toNumber(alarmData.alarm_device_total)
    }
  }

  if (commandJobResult.status === 'fulfilled') {
    const commandJobData = unwrapApiData<CommandJobSummaryPayload>(commandJobResult.value)
    if (commandJobData) {
      metrics.value.commandJobTotal = toNumber(commandJobData.total)
      metrics.value.commandJobsNeedingAttention = toNumber(commandJobData.attention_counts?.needs_operator_action_count)
    }
  }
}

onMounted(async () => {
  await loadSummaryMetrics()
})
</script>

<template>
  <NCard :bordered="false" class="rounded-8px shadow-sm">
    <div class="flex-y-center justify-between gap-16px">
      <div class="flex-y-center min-w-0">
        <IconLocalAvatar class="text-70px" />
        <div class="min-w-0 pl-12px">
          <h3 class="text-18px font-semibold">
            {{ $t('custom.dashboardWorkbench.title', { userName: auth.userInfo.userName }) }}
          </h3>
          <p class="text-#666 leading-30px dark:text-#aaa">{{ $t('custom.dashboardWorkbench.description') }}</p>
        </div>
      </div>
      <div class="workbench-header-summary-grid">
        <div v-for="item in statisticData" :key="item.id" class="workbench-header-summary-item">
          <div class="workbench-header-summary-label">{{ item.label }}</div>
          <div class="workbench-header-summary-value">{{ item.value }}</div>
          <div class="workbench-header-summary-detail">{{ item.detail }}</div>
        </div>
      </div>
    </div>
  </NCard>
</template>

<style scoped>
.workbench-header-summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.workbench-header-summary-item {
  min-width: 120px;
  border: 1px solid rgb(229 231 235);
  border-radius: 8px;
  padding: 10px 12px;
  background: rgb(249 250 251 / 85%);
}

.workbench-header-summary-label {
  font-size: 12px;
  line-height: 18px;
  color: rgb(107 114 128);
}

.workbench-header-summary-value {
  margin-top: 4px;
  font-size: 20px;
  line-height: 28px;
  font-weight: 600;
  color: rgb(17 24 39);
}

.workbench-header-summary-detail {
  margin-top: 4px;
  font-size: 12px;
  line-height: 18px;
  color: rgb(75 85 99);
}

.dark .workbench-header-summary-item {
  border-color: rgb(255 255 255 / 0.12);
  background: rgb(17 24 39 / 0.4);
}

.dark .workbench-header-summary-label {
  color: rgb(156 163 175);
}

.dark .workbench-header-summary-value {
  color: rgb(243 244 246);
}

.dark .workbench-header-summary-detail {
  color: rgb(209 213 219);
}
</style>
