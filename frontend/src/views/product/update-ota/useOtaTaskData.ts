import { computed, ref } from 'vue'
import type { SelectOption } from 'naive-ui'
import { useListPage } from '@/components/data-table-page/useListPage'
import { deviceList } from '@/service/api/device'
import { getOtaPackageList } from '@/service/product/update-package'
import { getOtaTaskDetail, getOtaTaskList } from '@/service/product/update-ota'
import {
  buildOtaDeviceOptions,
  extractList,
  extractTotal,
  mergeOtaDeviceCandidates,
  type OtaDeviceCandidate
} from './ota-task-state'
import type { OtaPackageRecord, OtaTaskDetailRecord, OtaTaskRecord, OtaTaskStatisticsItem } from './ota-task-types'

const OTA_DEVICE_SELECT_PAGE_SIZE = 50
const OTA_PACKAGE_SELECT_PAGE_SIZE = 20

export const useOtaTaskData = () => {
  const packageLoading = ref(false)
  const deviceLoading = ref(false)
  const packageList = ref<OtaPackageRecord[]>([])
  const detailStatistics = ref<OtaTaskStatisticsItem[]>([])
  const deviceCandidates = ref<OtaDeviceCandidate[]>([])
  const deviceOptions = ref<SelectOption[]>([])
  const selectedPackageId = ref<string | null>(null)
  const selectedTask = ref<OtaTaskRecord | null>(null)
  const packageSearchKeyword = ref('')
  let packageRequestSeq = 0
  let deviceRequestSeq = 0

  // 任务列表与任务明细都是“筛选 + 服务端分页”列表，统一交给 useListPage：
  // 它负责页码/条数联动、加载态和过期请求丢弃（切换升级包或任务时旧响应不会回写）。
  const tasks = useListPage<OtaTaskRecord, { ota_upgrade_package_id: string | null }>({
    initialQuery: () => ({ ota_upgrade_package_id: null }),
    pageSizes: [10, 20, 50],
    serialize: () => ({ ota_upgrade_package_id: selectedPackageId.value }),
    fetcher: async (params) => {
      if (!params.ota_upgrade_package_id) return { list: [], total: 0 }
      const { data, error } = await getOtaTaskList(params)
      if (error || params.ota_upgrade_package_id !== selectedPackageId.value) return null
      return { list: extractList(data) as OtaTaskRecord[], total: extractTotal(data) }
    }
  })

  const detail = useListPage<
    OtaTaskDetailRecord,
    { device_name: string; task_status: number | null }
  >({
    initialQuery: () => ({ device_name: '', task_status: null }),
    pageSizes: [10, 20, 50],
    serialize: (q) => ({
      ota_upgrade_task_id: selectedTask.value?.id,
      device_name: q.device_name,
      task_status: q.task_status || undefined
    }),
    fetcher: async (params) => {
      const taskId = (params as { ota_upgrade_task_id?: string }).ota_upgrade_task_id
      if (!taskId) return null
      const { data, error } = await getOtaTaskDetail(params)
      if (error || taskId !== selectedTask.value?.id) return null
      pendingDetailStatistics = Array.isArray(data?.statistics) ? data.statistics : []
      return { list: extractList(data) as OtaTaskDetailRecord[], total: extractTotal(data) }
    },
    onLoaded: () => {
      detailStatistics.value = pendingDetailStatistics
    }
  })
  let pendingDetailStatistics: OtaTaskStatisticsItem[] = []

  const taskLoading = tasks.loading
  const detailLoading = detail.loading
  const taskList = tasks.rows
  const detailList = detail.rows
  const taskPagination = tasks.pagination
  const detailPagination = detail.pagination
  const detailQuery = detail.query

  const selectedPackage = computed(() => packageList.value.find((item) => item.id === selectedPackageId.value) || null)

  // 显式收窄为 { label: string; value: string }: 映射结果必定是纯字符串标签/值,
  // 而 naive-ui 的 SelectOption 允许 label 为 undefined 或渲染函数,宽类型无法反向赋给子组件。
  const packageOptions = computed<Array<{ label: string; value: string }>>(() =>
    packageList.value.map((item) => ({
      label: `${item.name || item.version || item.id}${item.version ? ` (${item.version})` : ''}`,
      value: item.id
    }))
  )

  const mergePackageOptionsWithSelected = (rows: OtaPackageRecord[]) => {
    const selected = packageList.value.find((item) => item.id === selectedPackageId.value)
    if (!selected || rows.some((item) => item.id === selected.id)) return rows
    return [selected, ...rows]
  }

  const fetchPackages = async (search = packageSearchKeyword.value) => {
    const requestSeq = ++packageRequestSeq
    const normalizedSearch = search.trim()
    packageSearchKeyword.value = normalizedSearch
    packageLoading.value = true
    let packageSelectionChanged = false
    try {
      const { data, error } = await getOtaPackageList({
        page: 1,
        page_size: OTA_PACKAGE_SELECT_PAGE_SIZE,
        ...(normalizedSearch ? { name: normalizedSearch } : {})
      })
      if (requestSeq !== packageRequestSeq) return false
      if (!error) {
        packageList.value = mergePackageOptionsWithSelected(extractList(data) as OtaPackageRecord[])
        if (!selectedPackageId.value && packageList.value[0]) {
          selectedPackageId.value = packageList.value[0].id
          packageSelectionChanged = true
        }
      }
    } finally {
      if (requestSeq === packageRequestSeq) {
        packageLoading.value = false
      }
    }
    return packageSelectionChanged
  }

  const fetchTasks = async () => {
    if (!selectedPackageId.value) {
      tasks.clear()
      return
    }
    await tasks.load()
  }

  const fetchDevices = async (search = '', pageSize = OTA_DEVICE_SELECT_PAGE_SIZE) => {
    const requestSeq = ++deviceRequestSeq
    const normalizedSearch = search.trim()
    deviceLoading.value = true
    try {
      const { data, error } = await deviceList({
        page: 1,
        page_size: pageSize,
        device_config_id: selectedPackage.value?.device_config_id || '',
        ...(normalizedSearch ? { search: normalizedSearch } : {})
      })
      if (requestSeq !== deviceRequestSeq) return
      if (error) return
      const rows = extractList(data) as OtaDeviceCandidate[]
      deviceCandidates.value = mergeOtaDeviceCandidates(deviceCandidates.value, rows)
      deviceOptions.value = buildOtaDeviceOptions(deviceCandidates.value)
    } finally {
      if (requestSeq === deviceRequestSeq) {
        deviceLoading.value = false
      }
    }
  }

  const clearDeviceCandidates = () => {
    deviceRequestSeq += 1
    deviceCandidates.value = []
    deviceOptions.value = []
    deviceLoading.value = false
  }

  const fetchTaskDetails = async () => {
    if (!selectedTask.value?.id) return
    await detail.load()
  }

  const resetTaskPage = () => {
    tasks.page.value = 1
  }

  const resetDetailQuery = () => {
    detail.query.device_name = ''
    detail.query.task_status = null
    detail.page.value = 1
    void fetchTaskDetails()
  }

  const openTaskDetail = async (row: OtaTaskRecord) => {
    detail.cancel()
    selectedTask.value = row
    detail.query.device_name = ''
    detail.query.task_status = null
    detail.page.value = 1
    await fetchTaskDetails()
  }

  return {
    packageLoading,
    taskLoading,
    detailLoading,
    deviceLoading,
    taskList,
    detailList,
    detailStatistics,
    deviceCandidates,
    deviceOptions,
    selectedPackageId,
    selectedTask,
    selectedPackage,
    packageOptions,
    detailQuery,
    taskPagination,
    detailPagination,
    fetchPackages,
    fetchTasks,
    fetchDevices,
    fetchTaskDetails,
    openTaskDetail,
    resetTaskPage,
    resetDetailQuery,
    clearDeviceCandidates
  }
}
