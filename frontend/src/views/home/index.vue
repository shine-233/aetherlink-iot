<!--
文件用途：承载首页和第一台设备接入控制台的页面级视图。
核心逻辑：组合首次接入向导、设备状态、可视化仪表盘、接口请求和国际化文案，完成页面初始化、查询与交互反馈。
关键注意事项：页面通常依赖权限、分页、远端接口和路由状态，改动时需同步检查测试与接口契约。
重构建议：后续可继续拆分数据编排、列配置和弹窗流程，降低页面级组件复杂度。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { router } from '@/router'
import { normalizeLocalDashboard } from '@/components/local-visualization-viewer'
import { fetchCompatHomeConfig } from '@/service/api'
import { sceneAutomationsGet } from '@/service/api/automation'
import { fetchTenantSetupState } from '@/service/api/auth'
import {
  loadVisualizationHomeDashboard,
  probeVisualizationHomeDashboard,
  type VisualizationHomeDashboard,
  type VisualizationHomeLoadResult
} from '@/service/visualization-provider/home-dashboard'
import { $t } from '@/locales'
import { useAuthStore } from '@/store/modules/auth'
import { clearThingsVisHomeCache, readThingsVisHomeCache, writeThingsVisHomeCache } from '@/utils/thingsvis/home-cache'
import { isSysAdminUser } from '@/utils/thingsvis/space'
import { NATIVE_BOARD_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'
import { readNativeBoardTenantContext } from '@/service/visualization-provider/native-tenant-context'
import {
  buildHomeCustomerGuideProgress,
  buildHomeCustomerGuideSummary,
  type HomeCustomerGuideProgressStep,
  type HomeCustomerGuideStep
} from './homeCustomerGuide'
import {
  fetchDeploymentHealthReport,
  normalizeDeploymentHealth,
  type DeploymentHealthReport
} from './homeDeploymentHealth'
import {
  createHomeFirstRunFirstDevice,
  getHomeFirstRunTenantId,
  type HomeFirstRunProtocol,
  type HomeFirstRunQuickCreateResult
} from './homeFirstRunWizard'
import {
  loadHomeFirstRunGuideState,
  saveHomeFirstRunGuideState,
  type HomeFirstRunGuideState
} from './homeFirstRunStorage'
import { createHomeGuideRefreshCoordinator } from './homeGuideRefreshCoordinator'
import { useHomeFirstDeviceWorkbench } from './useHomeFirstDeviceWorkbench'
import { useViewportDeferredMount } from './useViewportDeferredMount'

const HomeSecondaryPanel = defineAsyncComponent(() => import('./HomeSecondaryPanel.vue'))
const HomeFirstDeviceWorkbenchView = defineAsyncComponent(() => import('./HomeFirstDeviceWorkbenchView.vue'))


type HomeFirstDeviceWorkbenchViewExpose = {
  focusDeploymentHealth: () => Promise<void>
  focusSection: (key: string) => Promise<void>
}

type TenantSetupNextStep = 'create_super_admin' | 'create_tenant_admin' | 'login'

type TenantSetupState = {
  has_admin: boolean
  has_tenant_admin?: boolean
  has_tenant?: boolean
  entry: 'login' | 'register'
  next_step?: TenantSetupNextStep
  market_base_url?: string
  market_register_url?: string
}

const defaultTenantSetupState = (): TenantSetupState => ({
  has_admin: true,
  has_tenant_admin: true,
  has_tenant: true,
  entry: 'login',
  next_step: 'login'
})

const firstDeviceWorkbenchViewRef = ref<HomeFirstDeviceWorkbenchViewExpose | null>(null)
const layoutFetched = ref(false)
const hasCompatHomeConfig = ref(false)
const compatHomeConfigCount = ref(0)
const isError = ref<boolean>(false)
const active = ref<boolean>(true)
const showSysAdminSetup = ref(false)
const authStore = useAuthStore()
const route = useRoute()
const isSysAdmin = computed(() => isSysAdminUser(authStore.userInfo))
const isFirstDeviceOnboardingRoute = computed(() => {
  const routeHash = String(route.hash || '').replace(/^#/, '')
  return route.query.onboarding === 'first-device' || routeHash.startsWith('first-device')
})

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
const isNativeHomeProvider = import.meta.env.VITE_ENABLE_THINGSVIS_COMPAT !== 'Y'
const homeVisualizationPath = computed(() =>
  isNativeHomeProvider ? '/visualization/native-boards' : '/visualization/thingsvis'
)
const showHomeResolvingGate = computed(
  () => isSysAdmin.value && isHomeResolving.value && !isFirstDeviceOnboardingRoute.value
)
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
const sysAdminSetupAction = computed(() =>
  isNativeHomeProvider ? $t('custom.home.actions.openNativeBoards') : $t('custom.home.actions.openThingsVis')
)
const deploymentHealthLoading = ref(false)
const deploymentHealth = ref<DeploymentHealthReport | null>(null)
const deploymentHealthRows = computed(() => normalizeDeploymentHealth(deploymentHealth.value, $t))
const deploymentHealthOk = computed(
  () =>
    deploymentHealth.value?.status === 'ok' &&
    deploymentHealthRows.value.length > 0 &&
    deploymentHealthRows.value.every((row) => row.ok)
)
const automationGuideLoading = ref(false)
const hasSceneAutomation = ref(false)
const firstRunCreateLoading = ref(false)
const firstRunProtocol = ref<HomeFirstRunProtocol>('MQTT')
const firstRunCreateResult = ref<HomeFirstRunQuickCreateResult | null>(null)
const firstRunCreateTenantRequired = ref(false)
const tenantSetupState = ref<TenantSetupState>(defaultTenantSetupState())
const homeFirstRunTenantId = computed(() => getHomeFirstRunTenantId(authStore.userInfo))
const nativeHomeTenantId = computed(() => homeFirstRunTenantId.value || readNativeBoardTenantContext(authStore.userInfo))
const hasHomeFirstRunTenantContext = computed(() => Boolean(homeFirstRunTenantId.value))
const hasNativeHomeTenantContext = computed(() => Boolean(nativeHomeTenantId.value))
const tenantSetupNextStep = computed<TenantSetupNextStep>(() => {
  if (!tenantSetupState.value?.has_admin) return 'create_super_admin'
  return tenantSetupState.value?.next_step || 'login'
})
const homeSetupGuideStep = computed(() => {
  if (tenantSetupNextStep.value === 'create_super_admin') {
    return {
      id: 'setup',
      title: $t('custom.home.setup.createSuperAdmin.title'),
      description: $t('custom.home.setup.createSuperAdmin.description'),
      route: '/login/register-super-admin',
      action: $t('custom.home.setup.createSuperAdmin.action')
    }
  }

  if (
    tenantSetupNextStep.value === 'create_tenant_admin' ||
    !tenantSetupState.value?.has_tenant_admin ||
    !tenantSetupState.value?.has_tenant
  ) {
    return {
      id: 'setup',
      title: $t('custom.home.setup.createTenantAdmin.title'),
      description: $t('custom.home.setup.createTenantAdmin.description'),
      route: '/management/user?setup=tenant-admin',
      action: $t('custom.home.setup.createTenantAdmin.action')
    }
  }

  if (!homeSetupReady.value) {
    return {
      id: 'setup',
      title: $t('custom.home.setup.loginAsTenantAdmin.title'),
      description: $t('custom.home.setup.loginAsTenantAdmin.description'),
      route: '/login',
      action: $t('custom.home.setup.loginAsTenantAdmin.action')
    }
  }

  return {
    id: 'setup',
    title: $t('custom.home.setup.ready.title'),
    description: $t('custom.home.setup.ready.description'),
    route: '/management/user',
    action: $t('custom.home.setup.ready.action')
  }
})
const homeSetupReady = computed(() => tenantSetupNextStep.value === 'login' && hasHomeFirstRunTenantContext.value)
const firstRunTenantBlocked = computed(() => firstRunCreateTenantRequired.value || !homeSetupReady.value)
const firstDeviceWorkbench = useHomeFirstDeviceWorkbench({
  deploymentHealthy: deploymentHealthOk,
  deploymentHealthRows,
  onTenantRequired: () => {
    firstRunCreateTenantRequired.value = true
  }
})
const firstDeviceLoading = firstDeviceWorkbench.loading
const firstDevice = firstDeviceWorkbench.device
const firstDeviceTelemetry = firstDeviceWorkbench.telemetry
const firstDeviceSimulation = firstDeviceWorkbench.simulation
const firstDeviceAccessGuide = firstDeviceWorkbench.accessGuide
const firstDeviceActionLoading = firstDeviceWorkbench.actionLoading
const firstDeviceTestResult = firstDeviceWorkbench.testResult
const firstDeviceBrowserTest = firstDeviceWorkbench.browserTest
const firstDevicePublishCommand = firstDeviceWorkbench.publishCommand
const firstDeviceOnboardingGuard = firstDeviceWorkbench.onboardingGuard
const firstDeviceReadyProof = firstDeviceWorkbench.readyProof
const firstDeviceChart = firstDeviceWorkbench.firstChart
const buildFirstDeviceSupportSummary = firstDeviceWorkbench.buildSupportSummary
const openFirstDeviceAccessGuide = firstDeviceWorkbench.openReadyCheck
const openFirstDeviceFullGuide = firstDeviceWorkbench.openFullGuide
const copyFirstDevicePublishCommand = firstDeviceWorkbench.copyPublishCommand
const simulateFirstDeviceTelemetry = firstDeviceWorkbench.simulateTelemetry
const firstDeviceWorkbenchLoaded = ref(false)
let firstDeviceWorkbenchLoadPromise: Promise<void> | null = null
const skipNextDefaultFirstDeviceFocus = ref(false)
const FIRST_DEVICE_FOCUS_REF_RETRY_LIMIT = 40
const FIRST_DEVICE_FOCUS_REF_RETRY_DELAY_MS = 50
const firstRunGuideState = ref<HomeFirstRunGuideState | null>(
  loadHomeFirstRunGuideState(typeof window === 'undefined' ? null : window.localStorage)
)
const homeCustomerGuideProgress = computed(() =>
  buildHomeCustomerGuideProgress({
    setupReady: homeSetupReady.value,
    setupStep: homeSetupGuideStep.value as Partial<HomeCustomerGuideStep>,
    hasDevice: Boolean(firstDevice.value),
    hasTemplate: Boolean(firstDevice.value?.configName),
    deviceOnline: Boolean(firstDevice.value?.online),
    hasTelemetry: firstDeviceTelemetry.value.length > 0,
    hasFirstChart: firstDeviceChart.value.ready,
    hasAutomation: hasSceneAutomation.value,
    hasDashboard: isCompleteThingsVisDashboard(thingsVisHome.value),
    deploymentHealthy: deploymentHealthOk.value
  })
)
const homeCustomerGuideSummary = computed(() => buildHomeCustomerGuideSummary(homeCustomerGuideProgress.value))
const currentHomeSetupGuideStep = computed(
  () => homeCustomerGuideProgress.value.find((step) => step.id === 'setup') ?? null
)
const homeFirstRunResumeText = computed(() => {
  const state = firstRunGuideState.value
  if (!state?.lastTitle) return ''
  if (state.quickCreateDeviceName) {
    return $t('custom.home.resume.afterCreate', { deviceName: state.quickCreateDeviceName, lastTitle: state.lastTitle })
  }
  return $t('custom.home.resume.continue', {
    lastTitle: state.lastTitle,
    lastAction: state.lastAction || $t('custom.home.resume.nextAction')
  })
})
const shouldShowHomeSecondarySections = computed(
  () => !isFirstDeviceOnboardingRoute.value || Boolean(firstDeviceReadyProof.value.ready)
)

function isCompleteThingsVisDashboard(dashboard?: VisualizationHomeDashboard | null): boolean {
  if (!dashboard || !dashboard.canvasConfig || typeof dashboard.canvasConfig !== 'object') return false

  if (dashboard.providerId === NATIVE_BOARD_PROVIDER_ID) {
    return normalizeLocalDashboard(dashboard.rendererData).ok
  }

  if (!Array.isArray(dashboard.nodes) || dashboard.nodes.length === 0) return false
  if (!Array.isArray(dashboard.dataSources) || dashboard.dataSources.length === 0) return false
  return true
}

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
) : Promise<VisualizationHomeLoadResult & { timedOut?: boolean }> => {
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
  if (!isNativeHomeProvider && cachedHome?.state === 'thingsvis' && isCompleteThingsVisDashboard(cachedHome.dashboard)) {
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
    scheduleIdleHomeTask(() => {
      void refreshThingsVisHomeDashboardInBackground()
    }, 300)
    return
  }

  if (!isSysAdmin.value) {
    await loadCompatHomeState()
    scheduleIdleHomeTask(() => {
      void refreshThingsVisHomeDashboardInBackground()
    }, 300)
    return
  }

  const thingsVisProbe = await probeVisualizationHomeDashboard(
    nativeHomeTenantId.value ? { tenantId: nativeHomeTenantId.value } : undefined
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
      : await loadThingsVisHomeDashboardWithBudget(null, THINGSVIS_HOME_DASHBOARD_BUDGET_MS, nativeHomeTenantId.value)
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

const saveFirstRunGuideStep = (
  step: HomeCustomerGuideProgressStep,
  quickCreateDeviceName = firstRunCreateResult.value?.deviceName || ''
) => {
  firstRunGuideState.value = saveHomeFirstRunGuideState(typeof window === 'undefined' ? null : window.localStorage, {
    lastStep: step.id,
    lastTitle: step.title,
    lastAction: step.action,
    lastRoute: step.route,
    quickCreateDeviceName
  })
}

const shouldLoadFirstRunWorkbenchData = () =>
  isFirstDeviceOnboardingRoute.value || (!showSysAdminSetup.value && !isError.value)

const runFirstDeviceWorkbenchRefresh = async (force = false) => {
  if (!shouldLoadFirstRunWorkbenchData()) return
  if (!force && firstDeviceWorkbenchLoaded.value) return
  if (firstDeviceWorkbenchLoadPromise) {
    await firstDeviceWorkbenchLoadPromise
    if (!force) return
  }

  const refreshPromise = firstDeviceWorkbench.refresh().then(() => {
    firstDeviceWorkbenchLoaded.value = true
  })
  firstDeviceWorkbenchLoadPromise = refreshPromise
  try {
    await refreshPromise
  } finally {
    if (firstDeviceWorkbenchLoadPromise === refreshPromise) {
      firstDeviceWorkbenchLoadPromise = null
    }
  }
}

const ensureFirstDeviceWorkbenchLoaded = () => runFirstDeviceWorkbenchRefresh(false)
const refreshFirstDeviceWorkbench = () => runFirstDeviceWorkbenchRefresh(true)

const waitForFirstDeviceWorkbenchView = async () => {
  if (firstDeviceWorkbenchViewRef.value) return firstDeviceWorkbenchViewRef.value

  for (let attempt = 0; attempt < FIRST_DEVICE_FOCUS_REF_RETRY_LIMIT; attempt += 1) {
    await nextTick()
    if (firstDeviceWorkbenchViewRef.value) return firstDeviceWorkbenchViewRef.value
    await new Promise((resolve) => window.setTimeout(resolve, FIRST_DEVICE_FOCUS_REF_RETRY_DELAY_MS))
    if (firstDeviceWorkbenchViewRef.value) return firstDeviceWorkbenchViewRef.value
  }

  return firstDeviceWorkbenchViewRef.value
}

const focusHomeWorkbenchSection = async (key: string) => {
  await ensureFirstDeviceWorkbenchLoaded()
  const workbenchView = await waitForFirstDeviceWorkbenchView()
  await (workbenchView?.focusSection(key) || Promise.resolve())
}

const consumeFirstDeviceFocusQuery = () => {
  const routeHash = String(route.hash || '').replace(/^#/, '')
  if (route.query.onboarding !== 'first-device' && !routeHash.startsWith('first-device')) return
  const queryFocus = Array.isArray(route.query.focus) ? route.query.focus[0] : route.query.focus
  if (!queryFocus && !routeHash && skipNextDefaultFirstDeviceFocus.value) {
    skipNextDefaultFirstDeviceFocus.value = false
    return
  }
  const focus = queryFocus || routeHash || 'quickstart'
  const focusMap: Record<string, string> = {
    deployment: 'deployment',
    health: 'deployment',
    device: 'device',
    identity: 'device',
    connection: 'connection',
    command: 'connection',
    test: 'test',
    sample: 'test',
    tester: 'test',
    browser_test: 'test',
    online: 'proof',
    telemetry: 'chart',
    chart: 'chart',
    proof: 'proof',
    support: 'support',
    quickstart: 'quickstart',
    'first-device-proof': 'proof',
    'first-device-chart': 'chart'
  }
  const section = focusMap[String(focus || 'quickstart')] || 'quickstart'

  window.setTimeout(() => {
    void focusHomeWorkbenchSection(section)
  }, 0)
  const shouldClearFocus = Boolean(queryFocus || routeHash)
  if (!shouldClearFocus) return
  skipNextDefaultFirstDeviceFocus.value = true
  const { focus: _focus, ...query } = route.query
  router.replace({
    path: route.path,
    query,
    hash: ''
  })
}

const buildFirstAutomationStarterQuery = () => {
  const query: Record<string, string> = {
    backType: 'automation',
    onboarding: 'first-device',
    starter: 'first-telemetry-rule'
  }
  const device = firstDevice.value
  const telemetry = firstDeviceTelemetry.value[0]

  if (device?.id) query.device_id = device.id
  if (device?.configId) query.device_config_id = device.configId
  if (device?.name) query.first_device_name = device.name
  if (device?.number) query.first_device_number = device.number
  if (telemetry?.key) query.telemetry_key = telemetry.key
  if (telemetry?.value !== undefined && telemetry?.value !== null) query.telemetry_value = String(telemetry.value)
  if (telemetry?.ts) query.telemetry_at = telemetry.ts

  return query
}

const openHomeGuideStep = async (step: HomeCustomerGuideProgressStep) => {
  saveFirstRunGuideStep(step)
  if (step.id === 'setup') {
    firstRunCreateTenantRequired.value = step.status !== 'done'
    router.push(step.route)
    return
  }
  if (step.id === 'deployment') {
    void refreshDeploymentHealth().finally(() => {
      void focusHomeWorkbenchSection('deployment')
    })
    return
  }
  if (step.id === 'device') {
    await ensureFirstDeviceWorkbenchLoaded()
    if (!firstDevice.value) {
      void focusHomeWorkbenchSection('device')
      if (homeSetupReady.value && deploymentHealthOk.value) {
        void createFirstRunFirstDevice()
      }
      return
    }
    if (!firstDevice.value.online) {
      void focusHomeWorkbenchSection('test')
      return
    }
    void focusHomeWorkbenchSection('proof')
    return
  }
  if (step.id === 'telemetry') {
    await ensureFirstDeviceWorkbenchLoaded()
    if (!firstDevice.value) {
      void focusHomeWorkbenchSection('device')
      return
    }
    if (!firstDeviceTelemetry.value.length) {
      void focusHomeWorkbenchSection('test')
      return
    }
    if (!firstDeviceChart.value.ready) {
      void focusHomeWorkbenchSection('chart')
      return
    }
    void focusHomeWorkbenchSection('proof')
    return
  }
  if (step.id === 'automation') {
    await ensureFirstDeviceWorkbenchLoaded()
    router.push(
      hasSceneAutomation.value
        ? {
            path: '/automation/scene-linkage',
            query: buildFirstAutomationStarterQuery()
          }
        : {
            path: '/automation/linkage-edit',
            query: buildFirstAutomationStarterQuery()
          }
    )
    return
  }
  if (step.id === 'dashboard') {
    router.push({
      path: '/visualization/thingsvis',
      query: { onboarding: 'first-device' }
    })
    return
  }
  router.push(step.route)
}

const createFirstRunFirstDevice = async () => {
  if (firstRunCreateLoading.value) return
  if (!homeSetupReady.value) {
    firstRunCreateTenantRequired.value = true
    window.$message?.warning($t('custom.home.firstRun.setupRequired', { title: homeSetupGuideStep.value.title }))
    return
  }
  firstRunCreateLoading.value = true
  firstRunCreateResult.value = null
  firstRunCreateTenantRequired.value = false
  try {
    const result = await createHomeFirstRunFirstDevice({ userInfo: authStore.userInfo, protocol: firstRunProtocol.value })
    firstRunCreateResult.value = result
    const deviceStep = homeCustomerGuideProgress.value.find((step) => step.id === 'device')
    if (deviceStep) saveFirstRunGuideStep(deviceStep, result.deviceName)
    window.$message?.success($t('custom.home.firstRun.deviceCreated', { protocol: result.protocol }))
    await refreshFirstDeviceWorkbench()
    await focusHomeWorkbenchSection('test')
  } catch (error: any) {
    window.$message?.error(error?.message || $t('custom.home.firstRun.deviceCreateFailed'))
  } finally {
    firstRunCreateLoading.value = false
  }
}

const runFirstDeviceQuickstartAction = (action: string) =>
  action === 'health'
    ? refreshDeploymentHealth()
    : ensureFirstDeviceWorkbenchLoaded().then(() =>
        firstDeviceWorkbench.runQuickstartAction(action, createFirstRunFirstDevice)
      )

const openFirstDeviceAccessGuideAfterLoad = () =>
  ensureFirstDeviceWorkbenchLoaded().then(() => {
    openFirstDeviceAccessGuide()
  })

const openFirstDeviceFullGuideAfterLoad = () =>
  ensureFirstDeviceWorkbenchLoaded().then(() => {
    openFirstDeviceFullGuide()
  })

const copyFirstDevicePublishCommandAfterLoad = () =>
  ensureFirstDeviceWorkbenchLoaded().then(() => copyFirstDevicePublishCommand())

const simulateFirstDeviceTelemetryAfterLoad = () =>
  ensureFirstDeviceWorkbenchLoaded().then(() => simulateFirstDeviceTelemetry())

let tenantSetupGuideStateRefreshPromise: Promise<void> | null = null
let deploymentHealthRefreshPromise: Promise<void> | null = null
let automationGuideRefreshPromise: Promise<void> | null = null

const refreshDeploymentHealth = () => {
  if (deploymentHealthRefreshPromise) return deploymentHealthRefreshPromise

  deploymentHealthLoading.value = true
  deploymentHealthRefreshPromise = fetchDeploymentHealthReport($t)
    .then((report) => {
      deploymentHealth.value = report
    })
    .finally(() => {
      deploymentHealthLoading.value = false
      deploymentHealthRefreshPromise = null
    })

  return deploymentHealthRefreshPromise
}

const refreshTenantSetupGuideState = () => {
  if (tenantSetupGuideStateRefreshPromise) return tenantSetupGuideStateRefreshPromise

  tenantSetupGuideStateRefreshPromise = fetchTenantSetupState()
    .then((response) => {
      tenantSetupState.value = response.data ?? defaultTenantSetupState()
    })
    .catch(() => {
      tenantSetupState.value = defaultTenantSetupState()
    })
    .finally(() => {
      tenantSetupGuideStateRefreshPromise = null
    })

  return tenantSetupGuideStateRefreshPromise
}

const unwrapListResponse = (response: any): any[] => {
  const data = response?.data?.data ?? response?.data ?? response ?? {}
  const list = data?.list ?? data?.records ?? data?.data ?? []
  return Array.isArray(list) ? list : []
}

const refreshAutomationGuideState = () => {
  if (automationGuideRefreshPromise) return automationGuideRefreshPromise

  automationGuideLoading.value = true
  automationGuideRefreshPromise = sceneAutomationsGet({ page: 1, page_size: 1 })
    .then((response) => {
      hasSceneAutomation.value = unwrapListResponse(response).length > 0
    })
    .catch(() => {
      hasSceneAutomation.value = false
    })
    .finally(() => {
      automationGuideLoading.value = false
      automationGuideRefreshPromise = null
    })

  return automationGuideRefreshPromise
}

const scheduleIdleHomeTask = (task: () => void, fallbackDelay = 100) => {
  if (typeof window === 'undefined') {
    task()
    return
  }
  if ('requestIdleCallback' in window) {
    window.requestIdleCallback(task, { timeout: 2000 })
    return
  }
  ;(window as Window).setTimeout(task, fallbackDelay)
}

const homeGuideRefreshCoordinator = createHomeGuideRefreshCoordinator({
  schedule: scheduleIdleHomeTask,
  refreshTenantSetup: refreshTenantSetupGuideState,
  refreshDeploymentHealth,
  refreshFirstDeviceWorkbench,
  refreshAutomation: refreshAutomationGuideState,
  shouldRefreshAutomation: () => shouldShowHomeSecondarySections.value
})

const refreshHomeGuideProgress = () => {
  homeGuideRefreshCoordinator.refreshFromUser()
}

const refreshInitialHomeGuideProgress = () => {
  if (!shouldLoadFirstRunWorkbenchData()) return
  homeGuideRefreshCoordinator.refreshOnInitialLoad()
}

const scheduleInitialHomeGuideProgress = (retryCount = 0) => {
  scheduleIdleHomeTask(() => {
    if (
      !isFirstDeviceOnboardingRoute.value &&
      !layoutFetched.value &&
      !showSysAdminSetup.value &&
      !isError.value &&
      retryCount < 8
    ) {
      window.setTimeout(() => scheduleInitialHomeGuideProgress(retryCount + 1), 250)
      return
    }
    refreshInitialHomeGuideProgress()
  }, 100)
}

const mountHomeThingsVisFrame = () => {
  if (!useThingsVis.value || !thingsVisHome.value) return
  homeThingsVisFrameMount.mountNow()
}

const observeHomeThingsVisFrame = async () => {
  if (!shouldShowHomeSecondarySections.value || !useThingsVis.value || !thingsVisHome.value) {
    homeThingsVisFrameMount.reset()
    return
  }

  if (shouldMountHomeThingsVisFrame.value) return
  await homeThingsVisFrameMount.observe()
}
onMounted(() => {
  consumeFirstDeviceFocusQuery()
  if (shouldShowHomeSecondarySections.value) {
    void getLayout()
  }
  scheduleInitialHomeGuideProgress()
  void observeHomeThingsVisFrame()
})

watch(
  () => `${route.query.onboarding || ''}|${route.query.focus || ''}|${route.hash || ''}`,
  () => {
    consumeFirstDeviceFocusQuery()
  }
)

watch(homeSetupReady, (ready) => {
  if (ready) {
    firstRunCreateTenantRequired.value = false
  }
})

watch(
  () => `${useThingsVis.value}|${thingsVisHome.value?.id || ''}`,
  () => {
    void observeHomeThingsVisFrame()
  }
)

watch(shouldShowHomeSecondarySections, (shouldShow) => {
  if (!shouldShow) {
    homeThingsVisFrameMount.reset()
    return
  }

  void getLayout()
  void refreshAutomationGuideState()
  void observeHomeThingsVisFrame()
})
</script>

<template>
  <div v-if="showHomeResolvingGate" class="h-full w-full flex-center px-16px">
    <div
      class="w-full max-w-520px rounded-8px border border-gray-200 bg-white px-24px py-22px shadow-sm dark:border-gray-700 dark:bg-#18181c"
    >
      <div class="flex items-start gap-14px">
        <n-spin size="small" class="mt-2px" />
        <div class="min-w-0 flex-1">
          <div class="text-16px font-600 leading-24px">{{ $t('custom.home.resolvingTitle') }}</div>
          <div class="mt-6px text-14px leading-22px text-gray-500">{{ homeResolvingDescription }}</div>
        </div>
      </div>
    </div>
  </div>

  <div v-else-if="isError && !useThingsVis && isSysAdmin && !isFirstDeviceOnboardingRoute" class="h-full w-full flex-center">
    <n-result status="418" :title="$t('custom.home.title')" :description="$t('custom.home.description')">
      <template #footer>
        <n-button
          type="primary"
          :disabled="active"
          @click="
            () => {
              router.go(0)
            }
          "
        >
          <n-countdown
            v-if="active"
            :duration="60000"
            :render="(props) => props.seconds + 's'"
            :active="active"
            @finish="active = false"
          />
          {{ active ? '' : $t('custom.home.refresh') }}
        </n-button>
      </template>
    </n-result>
  </div>

  <div v-else-if="showSysAdminSetup && !isFirstDeviceOnboardingRoute" class="h-full w-full flex-center">
    <n-result
      status="info"
      :title="sysAdminSetupTitle"
      :description="sysAdminSetupDescription"
    >
      <template #footer>
        <div class="flex items-center gap-3">
          <n-button
            type="primary"
            @click="
              () => {
                router.push(homeVisualizationPath)
              }
            "
          >
            {{ sysAdminSetupAction }}
          </n-button>
          <n-button
            @click="
              () => {
                router.go(0)
              }
            "
          >
            {{ $t('custom.home.actions.reload') }}
          </n-button>
        </div>
      </template>
    </n-result>
  </div>

  <div v-else class="home-workspace h-full w-full px-16px py-16px">
    <HomeFirstDeviceWorkbenchView
      ref="firstDeviceWorkbenchViewRef"
      :home-customer-guide-summary="homeCustomerGuideSummary"
      :home-first-run-resume-text="homeFirstRunResumeText"
      :home-customer-guide-progress="homeCustomerGuideProgress"
      :first-device-focus-mode="isFirstDeviceOnboardingRoute"
      :first-device-workbench-loaded="firstDeviceWorkbenchLoaded"
      :first-device-ready-proof="firstDeviceReadyProof"
      :first-device="firstDevice"
      :first-device-loading="firstDeviceLoading"
      :deployment-health-loading="deploymentHealthLoading"
      :automation-guide-loading="automationGuideLoading"
      :first-run-create-loading="firstRunCreateLoading"
      :first-run-protocol="firstRunProtocol"
      :deployment-health-ok="deploymentHealthOk"
      :first-run-create-result="firstRunCreateResult"
      :first-run-create-tenant-required="firstRunTenantBlocked"
      :first-run-setup-blocker-step="currentHomeSetupGuideStep"
      :first-device-access-guide="firstDeviceAccessGuide"
      :first-device-simulation="firstDeviceSimulation"
      :first-device-publish-command="firstDevicePublishCommand"
      :first-device-onboarding-guard="firstDeviceOnboardingGuard"
      :first-device-action-loading="firstDeviceActionLoading"
      :first-device-test-result="firstDeviceTestResult"
      :first-device-browser-test="firstDeviceBrowserTest"
      :first-device-chart="firstDeviceChart"
      :deployment-health-rows="deploymentHealthRows"
      :build-first-device-support-summary="buildFirstDeviceSupportSummary"
      @open-home-guide-step="openHomeGuideStep"
      @refresh-home-guide-progress="refreshHomeGuideProgress"
      @refresh-first-device-workbench="refreshFirstDeviceWorkbench"
      @update-first-run-protocol="firstRunProtocol = $event"
      @create-first-run-first-device="createFirstRunFirstDevice"
      @open-manual-device-add="router.push('/device/manage?onboarding=first-device&add=manual')"
      @open-things-model="router.push('/device/thingsmodel')"
      @copy-first-device-publish-command="copyFirstDevicePublishCommandAfterLoad"
      @simulate-first-device-telemetry="simulateFirstDeviceTelemetryAfterLoad"
      @open-first-device-full-guide="openFirstDeviceFullGuideAfterLoad"
      @open-first-device-access-guide="openFirstDeviceAccessGuideAfterLoad"
      @run-first-device-quickstart-action="runFirstDeviceQuickstartAction"
      @refresh-deployment-health="refreshDeploymentHealth"
    />

    <HomeSecondaryPanel
      v-if="shouldShowHomeSecondarySections"
      :is-home-resolving="isHomeResolving"
      :show-home-resolving-gate="showHomeResolvingGate"
      :home-resolving-description="homeResolvingDescription"
      :is-error="isError"
      :use-things-vis="useThingsVis"
      :things-vis-home="thingsVisHome"
      :things-vis-section-ref="(homeThingsVisSectionRef as any)"
      :should-mount-home-things-vis-frame="shouldMountHomeThingsVisFrame"
      :show-compat-home-notice="showCompatHomeNotice"
      :compat-home-config-count="compatHomeConfigCount"
      @reload="router.go(0)"
      @open-things-vis="router.push(homeVisualizationPath)"
      @mount-home-things-vis-frame="mountHomeThingsVisFrame"
      @continue-first-device="router.push('/home?onboarding=first-device')"
      @open-device-management="router.push('/device/manage')"
      @open-rdi-dashboard="router.push('/dashboard/rdi-overview')"
      @open-rdi-alarm-overview="router.push('/alarm/rdi-overview')"
      @open-alarm-center="router.push('/alarm')"
      @open-system-settings="router.push('/management/setting')"
    />
  </div>
</template>

<style scoped>
.home-workspace {
  overflow-y: auto;
  overflow-x: hidden;
  scrollbar-width: thin;
  scrollbar-color: rgba(0, 0, 0, 0.5) transparent;
}
</style>
