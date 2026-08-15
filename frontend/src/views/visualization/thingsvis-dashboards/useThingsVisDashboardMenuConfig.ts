import { onBeforeUnmount, ref, type ComputedRef } from 'vue'
import {
  getDefaultVisualizationProviderFacade,
  type VisualizationDashboardSummary
} from '@/service/visualization-provider/index'
import {
  deleteDashboardMenuConfig,
  fetchDashboardMenuConfig,
  fetchDashboardMenuConfigs,
  saveDashboardMenuConfig,
  type DashboardMenuConfig
} from '@/service/api/dashboard-menu'
import { refreshAuthRoutes } from '@/utils/router/refresh-auth-routes'

type MenuMessageApi = {
  success: (content: string) => void
  error: (content: string) => void
}

type UseThingsVisDashboardMenuConfigOptions = {
  t: (key: string, params?: Record<string, unknown>) => string
  message: MenuMessageApi
  getRouteFullPath: () => string
  providerId: ComputedRef<string>
  dashboardMenuConfigAvailable: ComputedRef<boolean>
}

const MENU_CONFIG_CONCURRENCY = 5
const MENU_CONFIG_PREFETCH_DELAY_MS = 120

export function useThingsVisDashboardMenuConfig(options: UseThingsVisDashboardMenuConfigOptions) {
  const provider = getDefaultVisualizationProviderFacade({ providerId: options.providerId.value })
  const menuConfigs = ref<Record<string, DashboardMenuConfig | null>>({})
  const menuConfigLoadSeq = ref(0)
  const latestDashboardList = ref<VisualizationDashboardSummary[]>([])
  const showMenuModal = ref(false)
  const menuSaving = ref(false)
  const menuConfigRequests = new Map<string, Promise<DashboardMenuConfig | null>>()
  let menuConfigPrefetchDashboardIds = new Set<string>()
  let menuConfigPrefetchPromise: Promise<void> | null = null
  let menuConfigPrefetchTimer: ReturnType<typeof setTimeout> | null = null
  const menuForm = ref({
    dashboardId: '',
    dashboardName: '',
    enabled: false,
    menuName: '',
    menuSort: 1
  })

  const mergeMenuConfigs = (entries: Array<readonly [string, DashboardMenuConfig | null]>) => {
    menuConfigs.value = {
      ...menuConfigs.value,
      ...Object.fromEntries(entries)
    }
  }

  const nextMenuConfigLoadSeq = () => {
    const loadSeq = menuConfigLoadSeq.value + 1
    menuConfigLoadSeq.value = loadSeq
    return loadSeq
  }

  const dashboardIdsFromList = (list: VisualizationDashboardSummary[]) => {
    const ids: string[] = []
    const seen = new Set<string>()
    for (const item of list) {
      if (!item.id || seen.has(item.id)) continue
      seen.add(item.id)
      ids.push(item.id)
    }
    return ids
  }

  const hasMenuConfigEntry = (dashboardId: string) =>
    Object.prototype.hasOwnProperty.call(menuConfigs.value, dashboardId)

  const cancelMenuConfigPrefetchTimer = () => {
    if (menuConfigPrefetchTimer === null) return
    clearTimeout(menuConfigPrefetchTimer)
    menuConfigPrefetchTimer = null
  }

  const cancelMenuConfigPrefetch = () => {
    cancelMenuConfigPrefetchTimer()
    menuConfigPrefetchDashboardIds.clear()
  }

  const menuConfigPrefetchCovers = (dashboardIds: string[]) =>
    dashboardIds.length > 0 && dashboardIds.every(id => menuConfigPrefetchDashboardIds.has(id))

  const startMenuConfigPrefetch = (list: VisualizationDashboardSummary[]) => {
    cancelMenuConfigPrefetchTimer()
    const snapshot = list.slice()
    const dashboardIds = dashboardIdsFromList(snapshot)
    if (menuConfigPrefetchPromise && menuConfigPrefetchCovers(dashboardIds)) {
      return menuConfigPrefetchPromise
    }

    menuConfigPrefetchDashboardIds = new Set(dashboardIds)
    const prefetch = loadMenuConfigs(snapshot).finally(() => {
      if (menuConfigPrefetchPromise === prefetch) {
        menuConfigPrefetchPromise = null
        menuConfigPrefetchDashboardIds.clear()
      }
    })
    menuConfigPrefetchPromise = prefetch
    return prefetch
  }

  const scheduleMenuConfigPrefetch = (list: VisualizationDashboardSummary[]) => {
    cancelMenuConfigPrefetchTimer()
    if (list.length === 0) return

    const snapshot = list.slice()
    menuConfigPrefetchDashboardIds = new Set(dashboardIdsFromList(snapshot))
    menuConfigPrefetchTimer = setTimeout(() => {
      menuConfigPrefetchTimer = null
      void startMenuConfigPrefetch(snapshot)
    }, MENU_CONFIG_PREFETCH_DELAY_MS)
  }

  const registerMenuConfigDashboards = (list: VisualizationDashboardSummary[]) => {
    latestDashboardList.value = list
    if (!options.dashboardMenuConfigAvailable.value) {
      cancelMenuConfigPrefetch()
      mergeMenuConfigs(dashboardIdsFromList(list).map(id => [id, null] as const))
      return
    }

    scheduleMenuConfigPrefetch(list)
  }

  const requestMenuConfig = async (dashboard: VisualizationDashboardSummary) => {
    if (!options.dashboardMenuConfigAvailable.value) return null
    if (!dashboard.id) return null
    if (hasMenuConfigEntry(dashboard.id)) return menuConfigs.value[dashboard.id] ?? null

    if (menuConfigPrefetchDashboardIds.has(dashboard.id)) {
      const sourceList = latestDashboardList.value.length > 0 ? latestDashboardList.value : [dashboard]
      await startMenuConfigPrefetch(sourceList)
      if (hasMenuConfigEntry(dashboard.id)) return menuConfigs.value[dashboard.id] ?? null
    }

    const pendingRequest = menuConfigRequests.get(dashboard.id)
    if (pendingRequest) return pendingRequest

    const request = fetchDashboardMenuConfig(dashboard.id)
      .then(({ data, error }) => {
        const config = error ? (menuConfigs.value[dashboard.id] ?? null) : (data ?? null)
        mergeMenuConfigs([[dashboard.id, config]])
        return config
      })
      .finally(() => {
        menuConfigRequests.delete(dashboard.id)
      })

    menuConfigRequests.set(dashboard.id, request)
    return request
  }

  const loadMenuConfigsIndividually = async (list: VisualizationDashboardSummary[], loadSeq: number) => {
    const entries: Array<readonly [string, DashboardMenuConfig | null]> = []

    for (let index = 0; index < list.length; index += MENU_CONFIG_CONCURRENCY) {
      if (loadSeq !== menuConfigLoadSeq.value) return null

      const batch = list.slice(index, index + MENU_CONFIG_CONCURRENCY)
      const batchEntries = await Promise.all(
        batch.map(async (item) => {
          const { data, error } = await fetchDashboardMenuConfig(item.id)
          return [item.id, error ? (menuConfigs.value[item.id] ?? null) : (data ?? null)] as const
        })
      )
      entries.push(...batchEntries)
    }

    return entries
  }

  const loadMenuConfigs = async (list: VisualizationDashboardSummary[]) => {
    if (!options.dashboardMenuConfigAvailable.value) return

    const loadSeq = nextMenuConfigLoadSeq()
    const unloadedList = list.filter(item => item.id && !hasMenuConfigEntry(item.id) && !menuConfigRequests.has(item.id))
    const dashboardIds = dashboardIdsFromList(unloadedList)
    if (dashboardIds.length === 0) {
      mergeMenuConfigs([])
      return
    }

    const { data, error } = await fetchDashboardMenuConfigs(dashboardIds)
    if (loadSeq !== menuConfigLoadSeq.value) return

    const entries = error
      ? await loadMenuConfigsIndividually(unloadedList, loadSeq)
      : dashboardIds.map(id => [id, data?.[id] ?? null] as const)

    if (!entries || loadSeq !== menuConfigLoadSeq.value) return

    mergeMenuConfigs(entries)
  }

  const ensureAllMenuConfigsLoaded = async () => {
    if (latestDashboardList.value.length === 0) return
    await loadMenuConfigs(latestDashboardList.value)
  }

  const openMenuConfig = async (dashboard: VisualizationDashboardSummary) => {
    if (!options.dashboardMenuConfigAvailable.value) return

    const config = menuConfigs.value[dashboard.id]
    menuForm.value = {
      dashboardId: dashboard.id,
      dashboardName: dashboard.name,
      enabled: Boolean(config?.enabled),
      menuName: config?.menu_name || dashboard.name,
      menuSort: config?.sort || 1
    }
    showMenuModal.value = true

    const latestConfig = await requestMenuConfig(dashboard)
    if (menuForm.value.dashboardId !== dashboard.id) return
    menuForm.value.enabled = Boolean(latestConfig?.enabled)
    menuForm.value.menuName = latestConfig?.menu_name || dashboard.name
    menuForm.value.menuSort = latestConfig?.sort || 1
  }

  const menuSavePayload = () => ({
    menu_name: menuForm.value.menuName.trim(),
    dashboard_name: menuForm.value.dashboardName,
    sort: menuForm.value.menuSort,
    enabled: true
  })

  const showMenuSaveError = (resultError: string) => {
    options.message.error(`${options.t('rdi.thingsvis.menuConfigSaveFailed')}: ${resultError}`)
  }

  const closeMenuConfigModalAfterSave = async () => {
    await refreshAuthRoutes(options.getRouteFullPath())
    showMenuModal.value = false
    options.message.success(options.t('rdi.thingsvis.menuConfigSaved'))
  }

  const saveCurrentMenuConfig = async () => {
    const { data, error } = await saveDashboardMenuConfig(menuForm.value.dashboardId, menuSavePayload())
    const resultError = error?.message || null
    if (resultError) {
      showMenuSaveError(resultError)
      return false
    }

    mergeMenuConfigs([[menuForm.value.dashboardId, data]])
    return true
  }

  const enabledMenuEntries = () => Object.entries(menuConfigs.value).filter(([, cfg]) => cfg?.enabled)

  const isEnabledMenuDashboard = (dashboardId: string) =>
    Object.keys(menuConfigs.value).some((id) => id === dashboardId && menuConfigs.value[id]?.enabled)

  const tryAddHomeDashboardToMenu = async () => {
    await ensureAllMenuConfigsLoaded()
    if (enabledMenuEntries().length !== 1) return

    const homeResult = await provider.execute(current => current.getHomeDashboard())
    const homeDashboard = homeResult.ok ? homeResult.data : null
    if (!homeDashboard || homeDashboard.id === menuForm.value.dashboardId || isEnabledMenuDashboard(homeDashboard.id)) {
      return
    }

    const { error: homeError } = await saveDashboardMenuConfig(homeDashboard.id, {
      menu_name: homeDashboard.name,
      dashboard_name: homeDashboard.name,
      sort: (menuForm.value.menuSort || 1) - 1,
      enabled: true
    })

    if (!homeError) {
      options.message.success(options.t('rdi.thingsvis.homeDashboardAdded', { name: homeDashboard.name }))
    }
  }

  const enableMenuConfig = async () => {
    const saved = await saveCurrentMenuConfig()
    if (!saved) return

    await tryAddHomeDashboardToMenu()
    await closeMenuConfigModalAfterSave()
  }

  const disableMenuConfig = async () => {
    const { error } = await deleteDashboardMenuConfig(menuForm.value.dashboardId)
    const resultError = error?.message || null
    if (resultError) {
      showMenuSaveError(resultError)
      return
    }

    mergeMenuConfigs([[menuForm.value.dashboardId, null]])
    await closeMenuConfigModalAfterSave()
  }

  const handleSaveMenuConfig = async () => {
    if (!options.dashboardMenuConfigAvailable.value) return
    if (!menuForm.value.dashboardId) return

    if (menuForm.value.enabled && !menuForm.value.menuName.trim()) {
      options.message.error(options.t('rdi.thingsvis.menuNamePlaceholder'))
      return
    }

    menuConfigLoadSeq.value += 1
    menuSaving.value = true
    try {
      if (menuForm.value.enabled) {
        await enableMenuConfig()
      } else {
        await disableMenuConfig()
      }
    } finally {
      menuSaving.value = false
    }
  }

  onBeforeUnmount(cancelMenuConfigPrefetch)

  return {
    menuConfigs,
    showMenuModal,
    menuSaving,
    menuForm,
    registerMenuConfigDashboards,
    requestMenuConfig,
    openMenuConfig,
    handleSaveMenuConfig
  }
}
