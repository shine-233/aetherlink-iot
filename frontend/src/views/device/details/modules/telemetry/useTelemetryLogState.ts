/**
 * 文件用途: 维护遥测操作日志的筛选、分页、加载态和表格数据。
 * 核心逻辑: 归一化日志查询参数，并忽略过期请求返回。
 * 关键注意事项: 后端日志接口形状变化时，先更新这里再调整页面展示。
 */
import { ref } from 'vue'
import { buildTelemetryLogQuery, telemetryLogPageCount } from './telemetryLogState'
import { useLoading } from '~/packages/hooks'

type TelemetryLogRow = Record<string, unknown>

type TelemetryLogListResponse = {
  list?: TelemetryLogRow[]
  value?: TelemetryLogRow[]
  count?: number
}

type TelemetryLogListRequestResult = {
  data: TelemetryLogListResponse | null
  error?: unknown
}

type TelemetryLogRequestSnapshot = {
  deviceId: string
  page: number
  operationType: string
  status: string
}

type PendingTelemetryLogRequest = {
  key: string
  requestId: number
  promise: Promise<void>
}

type UseTelemetryLogStateOptions = {
  getDeviceId: () => string
  loadTelemetryLogList: (params: Record<string, unknown>) => Promise<TelemetryLogListRequestResult>
  translate: (key: string) => string
}

export const useTelemetryLogState = ({ getDeviceId, loadTelemetryLogList, translate }: UseTelemetryLogStateOptions) => {
  const operationType = ref('')
  const sendResult = ref('')
  const tableData = ref<TelemetryLogRow[]>([])
  const total = ref(0)
  const logPage = ref(1)
  const { loading, startLoading, endLoading } = useLoading()
  let latestLogRequestId = 0
  let pendingLogRequest: PendingTelemetryLogRequest | null = null

  const operationOptions = [
    { label: translate('custom.device_details.whole'), value: '' },
    { label: translate('custom.device_details.manualOperation'), value: '1' },
    { label: translate('custom.device_details.triggerOperation'), value: '2' }
  ]

  const resultOptions = [
    { label: translate('custom.device_details.whole'), value: '' },
    { label: translate('custom.devicePage.success'), value: '1' },
    { label: translate('custom.devicePage.fail'), value: '2' }
  ]

  const updateLogTable = (data: TelemetryLogListResponse) => {
    tableData.value = data.value || data.list || []
    total.value = telemetryLogPageCount(data.count || 0)
  }

  const createLogRequestSnapshot = (): TelemetryLogRequestSnapshot => ({
    deviceId: getDeviceId(),
    page: logPage.value,
    operationType: operationType.value,
    status: sendResult.value
  })

  const logRequestStillMatches = (snapshot: TelemetryLogRequestSnapshot, requestId: number) =>
    latestLogRequestId === requestId &&
    getDeviceId() === snapshot.deviceId &&
    logPage.value === snapshot.page &&
    operationType.value === snapshot.operationType &&
    sendResult.value === snapshot.status

  const fetchData = async () => {
    const snapshot = createLogRequestSnapshot()
    const requestKey = JSON.stringify(snapshot)
    if (pendingLogRequest?.key === requestKey) return pendingLogRequest.promise

    const requestId = latestLogRequestId + 1
    latestLogRequestId = requestId
    startLoading()
    const requestPromise = (async () => {
      try {
        const { data, error } = await loadTelemetryLogList(buildTelemetryLogQuery(snapshot))
        if (!error && data && logRequestStillMatches(snapshot, requestId)) {
          updateLogTable(data)
        }
      } finally {
        if (pendingLogRequest?.requestId === requestId) {
          pendingLogRequest = null
        }

        if (latestLogRequestId === requestId) {
          endLoading()
        }
      }
    })()

    pendingLogRequest = {
      key: requestKey,
      requestId,
      promise: requestPromise
    }

    return requestPromise
  }

  const handleLogPageChange = (page: number) => {
    logPage.value = page
    void fetchData()
  }

  const fetchFirstLogPage = () => {
    logPage.value = 1
    void fetchData()
  }

  const resetLogState = () => {
    latestLogRequestId += 1
    pendingLogRequest = null
    logPage.value = 1
    tableData.value = []
    total.value = 0
    endLoading()
  }

  return {
    fetchData,
    fetchFirstLogPage,
    handleLogPageChange,
    loading,
    operationOptions,
    operationType,
    resultOptions,
    sendResult,
    tableData,
    total,
    resetLogState
  }
}
