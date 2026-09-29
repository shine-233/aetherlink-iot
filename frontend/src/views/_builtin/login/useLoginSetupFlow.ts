/**
 * 文件用途：登录页首次安装/市场注册流程状态机。
 * 核心逻辑：拉取 tenant setup state → 未初始化且未从市场返回时跳转市场注册页；
 *   可本地初始化（无市场地址）或已从市场返回时渲染 register-super-admin 模块。
 * 关键注意事项：
 *   - 市场地址只接受 http(s) 且非本机地址，否则回退 VITE_MARKET_URL（可为空，避免误跳示例域名）。
 *   - setup state 请求失败时按"已初始化"降级，保证登录入口始终可用。
 */
import { computed, ref } from 'vue'
import { fetchTenantSetupState } from '@/service/api/auth'
import { createLogger } from '@/utils/logger'

export type TenantSetupNextStep = 'create_super_admin' | 'create_tenant_admin' | 'login'

export interface TenantSetupState {
  has_admin: boolean
  has_tenant_admin?: boolean
  has_tenant?: boolean
  entry: 'login' | 'register'
  next_step?: TenantSetupNextStep
  market_base_url?: string
  market_register_url?: string
}

export const defaultTenantSetupState = (): TenantSetupState => ({
  has_admin: true,
  has_tenant_admin: true,
  has_tenant: true,
  entry: 'login',
  next_step: 'login'
})

const LOCAL_HOST_PATTERN = /localhost|127\.0\.0\.1|0\.0\.0\.0/

/** Normalizes a backend-supplied market URL, falling back when it is missing, non-http(s) or local. */
export function normalizeMarketUrl(baseUrl: string | undefined, fallback: string) {
  const url = baseUrl?.trim()
  if (!url || !/^https?:\/\//i.test(url) || LOCAL_HOST_PATTERN.test(url)) return fallback
  return url
}

/** Builds the market /register URL with callback + return_to pointing at `currentHref`. */
export function buildMarketRegisterUrl(base: string, currentHref: string) {
  if (!base) throw new Error('市场注册地址未配置')
  const url = base.endsWith('/register') ? new URL(base) : new URL('/register', base)
  url.searchParams.set('callback', currentHref)
  url.searchParams.set('return_to', currentHref)
  return url.toString()
}

export function resolveSetupNextStep(state: TenantSetupState | null): TenantSetupNextStep {
  if (!state?.has_admin) return 'create_super_admin'
  return state.next_step || 'login'
}

/** i18n key pair for the setup guide banner. */
export function setupGuideKeys(step: TenantSetupNextStep) {
  if (step === 'create_super_admin') {
    return { title: 'custom.login.setup.createSuperAdminTitle', description: 'custom.login.setup.createSuperAdminDesc' }
  }
  if (step === 'create_tenant_admin') {
    return { title: 'custom.login.setup.createTenantAdminTitle', description: 'custom.login.setup.createTenantAdminDesc' }
  }
  return { title: 'custom.login.setup.welcomeBackTitle', description: 'custom.login.setup.welcomeBackDesc' }
}

export interface UseLoginSetupFlowOptions {
  searchParams: URLSearchParams
  fallbackMarketUrl: string
  /** Navigation side effect; injectable for tests. */
  navigate?: (url: string) => void
  currentHref?: () => string
}

const logger = createLogger('LoginPage')

export function useLoginSetupFlow(options: UseLoginSetupFlowOptions) {
  const { searchParams, fallbackMarketUrl } = options
  const navigate = options.navigate ?? ((url: string) => window.location.replace(url))
  const currentHref = options.currentHref ?? (() => window.location.href)

  const setupState = ref<TenantSetupState | null>(null)
  const loading = ref(true)
  const redirectingToMarket = ref(false)

  const returnedFromMarket = computed(
    () => searchParams.get('market_registered') === '1' || searchParams.get('market_logged_in') === '1'
  )
  const marketEmail = computed(() => searchParams.get('market_email')?.trim() || '')
  const marketSource = computed(() => searchParams.get('market_source')?.trim() || 'horizon')
  const marketRegisterUrl = computed(() =>
    normalizeMarketUrl(setupState.value?.market_register_url || setupState.value?.market_base_url, fallbackMarketUrl)
  )
  const shouldUseLocalSuperAdminInit = computed(
    () => !!setupState.value && !setupState.value.has_admin && !returnedFromMarket.value && !marketRegisterUrl.value
  )
  /** True when the super-admin init form should be shown instead of the regular login. */
  const needsSuperAdminInit = computed(
    () =>
      !!setupState.value &&
      !setupState.value.has_admin &&
      (returnedFromMarket.value || shouldUseLocalSuperAdminInit.value)
  )
  const setupNextStep = computed(() => resolveSetupNextStep(setupState.value))

  function redirectToMarketRegister() {
    if (redirectingToMarket.value) return
    try {
      redirectingToMarket.value = true
      navigate(buildMarketRegisterUrl(marketRegisterUrl.value, currentHref()))
    } catch (error) {
      logger.error('[LoginPage] 跳转市场注册页失败:', error instanceof Error ? error.message : error)
      redirectingToMarket.value = false
      loading.value = false
    }
  }

  async function loadSetupState() {
    try {
      const res = await fetchTenantSetupState()
      setupState.value = (res.data as TenantSetupState | null | undefined) ?? defaultTenantSetupState()
    } catch (error) {
      logger.error('[LoginPage] 获取安装状态失败，使用默认值:', error instanceof Error ? error.message : error)
      setupState.value = defaultTenantSetupState()
    } finally {
      if (setupState.value && !setupState.value.has_admin && !needsSuperAdminInit.value) {
        redirectToMarketRegister()
      } else {
        loading.value = false
      }
    }
  }

  return {
    setupState,
    loading,
    redirectingToMarket,
    returnedFromMarket,
    marketEmail,
    marketSource,
    marketRegisterUrl,
    shouldUseLocalSuperAdminInit,
    needsSuperAdminInit,
    setupNextStep,
    loadSetupState,
    redirectToMarketRegister
  }
}
