// 文件用途：承载租户初始化状态拉取与首页安装引导步骤推导。
// 核心逻辑：带在途去重地获取租户初始化状态，推导下一步动作、就绪判定与引导卡片内容。
// 关键注意事项：引导步骤的文案与路由是页面对外契约，调整需同步客户引导进度与测试。
import { computed, ref, type ComputedRef } from 'vue'
import { fetchTenantSetupState } from '@/service/api/auth'
import { $t } from '@/locales'

export type TenantSetupNextStep = 'create_super_admin' | 'create_tenant_admin' | 'login'

export type TenantSetupState = {
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

type UseHomeTenantSetupGuideOptions = {
  hasFirstRunTenantContext: ComputedRef<boolean>
}

export function useHomeTenantSetupGuide(options: UseHomeTenantSetupGuideOptions) {
  let tenantSetupGuideStateRefreshPromise: Promise<void> | null = null

  const tenantSetupState = ref<TenantSetupState>(defaultTenantSetupState())

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

  const tenantSetupNextStep = computed<TenantSetupNextStep>(() => {
    if (!tenantSetupState.value?.has_admin) return 'create_super_admin'
    return tenantSetupState.value?.next_step || 'login'
  })

  const homeSetupReady = computed(() => tenantSetupNextStep.value === 'login' && options.hasFirstRunTenantContext.value)

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

  return {
    tenantSetupState,
    refreshTenantSetupGuideState,
    tenantSetupNextStep,
    homeSetupReady,
    homeSetupGuideStep
  }
}
