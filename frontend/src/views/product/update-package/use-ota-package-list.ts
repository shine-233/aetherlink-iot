import { reactive, ref } from 'vue'
import type { PaginationProps } from 'naive-ui'
import { getDeviceConfigList } from '@/service/api/device'
import { getOtaPackageList } from '@/service/product/update-package'
import type { DeviceConfigOption, OtaPackageRecord } from './ota-package-types'

const DEVICE_CONFIG_SELECT_PAGE_SIZE = 20

function extractList(payload: any): any[] {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.data?.list)) return payload.data.list
  if (Array.isArray(payload?.records)) return payload.records
  return []
}

function extractTotal(payload: any) {
  return Number(payload?.total || payload?.data?.total || 0)
}

export function useOtaPackageList() {
  const loading = ref(false)
  const deviceConfigLoading = ref(false)
  const tableData = ref<OtaPackageRecord[]>([])
  const deviceConfigOptions = ref<DeviceConfigOption[]>([])
  const deviceConfigSearchKeyword = ref('')
  let deviceConfigRequestSeq = 0

  const queryParams = reactive({
    page: 1,
    page_size: 10,
    name: '',
    version: '',
    device_config_id: null as string | null
  })

  function normalizeDeviceConfigOptions(rows: any[]): DeviceConfigOption[] {
    return rows.map((item: any) => ({
      label: item.name || item.device_config_name || item.id,
      value: item.id
    }))
  }

  function ensureDeviceConfigOption(option: DeviceConfigOption | null | undefined) {
    if (!option?.value) return
    const exists = deviceConfigOptions.value.some((item) => item.value === option.value)
    if (!exists) {
      deviceConfigOptions.value = [option, ...deviceConfigOptions.value]
    }
  }

  function mergeDeviceConfigOptions(rows: DeviceConfigOption[]) {
    const current = deviceConfigOptions.value.find((item) => item.value === queryParams.device_config_id)
    const next = current && !rows.some((item) => item.value === current.value) ? [current, ...rows] : rows
    const seen = new Set<string>()
    return next.filter((item) => {
      if (seen.has(item.value)) return false
      seen.add(item.value)
      return true
    })
  }

  async function fetchDeviceConfigs(search = deviceConfigSearchKeyword.value) {
    const requestSeq = ++deviceConfigRequestSeq
    const normalizedSearch = search.trim()
    deviceConfigSearchKeyword.value = normalizedSearch
    deviceConfigLoading.value = true

    try {
      const { data, error } = await getDeviceConfigList({
        page: 1,
        page_size: DEVICE_CONFIG_SELECT_PAGE_SIZE,
        ...(normalizedSearch ? { name: normalizedSearch } : {})
      })
      if (requestSeq !== deviceConfigRequestSeq) return
      if (error) return
      deviceConfigOptions.value = mergeDeviceConfigOptions(normalizeDeviceConfigOptions(extractList(data)))
    } finally {
      if (requestSeq === deviceConfigRequestSeq) {
        deviceConfigLoading.value = false
      }
    }
  }

  async function fetchPackages() {
    loading.value = true
    try {
      const { data, error } = await getOtaPackageList({
        page: queryParams.page,
        page_size: queryParams.page_size,
        name: queryParams.name,
        version: queryParams.version,
        device_config_id: queryParams.device_config_id || ''
      })
      if (!error) {
        tableData.value = extractList(data) as OtaPackageRecord[]
        pagination.itemCount = extractTotal(data)
      }
    } finally {
      loading.value = false
    }
  }

  function resetQuery() {
    queryParams.page = 1
    queryParams.name = ''
    queryParams.version = ''
    queryParams.device_config_id = null
    pagination.page = 1
    fetchPackages()
  }

  const pagination: PaginationProps = reactive({
    page: queryParams.page,
    pageSize: queryParams.page_size,
    showSizePicker: true,
    pageSizes: [10, 20, 50],
    itemCount: 0,
    onChange: (page) => {
      queryParams.page = page
      pagination.page = page
      fetchPackages()
    },
    onUpdatePageSize: (pageSize) => {
      queryParams.page = 1
      queryParams.page_size = pageSize
      pagination.page = 1
      pagination.pageSize = pageSize
      fetchPackages()
    }
  })

  return {
    loading,
    deviceConfigLoading,
    tableData,
    deviceConfigOptions,
    queryParams,
    pagination,
    fetchDeviceConfigs,
    ensureDeviceConfigOption,
    fetchPackages,
    resetQuery
  }
}
