import { computed, reactive, ref } from 'vue'
import type { PaginationProps, SelectOption } from 'naive-ui'
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
  const taskLoading = ref(false)
  const detailLoading = ref(false)
  const deviceLoading = ref(false)
  const packageList = ref<OtaPackageRecord[]>([])
  const taskList = ref<OtaTaskRecord[]>([])
  const detailList = ref<OtaTaskDetailRecord[]>([])
  const detailStatistics = ref<OtaTaskStatisticsItem[]>([])
  const deviceCandidates = ref<OtaDeviceCandidate[]>([])
  const deviceOptions = ref<SelectOption[]>([])
  const selectedPackageId = ref<string | null>(null)
  const selectedTask = ref<OtaTaskRecord | null>(null)
  const packageSearchKeyword = ref('')
  let packageRequestSeq = 0
  let taskRequestSeq = 0
  let detailRequestSeq = 0
  let deviceRequestSeq = 0

  const taskQuery = reactive({
    page: 1,
    page_size: 10
  })

  const detailQuery = reactive({
    page: 1,
    page_size: 10,
    device_name: '',
    task_status: null as number | null
  })

  const selectedPackage = computed(() => packageList.value.find((item) => item.id === selectedPackageId.value) || null)

  const packageOptions = computed<SelectOption[]>(() =>
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
    const requestSeq = ++taskRequestSeq
    const packageId = selectedPackageId.value
    if (!packageId) {
      taskList.value = []
      taskPagination.itemCount = 0
      taskLoading.value = false
      return
    }
    taskLoading.value = true
    try {
      const { data, error } = await getOtaTaskList({
        page: taskQuery.page,
        page_size: taskQuery.page_size,
        ota_upgrade_package_id: packageId
      })
      if (requestSeq !== taskRequestSeq || packageId !== selectedPackageId.value) return
      if (!error) {
        taskList.value = extractList(data) as OtaTaskRecord[]
        taskPagination.itemCount = extractTotal(data)
      }
    } finally {
      if (requestSeq === taskRequestSeq) {
        taskLoading.value = false
      }
    }
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
    const taskId = selectedTask.value?.id
    if (!taskId) return
    const requestSeq = ++detailRequestSeq
    const query = {
      page: detailQuery.page,
      pageSize: detailQuery.page_size,
      deviceName: detailQuery.device_name,
      taskStatus: detailQuery.task_status
    }
    detailLoading.value = true
    try {
      const { data, error } = await getOtaTaskDetail({
        page: query.page,
        page_size: query.pageSize,
        ota_upgrade_task_id: taskId,
        device_name: query.deviceName,
        task_status: query.taskStatus || undefined
      })
      if (requestSeq !== detailRequestSeq || taskId !== selectedTask.value?.id) return
      if (!error) {
        detailList.value = extractList(data) as OtaTaskDetailRecord[]
        detailStatistics.value = Array.isArray(data?.statistics) ? data.statistics : []
        detailPagination.itemCount = extractTotal(data)
      }
    } finally {
      if (requestSeq === detailRequestSeq) {
        detailLoading.value = false
      }
    }
  }

  const resetTaskPage = () => {
    taskQuery.page = 1
    taskPagination.page = 1
  }

  const resetDetailQuery = () => {
    detailQuery.page = 1
    detailQuery.device_name = ''
    detailQuery.task_status = null
    detailPagination.page = 1
    fetchTaskDetails()
  }

  const openTaskDetail = async (row: OtaTaskRecord) => {
    detailRequestSeq += 1
    selectedTask.value = row
    detailQuery.page = 1
    detailQuery.device_name = ''
    detailQuery.task_status = null
    detailPagination.page = 1
    await fetchTaskDetails()
  }

  const taskPagination: PaginationProps = reactive({
    page: taskQuery.page,
    pageSize: taskQuery.page_size,
    showSizePicker: true,
    pageSizes: [10, 20, 50],
    itemCount: 0,
    onChange: (page) => {
      taskQuery.page = page
      taskPagination.page = page
      fetchTasks()
    },
    onUpdatePageSize: (pageSize) => {
      taskQuery.page = 1
      taskQuery.page_size = pageSize
      taskPagination.page = 1
      taskPagination.pageSize = pageSize
      fetchTasks()
    }
  })

  const detailPagination: PaginationProps = reactive({
    page: detailQuery.page,
    pageSize: detailQuery.page_size,
    showSizePicker: true,
    pageSizes: [10, 20, 50],
    itemCount: 0,
    onChange: (page) => {
      detailQuery.page = page
      detailPagination.page = page
      fetchTaskDetails()
    },
    onUpdatePageSize: (pageSize) => {
      detailQuery.page = 1
      detailQuery.page_size = pageSize
      detailPagination.page = 1
      detailPagination.pageSize = pageSize
      fetchTaskDetails()
    }
  })

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
