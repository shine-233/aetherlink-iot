/**
 * 文件用途: 编排遥测页的控制项、操作日志和模拟上报入口。
 * 核心逻辑: 组合日志状态、控制项列表、设备协议判断与删除/下发动作用例。
 * 关键注意事项: 首屏初始化顺序在这里集中维护，避免 telemetry.vue 再次膨胀。
 */
import { onMounted, ref, watch } from 'vue'
import {
  buildControlPublishPayload,
  buildDeleteParams,
  shouldShowSimulationEntry,
  type TelemetryControlItem,
  type TelemetryControlPublishPayload,
  type TelemetryDeleteParams
} from './telemetryControlState'
import { deleteTelemetryItem, loadTelemetryControlList } from './telemetryDeviceOperations'
import { useTelemetryLogState } from './useTelemetryLogState'

type RequestResult<T> = {
  data: T | null
  error?: unknown
}

type PendingControlListRequest = {
  templateId: string
  requestId: number
  promise: Promise<void>
}

type UseTelemetryOperationsSectionOptions = {
  getDeviceId: () => string
  getDeviceTemplateId: () => string
  getDeviceConfig: () => { protocol_type?: string } | undefined
  loadTelemetryLogList: (
    params: Record<string, unknown>
  ) => Promise<RequestResult<{ list?: Record<string, unknown>[]; value?: Record<string, unknown>[]; count?: number }>>
  loadDeviceControlList: (params: Record<string, unknown>) => Promise<RequestResult<{ list?: TelemetryControlItem[] }>>
  deleteTelemetryData: (params: TelemetryDeleteParams) => Promise<RequestResult<unknown>>
  publishTelemetryData: (payload: TelemetryControlPublishPayload) => Promise<RequestResult<unknown>>
  refreshTelemetry: () => void | Promise<void>
  translate: (key: string) => string
}

export const useTelemetryOperationsSection = ({
  getDeviceId,
  getDeviceTemplateId,
  getDeviceConfig,
  loadTelemetryLogList,
  loadDeviceControlList,
  deleteTelemetryData,
  publishTelemetryData,
  refreshTelemetry,
  translate
}: UseTelemetryOperationsSectionOptions) => {
  const showLog = ref(shouldShowSimulationEntry(getDeviceConfig()))
  const logSectionVisible = ref(false)
  const controlListLoaded = ref(false)
  const controlListLoading = ref(false)
  const controlList = ref<TelemetryControlItem[]>([])
  const delparam = ref<TelemetryDeleteParams | null>(null)
  let controlListRequestId = 0
  let pendingControlListRequest: PendingControlListRequest | null = null
  const {
    fetchData,
    fetchFirstLogPage,
    handleLogPageChange,
    loading,
    operationOptions,
    operationType,
    resultOptions,
    resetLogState,
    sendResult,
    tableData,
    total
  } = useTelemetryLogState({
    getDeviceId,
    loadTelemetryLogList,
    translate
  })

  const options = ref([
    {
      label: translate('custom.device_details.deleteAttribute'),
      key: '1'
    }
  ])

  const refreshOperationsSection = () => {
    if (logSectionVisible.value) {
      void fetchData()
    }
    void refreshTelemetry()
  }

  const syncSimulationEntryVisibility = () => {
    showLog.value = shouldShowSimulationEntry(getDeviceConfig())
  }

  const resetControlList = () => {
    controlListRequestId += 1
    pendingControlListRequest = null
    controlList.value = []
    controlListLoaded.value = false
    controlListLoading.value = false
  }

  const resetDeferredSections = () => {
    resetControlList()
    logSectionVisible.value = false
    resetLogState()
  }

  const ensureControlList = async () => {
    const deviceTemplateId = getDeviceTemplateId()
    if (!deviceTemplateId) return
    if (controlListLoaded.value) return
    if (pendingControlListRequest?.templateId === deviceTemplateId) return pendingControlListRequest.promise

    const requestId = controlListRequestId + 1
    controlListRequestId = requestId
    controlListLoading.value = true
    const requestPromise = (async () => {
      try {
        const list = await loadTelemetryControlList(deviceTemplateId, loadDeviceControlList)
        if (controlListRequestId !== requestId || getDeviceTemplateId() !== deviceTemplateId) return
        controlList.value = list
        controlListLoaded.value = true
      } finally {
        if (pendingControlListRequest?.requestId === requestId) {
          pendingControlListRequest = null
        }

        if (controlListRequestId === requestId && getDeviceTemplateId() === deviceTemplateId) {
          controlListLoading.value = false
        }
      }
    })()

    pendingControlListRequest = {
      templateId: deviceTemplateId,
      requestId,
      promise: requestPromise
    }

    return requestPromise
  }

  const openOperationLogs = () => {
    logSectionVisible.value = true
    fetchFirstLogPage()
  }

  const handleDeleteTable = async () => {
    if (!delparam.value) return
    const deleted = await deleteTelemetryItem(delparam.value, deleteTelemetryData)
    if (deleted) {
      refreshOperationsSection()
    }
  }

  const handleSelect = (key: string | number, item: Pick<TelemetryControlItem, 'key'>) => {
    if (String(key) === '1') {
      delparam.value = buildDeleteParams(item, getDeviceId())
      void handleDeleteTable()
    }
  }

  const onControlChange = async (row: Pick<TelemetryControlItem, 'content'>) => {
    await publishTelemetryData(buildControlPublishPayload(getDeviceId(), row.content || ''))
    refreshOperationsSection()
  }

  const initializeTelemetryView = () => {
    void refreshTelemetry()
    syncSimulationEntryVisibility()
  }

  watch(
    () => getDeviceTemplateId(),
    () => {
      resetControlList()
    }
  )

  watch(
    () => getDeviceId(),
    () => {
      resetDeferredSections()
      syncSimulationEntryVisibility()
      void refreshTelemetry()
    }
  )

  watch(() => getDeviceConfig(), syncSimulationEntryVisibility)

  onMounted(() => {
    initializeTelemetryView()
  })

  return {
    controlList,
    controlListLoaded,
    controlListLoading,
    ensureControlList,
    fetchData,
    fetchFirstLogPage,
    handleLogPageChange,
    handleSelect,
    loading,
    operationOptions,
    operationType,
    options,
    resultOptions,
    sendResult,
    showLog,
    logSectionVisible,
    tableData,
    total,
    openOperationLogs,
    onControlChange,
    refreshOperationsSection
  }
}
