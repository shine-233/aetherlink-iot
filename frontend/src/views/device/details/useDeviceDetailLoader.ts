/**
 * 文件用途: 设备详情页详情加载与路由同步 composable。
 * 核心逻辑: 维护路由 d_id/tab、详情数据快照与请求序号；统一 reload 入口负责
 * “跨设备清理 -> 请求详情 -> 丢弃过期响应 -> 同步状态与 tab -> 订阅在线状态 -> 激活请求 tab”。
 * 关键注意事项:
 * 1. 详情请求失败时保留现有页面态；跨设备切换是例外，旧设备数据立即失效。
 * 2. 只有最新序号且仍匹配当前路由 d_id 的响应可以提交状态（isCurrentRequest）。
 * 3. 分享只读模式下只保留 sharedReadOnlySafe 的 tab，并回落到 message tab。
 */
import { computed, ref } from 'vue'
import type { Ref } from 'vue'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { deviceDetail } from '@/service/api/device'
import { createLogger } from '@/utils/logger'
import { normalizeDeviceDetailState, type DeviceDetailData } from './device-edit-state'
import { normalizeOnlineStatusUpdatedAt } from './device-online-status-frame'
import { isSharedRouteQuery, normalizeRouteQueryParam } from './device-detail-route'
import type { useDeviceDetailTabsController } from './useDeviceDetailTabsController'
import type { DeviceOnlineStatusSocket } from './useDeviceOnlineStatusSocket'

const logger = createLogger('DeviceDetail')

export type DeviceDetailReloadOptions = {
  refreshActiveTab?: boolean
}

type TabsController = ReturnType<typeof useDeviceDetailTabsController>

type UseDeviceDetailLoaderOptions = {
  route: RouteLocationNormalizedLoaded
  tabs: TabsController
  onlineStatus: DeviceOnlineStatusSocket
  /** 由 tabs controller 的 onDeviceTypeChange 写入，跨设备切换时在这里清空。 */
  deviceType: Ref<string>
  /** 跨设备切换或进入只读分享时需要关闭的页面弹窗。 */
  onResetDialogs: (reason: 'route-transition' | 'shared-read-only') => void
}

