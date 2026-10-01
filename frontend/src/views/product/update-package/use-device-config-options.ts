/**
 * 文件用途: 升级包页面的设备配置远程搜索选项（列表筛选与表单弹窗共用一份选项）。
 * 核心逻辑: 请求序号丢弃过期响应、选中项跨搜索保留、250ms 防抖搜索；定时器随作用域销毁清理。
 * 关键注意事项: getSelectedId 由调用方传入当前筛选值，保证远程搜索结果不会挤掉已选配置。
 * 重构建议: 若其它 OTA 页面需要同样的选项逻辑，可下沉到 service 或共享 hooks 层。
 */
import { getCurrentScope, onScopeDispose, ref } from 'vue'
import { getDeviceConfigList } from '@/service/api/device'
import type { DeviceConfigOption } from './ota-package-types'

const DEVICE_CONFIG_SELECT_PAGE_SIZE = 20
const DEVICE_CONFIG_SEARCH_DELAY = 250

interface UseDeviceConfigOptionsConfig {
  /** 当前已选设备配置 id（列表筛选值），远程搜索合并时保持该选项可见。 */
  getSelectedId?: () => string | null | undefined
}

/** 兼容 list / data.list / records / 裸数组四种载荷形状。 */
function extractDeviceConfigRows(payload: unknown): unknown[] {
  if (Array.isArray(payload)) return payload
  const record = payload as Record<string, unknown> | null | undefined
  if (Array.isArray(record?.list)) return record.list
  const nested = record?.data as Record<string, unknown> | undefined
  if (Array.isArray(nested?.list)) return nested.list
  if (Array.isArray(record?.records)) return record.records
  return []
}

export function useDeviceConfigOptions(config: UseDeviceConfigOptionsConfig = {}) {
  const deviceConfigLoading = ref(false)
  const deviceConfigOptions = ref<DeviceConfigOption[]>([])
  let searchKeyword = ''
  let deviceConfigRequestSeq = 0
  let searchTimer: ReturnType<typeof setTimeout> | undefined

  function normalizeDeviceConfigOptions(rows: unknown[]): DeviceConfigOption[] {
    return rows.map((item) => {
      const fields = item as { name?: string; device_config_name?: string; id?: string }
      return {
        label: fields.name || fields.device_config_name || (fields.id as string),
        value: fields.id as string
      }
    })
  }

  /** 编辑行回填时把已选配置补进选项，避免远程搜索结果里没有它导致只显示裸 id。 */
  function ensureDeviceConfigOption(option: DeviceConfigOption | null | undefined) {
    if (!option?.value) return
    const exists = deviceConfigOptions.value.some((item) => item.value === option.value)
    if (!exists) {
      deviceConfigOptions.value = [option, ...deviceConfigOptions.value]
    }
  }

  function mergeDeviceConfigOptions(rows: DeviceConfigOption[]) {
    const selectedId = config.getSelectedId?.()
    const current = selectedId ? deviceConfigOptions.value.find((item) => item.value === selectedId) : undefined
    const next = current && !rows.some((item) => item.value === current.value) ? [current, ...rows] : rows
    const seen = new Set<string>()
    return next.filter((item) => {
      if (seen.has(item.value)) return false
      seen.add(item.value)
      return true
    })
  }

  async function fetchDeviceConfigs(search = searchKeyword) {
    const requestSeq = ++deviceConfigRequestSeq
    const normalizedSearch = search.trim()
    searchKeyword = normalizedSearch
    deviceConfigLoading.value = true

    try {
      const { data, error } = await getDeviceConfigList({
        page: 1,
        page_size: DEVICE_CONFIG_SELECT_PAGE_SIZE,
        ...(normalizedSearch ? { name: normalizedSearch } : {})
      })
      if (requestSeq !== deviceConfigRequestSeq) return
      if (error) return
      deviceConfigOptions.value = mergeDeviceConfigOptions(normalizeDeviceConfigOptions(extractDeviceConfigRows(data)))
    } finally {
      if (requestSeq === deviceConfigRequestSeq) {
        deviceConfigLoading.value = false
      }
    }
  }

  /** NSelect remote 搜索入口：250ms 防抖后拉取选项。 */
  function handleDeviceConfigSearch(search: string) {
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
      void fetchDeviceConfigs(search)
    }, DEVICE_CONFIG_SEARCH_DELAY)
  }

  if (getCurrentScope()) {
    onScopeDispose(() => {
      if (searchTimer) clearTimeout(searchTimer)
    })
  }

  return {
    deviceConfigLoading,
    deviceConfigOptions,
    fetchDeviceConfigs,
    ensureDeviceConfigOption,
    handleDeviceConfigSearch
  }
}
