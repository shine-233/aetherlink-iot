/**
 * File purpose: overview data-plane composable for the RDI Overview page.
 * Owns the summary stats, alarm history list/pagination, active-alarm counts,
 * and monthly trend state plus their request sequencing. Rendering, dialog and
 * navigation glue stay in the owning view. Request order and payload shapes
 * must stay identical to the previous inline implementation.
 */
import { reactive, ref, toValue, type MaybeRefOrGetter } from 'vue'
import type { PaginationProps } from 'naive-ui'
import dayjs from 'dayjs'
import { alarmHistory, alarmHistoryMonthlyTrend } from '@/service/api/alarm'
import { getAlarmCount, sumData } from '@/service/api/system-data'
import { normalizeAlarmMonthlyTrendPoints, type AlarmRecord, type AlarmTrendPoint } from './rdiOverviewState'

export function useRdiOverviewData(options: {
  isMasterAccount: MaybeRefOrGetter<boolean>
}) {
  const loading = ref(false)
  const alarmLoading = ref(false)
  const alarmTrendLoading = ref(false)
  const alarms = ref<AlarmRecord[]>([])
  const alarmTrendPoints = ref<AlarmTrendPoint[]>(normalizeAlarmMonthlyTrendPoints([]))
  const alarmTrendYear = ref(dayjs().year())
  const alarmTrendTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  const alarmDeviceTotal = ref(0)

  const stats = reactive({
    totalDevices: 0,
    onlineDevices: 0,
    offlineDevices: 0,
    activeAlarms: 0,
    alarmHistoryTotal: 0
  })

  const queryParams = reactive({
    page: 1,
    page_size: 10,
    alarm_status: 'ACTIVE'
  })

  async function fetchDevices() {
    loading.value = true
    try {
      const res = toValue(options.isMasterAccount) ? await sumData({ all_tenants: true }) : await sumData()
      const data = res?.data || {}
      stats.totalDevices = Number(data.device_total ?? data.DeviceTotal ?? 0)
      stats.onlineDevices = Number(data.device_on ?? data.DeviceOn ?? 0)
      stats.offlineDevices = Number(
        data.device_offline ?? data.DeviceOffline ?? Math.max(stats.totalDevices - stats.onlineDevices, 0)
      )
    } finally {
      loading.value = false
    }
  }

  async function fetchCounts() {
    const res = toValue(options.isMasterAccount)
      ? await getAlarmCount({ all_tenants: true })
      : await getAlarmCount()
    const data = (res?.data || {}) as any
    alarmDeviceTotal.value = Number(data.alarm_device_total ?? data.AlarmDeviceTotal ?? 0)
    stats.alarmHistoryTotal = Number(data.alarm_history_total ?? data.AlarmHistoryTotal ?? 0)
    const activeAlarmTotal = data.active_alarm_total ?? data.ActiveAlarmTotal
    if (activeAlarmTotal !== undefined && activeAlarmTotal !== null) {
      stats.activeAlarms = Number(activeAlarmTotal || 0)
      return true
    }
    return false
  }

  async function fetchActiveAlarmCounts() {
    const res = await alarmHistory({
      page: 1,
      page_size: 1,
      alarm_status: 'ACTIVE',
      ...(toValue(options.isMasterAccount) ? { all_tenants: true } : {})
    })
    stats.activeAlarms = Number(res?.data?.total || 0)
  }

  async function refreshAlarmSummaryCounts() {
    const hasActiveAlarmTotal = await fetchCounts()
    if (!hasActiveAlarmTotal) {
      await fetchActiveAlarmCounts()
    }
  }

  async function fetchAlarms() {
    alarmLoading.value = true
    try {
      const res = await alarmHistory({
        page: queryParams.page,
        page_size: queryParams.page_size,
        alarm_status: queryParams.alarm_status || undefined,
        ...(toValue(options.isMasterAccount) ? { all_tenants: true } : {})
      })
      alarms.value = res?.data?.list || []
      alarmPagination.itemCount = res?.data?.total || 0
    } finally {
      alarmLoading.value = false
    }
  }

  function searchAlarms() {
    queryParams.page = 1
    alarmPagination.page = 1
    fetchAlarms()
  }

  async function fetchAlarmTrend() {
    alarmTrendLoading.value = true
    try {
      const res = toValue(options.isMasterAccount)
        ? await alarmHistoryMonthlyTrend(alarmTrendYear.value, alarmTrendTimezone, { all_tenants: true })
        : await alarmHistoryMonthlyTrend(alarmTrendYear.value, alarmTrendTimezone)
      alarmTrendPoints.value = normalizeAlarmMonthlyTrendPoints(res?.data?.months || [])
    } finally {
      alarmTrendLoading.value = false
    }
  }

  const alarmPagination: PaginationProps = reactive({
    page: queryParams.page,
    pageSize: queryParams.page_size,
    itemCount: 0,
    showSizePicker: true,
    pageSizes: [10, 20, 50],
    onChange: (page) => {
      queryParams.page = page
      alarmPagination.page = page
      fetchAlarms()
    },
    onUpdatePageSize: (pageSize) => {
      queryParams.page = 1
      queryParams.page_size = pageSize
      alarmPagination.page = 1
      alarmPagination.pageSize = pageSize
      fetchAlarms()
    }
  })

  function resetAlarmFilter() {
    queryParams.alarm_status = 'ACTIVE'
    queryParams.page = 1
    alarmPagination.page = 1
    fetchAlarms()
  }

  return {
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
  }
}