export function useDeviceDetailLoader(options: UseDeviceDetailLoaderOptions) {
  const { route, tabs, onlineStatus } = options

  const routeDeviceId = ref(normalizeRouteQueryParam(route.query.d_id))
  const routeTabKey = ref(normalizeRouteQueryParam(route.query.tab))
  const getDeviceId = () => routeDeviceId.value

  const deviceData = ref<DeviceDetailData>({})
  const loadedDeviceId = ref('')
  const labels = ref<string[]>([])
  const name = ref('')
  const deviceNumber = ref('')
  const { deviceType } = options
  const deviceLoop = ref(false)

  const sharedRouteRequested = computed(() => isSharedRouteQuery(route.query))
  const isSharedReadOnly = computed(() => sharedRouteRequested.value || deviceData.value?.shared_read_only === true)
  const isCurrentDeviceDetailLoaded = computed(() => {
    const currentDeviceId = getDeviceId()
    return Boolean(
      currentDeviceId &&
      currentDeviceId === normalizeRouteQueryParam(route.query.d_id) &&
      loadedDeviceId.value === currentDeviceId &&
      normalizeRouteQueryParam(deviceData.value?.id) === currentDeviceId
    )
  })
  const canUseOwnerDetailActions = computed(() => isCurrentDeviceDetailLoaded.value && !isSharedReadOnly.value)
  const visibleDetailComponents = computed(() => {
    if (!isCurrentDeviceDetailLoaded.value) return []
    return isSharedReadOnly.value
      ? tabs.components.value.filter((component) => component.sharedReadOnlySafe === true)
      : tabs.components.value
  })

  function syncRouteState() {
    routeDeviceId.value = normalizeRouteQueryParam(route.query.d_id)
    routeTabKey.value = normalizeRouteQueryParam(route.query.tab)
  }

  function syncRouteTabKey(value: unknown) {
    routeTabKey.value = normalizeRouteQueryParam(value)
  }

  function syncDeviceDetailState(data: DeviceDetailData) {
    const normalized = normalizeDeviceDetailState(data)
    deviceData.value = data
    if (data?.shared_read_only === true) options.onResetDialogs('shared-read-only')
    labels.value = normalized.labels
    deviceNumber.value = normalized.deviceNumber as string
    onlineStatus.isOnline.value = normalized.isOnline as number
    onlineStatus.updatedAt.value = normalizeOnlineStatusUpdatedAt(data, getDeviceId())
    name.value = normalized.name as string
  }

  let requestSeq = 0

  function isCurrentRequest(seq: number, requestDeviceId: string) {
    return seq === requestSeq && normalizeRouteQueryParam(route.query.d_id) === requestDeviceId
  }

  function clearForRouteTransition(requestDeviceId: string) {
    if (!loadedDeviceId.value || loadedDeviceId.value === requestDeviceId) return
    loadedDeviceId.value = ''
    deviceData.value = {}
    labels.value = []
    name.value = ''
    deviceNumber.value = ''
    deviceType.value = ''
    onlineStatus.reset()
    options.onResetDialogs('route-transition')
  }

  async function requestDeviceDetail(currentDeviceId: string) {
    const seq = ++requestSeq
    clearForRouteTransition(currentDeviceId)
    deviceLoop.value = false
    const { error, data } = await deviceDetail(currentDeviceId)

    if (!isCurrentRequest(seq, currentDeviceId)) {
      logger.info('[DeviceDetail] Discard stale detail response.', { deviceId: currentDeviceId, requestSeq: seq })
      return null
    }

    deviceLoop.value = true

    if (error || !data) {
      logger.warn('[DeviceDetail] Skip detail state reset because request failed.', {
        deviceId: currentDeviceId,
        hasData: Boolean(data),
        error: error instanceof Error ? error.message : error
      })
      return null
    }

    return { data, seq }
  }

  async function loadAndSyncDeviceDetail(currentDeviceId: string) {
    const result = await requestDeviceDetail(currentDeviceId)
    if (!result || !isCurrentRequest(result.seq, currentDeviceId)) return false

    syncDeviceDetailState(result.data)
    await tabs.syncDeviceDetailTabs(result.data)
    if (!isCurrentRequest(result.seq, currentDeviceId)) return false

    loadedDeviceId.value = currentDeviceId
    onlineStatus.subscribe(currentDeviceId)
    return true
  }

  function activateRequestedDetailTab(requestedTabKey: string) {
    if (!isSharedReadOnly.value) {
      tabs.activateTabIfVisible(requestedTabKey)
      return
    }
    const visibleComponents = visibleDetailComponents.value
    if (!visibleComponents.length) return
    const requested = visibleComponents.find((component) => component.key === requestedTabKey)
    const fallback = visibleComponents.find((component) => component.key === 'message') || visibleComponents[0]
    tabs.changeTabs((requested || fallback).key)
  }

  async function reloadDeviceDetail(currentDeviceId: string, reloadOptions: DeviceDetailReloadOptions = {}) {
    if (!currentDeviceId) return
    const hasSynced = await loadAndSyncDeviceDetail(currentDeviceId)
    if (!hasSynced) return
    activateRequestedDetailTab(routeTabKey.value)
    tabs.refreshActiveTabIfNeeded(reloadOptions.refreshActiveTab)
  }

  /** 路由刷新、弹窗关闭、保存成功和子模块变更都统一走这个详情重载入口。 */
  async function reload(reloadOptions: DeviceDetailReloadOptions = {}) {
    await reloadDeviceDetail(getDeviceId(), reloadOptions)
  }

  async function reloadFromRoute(reloadOptions: DeviceDetailReloadOptions = {}) {
    syncRouteState()
    await reloadDeviceDetail(getDeviceId(), reloadOptions)
  }

  return {
    getDeviceId,
    routeTabKey,
    deviceData,
    loadedDeviceId,
    labels,
    name,
    deviceNumber,
    deviceType,
    deviceLoop,
    isSharedReadOnly,
    canUseOwnerDetailActions,
    visibleDetailComponents,
    syncRouteTabKey,
    activateRequestedDetailTab,
    reload,
    reloadFromRoute
  }
}
