// 文件用途：承载首页布局解析编排（ThingsVis 仪表盘探测、兼容首页配置与超管初始化分支）。
// 核心逻辑：从 home/index.vue 抽出的数据编排层，负责缓存读取、预算内加载、后台刷新与状态回写。
// 关键注意事项：对外仅暴露与原页面同名同语义的状态与方法，调用方需注入鉴权与租户上下文。
import { computed, ref, type Ref } from 'vue'
import { normalizeLocalDashboard } from '@/components/local-visualization-viewer'
import { fetchCompatHomeConfig } from '@/service/api'
import {
  loadVisualizationHomeDashboard,
  probeVisualizationHomeDashboard,
  type VisualizationHomeDashboard,
  type VisualizationHomeLoadResult
} from '@/service/visualization-provider/home-dashboard'
import { $t } from '@/locales'
import { clearThingsVisHomeCache, readThingsVisHomeCache, writeThingsVisHomeCache } from '@/utils/thingsvis/home-cache'
import { NATIVE_BOARD_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'
import { useViewportDeferredMount } from './useViewportDeferredMount'

export const isNativeHomeProvider = import.meta.env.VITE_ENABLE_THINGSVIS_COMPAT !== 'Y'

export function isCompleteThingsVisDashboard(dashboard?: VisualizationHomeDashboard | null): boolean {
  if (!dashboard || !dashboard.canvasConfig || typeof dashboard.canvasConfig !== 'object') return false

  if (dashboard.providerId === NATIVE_BOARD_PROVIDER_ID) {
    return normalizeLocalDashboard(dashboard.rendererData).ok
  }

  if (!Array.isArray(dashboard.nodes) || dashboard.nodes.length === 0) return false
  if (!Array.isArray(dashboard.dataSources) || dashboard.dataSources.length === 0) return false
  return true
}

type UseHomeLayoutResolverOptions = {
  isSysAdmin: Readonly<Ref<boolean>>
  nativeTenantId: Readonly<Ref<string>>
  hasNativeHomeTenantContext: Readonly<Ref<boolean>>
  shouldShowSecondarySections: Readonly<Ref<boolean>>
  scheduleIdleTask: (task: () => void, fallbackDelay?: number) => void
}

export function useHomeLayoutResolver(options: UseHomeLayoutResolverOptions) {
  const { isSysAdmin, nativeTenantId, hasNativeHomeTenantContext, shouldShowSecondarySections, scheduleIdleTask } =
    options

  const layoutFetched = ref(false)
  const hasCompatHomeConfig = ref(false)
  const compatHomeConfigCount = ref(0)
  const isError = ref<boolean>(false)
  const showSysAdminSetup = ref(false)

  // ThingsVis 首页相关
  const thingsVisHome = ref<VisualizationHomeDashboard | null>(null)
  const useThingsVis = ref(false)
  const isThingsVisLoading = ref(false)
  const homeThingsVisSectionRef = ref<HTMLElement | null>(null)
  const homeThingsVisFrameMount = useViewportDeferredMount(homeThingsVisSectionRef, {
    rootMargin: '360px 0px',
    fallbackDelay: 600
  })
  const shouldMountHomeThingsVisFrame = homeThingsVisFrameMount.shouldMount
  const showCompatHomeNotice = computed(() => !useThingsVis.value && layoutFetched.value && hasCompatHomeConfig.value)
  const isHomeResolving = computed(() => !layoutFetched.value || isThingsVisLoading.value)

  const nativeTenantContextRequired = ref(false)
  const homeDashboardUnavailable = ref(false)
  const sysAdminSetupTitle = computed(() => {
    if (nativeTenantContextRequired.value) return $t('custom.home.sysAdminSetup.nativeTenantContextTitle')
    if (homeDashboardUnavailable.value) {
      return $t(
        isNativeHomeProvider
          ? 'custom.home.sysAdminSetup.nativeUnavailableTitle'
          : 'custom.home.sysAdminSetup.unavailableTitle'
      )
    }
    return $t('custom.home.sysAdminSetup.missingDashboardTitle')
  })
  const sysAdminSetupDescription = computed(() => {
    if (nativeTenantContextRequired.value) return $t('custom.home.sysAdminSetup.nativeTenantContextDescription')
    if (homeDashboardUnavailable.value) {
      return $t(
        isNativeHomeProvider
          ? 'custom.home.sysAdminSetup.nativeUnavailableDescription'
          : 'custom.home.sysAdminSetup.unavailableDescription'
      )
    }
    return $t('custom.home.sysAdminSetup.missingDashboardDescription')
  })

  // ThingsVis 请求失败时的重试状态（针对超管首次登录场景）
  const thingsVisRetryCount = ref(0)
  const MAX_THINGSVIS_RETRY = 5 // 增加到 5 次重试
  const THINGSVIS_HOME_DASHBOARD_BUDGET_MS = 1800

  const homeResolvingDescription = computed(() => {
    if (thingsVisRetryCount.value > 0) {
      return $t('custom.home.resolvingRetryDescription', { count: thingsVisRetryCount.value })
    }

    if (isThingsVisLoading.value) {
      return $t('custom.home.resolvingThingsVisDescription')
    }

    return $t('custom.home.resolvingDescription')
  })

  const parseCompatHomeConfigCount = (config: unknown): number => {
    if (Array.isArray(config)) return config.length
    if (config && typeof config === 'object' && Array.isArray((config as { layout?: unknown[] }).layout)) {
      return (config as { layout: unknown[] }).layout.length
    }
    return 0
  }

  const loadCompatHomeState = async () => {
    const { data, error } = await fetchCompatHomeConfig({})

    if (error || !data?.config) {
      isError.value = Boolean(error)
      hasCompatHomeConfig.value = false
      compatHomeConfigCount.value = 0
      layoutFetched.value = true
      return
    }

    try {
      const configJson = JSON.parse(data.config)
      compatHomeConfigCount.value = parseCompatHomeConfigCount(configJson)
      hasCompatHomeConfig.value = compatHomeConfigCount.value > 0
      isError.value = false
    } catch {
      isError.value = true
      hasCompatHomeConfig.value = false
      compatHomeConfigCount.value = 0
    }
    layoutFetched.value = true
  }

  const loadThingsVisHomeDashboardWithBudget = async (
    fallbackDashboard: VisualizationHomeDashboard | null,
    budgetMs = THINGSVIS_HOME_DASHBOARD_BUDGET_MS,
    tenantId?: string
  ): Promise<VisualizationHomeLoadResult & { timedOut?: boolean }> => {
    if (fallbackDashboard) {
      return { ok: true, data: fallbackDashboard, timedOut: false }
    }

    let timeoutId: ReturnType<typeof setTimeout> | undefined
    const dashboardRequest = loadVisualizationHomeDashboard(tenantId ? { tenantId } : undefined)
    const timeoutRequest = new Promise<{ result: VisualizationHomeLoadResult; timedOut: true }>((resolve) => {
      timeoutId = setTimeout(() => resolve({ result: { ok: true, data: null }, timedOut: true }), budgetMs)
    })

    const result = await Promise.race([
      dashboardRequest.then((result) => ({ result, timedOut: false as const })),
      timeoutRequest
    ])
    if (timeoutId) clearTimeout(timeoutId)
    if (result.timedOut && !isSysAdmin.value) {
      void dashboardRequest
        .then((lateResult) => {
          const dashboard = lateResult.ok ? lateResult.data : null
          if (lateResult.ok && isCompleteThingsVisDashboard(dashboard)) {
            writeThingsVisHomeCache('thingsvis', dashboard)
          }
        })
        .catch(() => {})
    }
    return { ...result.result, timedOut: result.timedOut }
  }

  const refreshThingsVisHomeDashboardInBackground = async () => {
    if (isSysAdmin.value) return
    try {
      const probe = await probeVisualizationHomeDashboard()
      if (!probe.reachable) return
      const result = probe.dashboard
        ? { ok: true as const, data: probe.dashboard }
        : await loadThingsVisHomeDashboardWithBudget(null, THINGSVIS_HOME_DASHBOARD_BUDGET_MS)
      if ('timedOut' in result && result.timedOut) return

      const dashboard = result.ok ? result.data : null
      if (result.ok && isCompleteThingsVisDashboard(dashboard)) {
        thingsVisHome.value = dashboard
        useThingsVis.value = true
        thingsVisRetryCount.value = 0
        writeThingsVisHomeCache('thingsvis', dashboard)
        return
      }

      const resultStatus = result.ok === false ? result.error.status : undefined
      if (!dashboard && (result.ok || resultStatus === 404)) {
        writeThingsVisHomeCache('classic')
      }
    } catch {
      // Keep the current homepage fallback usable when ThingsVis is slow or unavailable.
    }
  }

  let layoutLoadPromise: Promise<void> | null = null

  const resolveHomeLayout = async (retryCount = 0): Promise<void> => {
    isError.value = false
    showSysAdminSetup.value = false
    useThingsVis.value = false
    thingsVisHome.value = null
    nativeTenantContextRequired.value = false
    homeDashboardUnavailable.value = false
    layoutFetched.value = false
    hasCompatHomeConfig.value = false
    compatHomeConfigCount.value = 0

    // Native home boards are tenant-scoped. SYS_ADMIN must select the target
    // tenant from Native boards before this page can read or modify a board;
    // never infer a tenant from the first available record.
    if (isNativeHomeProvider && isSysAdmin.value && !hasNativeHomeTenantContext.value) {
      nativeTenantContextRequired.value = true
      showSysAdminSetup.value = true
      layoutFetched.value = true
      return
    }

    const cachedHome = readThingsVisHomeCache()
    if (
      !isNativeHomeProvider &&
      cachedHome?.state === 'thingsvis' &&
      isCompleteThingsVisDashboard(cachedHome.dashboard)
    ) {
      thingsVisHome.value = cachedHome.dashboard ?? null
      useThingsVis.value = true
      layoutFetched.value = true
      return
    }
    if (cachedHome?.state === 'thingsvis') {
      clearThingsVisHomeCache()
    }

    if (cachedHome?.state === 'sysadmin-setup' && isSysAdmin.value) {
      showSysAdminSetup.value = true
      layoutFetched.value = true
      return
    }

    if (cachedHome?.state === 'classic' && !isSysAdmin.value) {
      await loadCompatHomeState()
      scheduleIdleTask(() => {
        void refreshThingsVisHomeDashboardInBackground()
      }, 300)
      return
    }

    if (!isSysAdmin.value) {
      await loadCompatHomeState()
      scheduleIdleTask(() => {
        void refreshThingsVisHomeDashboardInBackground()
      }, 300)
      return
    }

    const thingsVisProbe = await probeVisualizationHomeDashboard(
      nativeTenantId.value ? { tenantId: nativeTenantId.value } : undefined
    )
    if (!thingsVisProbe.reachable) {
      homeDashboardUnavailable.value = true

      if (isSysAdmin.value) {
        showSysAdminSetup.value = true
        layoutFetched.value = true
        return
      }

      if (isSysAdmin.value) {
        writeThingsVisHomeCache('classic')
      }
      await loadCompatHomeState()
      return
    }

    // 先检查 ThingsVis 是否有首页的仪表盘
    try {
      isThingsVisLoading.value = true
      const thingsVisResult = thingsVisProbe.dashboard
        ? { ok: true as const, data: thingsVisProbe.dashboard }
        : await loadThingsVisHomeDashboardWithBudget(null, THINGSVIS_HOME_DASHBOARD_BUDGET_MS, nativeTenantId.value)
      isThingsVisLoading.value = false
      if ('timedOut' in thingsVisResult && thingsVisResult.timedOut) {
        if (isSysAdmin.value) {
          showSysAdminSetup.value = true
          layoutFetched.value = true
          return
        }
        await loadCompatHomeState()
        return
      }
      const homeNotConfigured = thingsVisResult.ok ? !thingsVisResult.data : thingsVisResult.error.status === 404
      const dashboard = thingsVisResult.ok ? thingsVisResult.data : null
      if (thingsVisResult.ok && dashboard) {
        thingsVisHome.value = dashboard
        useThingsVis.value = true
        layoutFetched.value = true
        thingsVisRetryCount.value = 0
        writeThingsVisHomeCache('thingsvis', dashboard)
        return
      }

      // ThingsVis 请求成功但没有数据，可能是首次登录看板尚未创建
      // 等待一下再重试
      if (homeNotConfigured && isSysAdmin.value && retryCount < MAX_THINGSVIS_RETRY) {
        thingsVisRetryCount.value = retryCount + 1
        await new Promise((resolve) => setTimeout(resolve, 500 * (retryCount + 1)))
        return resolveHomeLayout(retryCount + 1)
      }

      if (isSysAdmin.value) {
        if (!homeNotConfigured && !thingsVisResult.ok) {
          isError.value = true
          layoutFetched.value = true
          return
        }

        showSysAdminSetup.value = true
        layoutFetched.value = true
        thingsVisRetryCount.value = 0
        writeThingsVisHomeCache('sysadmin-setup')
        return
      }

      if (homeNotConfigured) {
        writeThingsVisHomeCache('classic')
      }
    } catch (e) {
      // ThingsVis 服务不可用，继续使用现有首页兜底
      isThingsVisLoading.value = false
      homeDashboardUnavailable.value = true
      if (isSysAdmin.value) {
        showSysAdminSetup.value = true
        layoutFetched.value = true
        return
      }
    }

    // 使用轻量兼容首页提示，避免再走已退场的 classic card 渲染链
    await loadCompatHomeState()
  }

  const getLayout = async () => {
    if (layoutLoadPromise) return layoutLoadPromise

    const currentLoad = resolveHomeLayout().finally(() => {
      if (layoutLoadPromise === currentLoad) {
        layoutLoadPromise = null
      }
    })
    layoutLoadPromise = currentLoad
    return currentLoad
  }

  const mountHomeThingsVisFrame = () => {
    if (!useThingsVis.value || !thingsVisHome.value) return
    homeThingsVisFrameMount.mountNow()
  }

  const resetHomeThingsVisFrame = () => {
    homeThingsVisFrameMount.reset()
  }

  const observeHomeThingsVisFrame = async () => {
    if (!shouldShowSecondarySections.value || !useThingsVis.value || !thingsVisHome.value) {
      homeThingsVisFrameMount.reset()
      return
    }

    if (shouldMountHomeThingsVisFrame.value) return
    await homeThingsVisFrameMount.observe()
  }

  return {
    layoutFetched,
    hasCompatHomeConfig,
    compatHomeConfigCount,
    isError,
    showSysAdminSetup,
    thingsVisHome,
    useThingsVis,
    nativeTenantContextRequired,
    homeDashboardUnavailable,
    showCompatHomeNotice,
    isHomeResolving,
    sysAdminSetupTitle,
    sysAdminSetupDescription,
    homeResolvingDescription,
    homeThingsVisSectionRef,
    shouldMountHomeThingsVisFrame,
    mountHomeThingsVisFrame,
    resetHomeThingsVisFrame,
    observeHomeThingsVisFrame,
    refreshThingsVisHomeDashboardInBackground,
    getLayout
  }
}
