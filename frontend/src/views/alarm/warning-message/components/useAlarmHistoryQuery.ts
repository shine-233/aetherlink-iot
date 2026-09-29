/**
 * 文件用途：告警历史列表的查询状态、分页与实时刷新组合函数。
 * 核心逻辑：维护筛选条件/分页/表格数据，以请求序号丢弃过期响应；订阅租户级告警实时事件后去抖刷新。
 * 关键注意事项：卸载时必须同时停止 socket 与清除去抖定时器，否则卸载后仍会发起列表请求。
 */
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import type { Ref } from 'vue'
import type { PaginationProps } from 'naive-ui'
import dayjs from 'dayjs'
import { alarmHistory } from '@/service/api/alarm'
import { useAlarmStatusSocket } from '@/hooks/alarm/useAlarmStatusSocket'
import type { FleetRolloutContext } from '../../../device/modules/fleet-rollout-context'
import type { AlarmConfigurationRow } from './alarmConfigurationColumns'

export const ALARM_REALTIME_REFRESH_DEBOUNCE_MS = 800
const ALARM_QUERY_TIME_FORMAT = 'YYYY-MM-DDTHH:mm:ssZ'

export interface AlarmHistoryQueryProps {
  initialDeviceId?: string
  fleetContext?: FleetRolloutContext | null
}

export interface AlarmHistoryQueryData {
  alarm_status: string
  alarm_type: string
  device_id: string
  start_time: string
  end_time: string
  page: number
  page_size: number
}

export const defaultAlarmRange = (): [number, number] => [dayjs().subtract(1, 'month').valueOf(), dayjs().valueOf()]

/** 把日期范围格式化为后端查询所需的起止时间；空范围返回空串。 */
export const formatAlarmRangeQuery = (range: [number, number] | null | undefined) => {
  if (!range) return { start_time: '', end_time: '' }
  return {
    start_time: dayjs(range[0]).format(ALARM_QUERY_TIME_FORMAT),
    end_time: dayjs(range[1]).format(ALARM_QUERY_TIME_FORMAT)
  }
}

export function useAlarmHistoryQuery(props: AlarmHistoryQueryProps) {
  const loading = ref(false)
  let requestSeq = 0

  const range = ref<[number, number]>(defaultAlarmRange())
  const initialFocusedDeviceId = () => props.initialDeviceId || props.fleetContext?.deviceIds[0] || ''
  const focusedDeviceId = ref(initialFocusedDeviceId())
  const hasRouteDeviceContext = computed(() => Boolean(props.initialDeviceId || props.fleetContext?.deviceIds.length))
  const fleetDeviceCount = computed(() => props.fleetContext?.deviceIds.length || 0)

  const queryData = ref<AlarmHistoryQueryData>({
    alarm_status: '',
    alarm_type: '',
    device_id: focusedDeviceId.value,
    start_time: '',
    end_time: '',
    page: 1,
    page_size: 10
  })
  const tableData = ref<AlarmConfigurationRow[]>([]) as Ref<AlarmConfigurationRow[]>
  const selectedAlarmRowKeys = ref<Array<string | number>>([])

  const getAlarmHistory = async () => {
    const seq = requestSeq + 1
    requestSeq = seq
    loading.value = true
    Object.assign(queryData.value, formatAlarmRangeQuery(range.value), {
      device_id: focusedDeviceId.value,
      page: pagination.page as number,
      page_size: pagination.pageSize as number
    })
    try {
      const { data } = await alarmHistory({ ...queryData.value })
      if (seq !== requestSeq) return
      if (data) {
        pagination.itemCount = data.total
        tableData.value = data.list
      }
    } finally {
      if (seq === requestSeq) loading.value = false
    }
  }

  const goToPage = (page: number, pageSize = pagination.pageSize as number) => {
    pagination.page = page
    pagination.pageSize = pageSize
    selectedAlarmRowKeys.value = []
    getAlarmHistory()
  }

  const pagination: PaginationProps = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: true,
    pageSizes: [10, 15, 20, 25, 30],
    onChange: (page: number) => goToPage(page),
    onUpdatePageSize: (pageSize: number) => goToPage(1, pageSize)
  })

  const handleSearch = () => goToPage(1)

  const resetData = () => {
    range.value = defaultAlarmRange()
    queryData.value.alarm_status = ''
    queryData.value.alarm_type = ''
    focusedDeviceId.value = initialFocusedDeviceId()
    handleSearch()
  }

  // TB-30：订阅租户级告警实时事件，收到生命周期事件后去抖刷新列表。
  let realtimeRefreshTimer: ReturnType<typeof setTimeout> | null = null
  const clearRealtimeTimer = () => {
    if (realtimeRefreshTimer) {
      clearTimeout(realtimeRefreshTimer)
      realtimeRefreshTimer = null
    }
  }
  const { start: startRealtime, stop: stopRealtime } = useAlarmStatusSocket(() => {
    if (realtimeRefreshTimer) return
    realtimeRefreshTimer = setTimeout(() => {
      realtimeRefreshTimer = null
      getAlarmHistory()
    }, ALARM_REALTIME_REFRESH_DEBOUNCE_MS)
  })

  onMounted(() => {
    getAlarmHistory()
    startRealtime()
  })

  onUnmounted(() => {
    stopRealtime()
    clearRealtimeTimer()
    // 卸载后丢弃仍在途的列表响应。
    requestSeq += 1
  })

  watch(
    () => [props.initialDeviceId, props.fleetContext?.deviceIds.join(',')],
    () => {
      focusedDeviceId.value = initialFocusedDeviceId()
      handleSearch()
    }
  )

  return {
    loading,
    range,
    focusedDeviceId,
    hasRouteDeviceContext,
    fleetDeviceCount,
    queryData,
    tableData,
    selectedAlarmRowKeys,
    pagination,
    getAlarmHistory,
    handleSearch,
    resetData
  }
}
