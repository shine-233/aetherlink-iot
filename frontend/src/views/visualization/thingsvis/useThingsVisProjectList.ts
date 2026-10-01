// 文件用途：ThingsVis 总览页的项目列表状态、provider 解析与导航。
// 核心逻辑：按路由 query 解析可视化 provider，拉取项目列表并做关键字过滤，
// 同时暴露进入项目仪表盘列表的导航入口。
// 关键注意事项：provider 可能为外部禁用态，此时不应发起拉取；
// 路由 query（provider / projectId / onboarding）决定语义，改动要同步核对路由参数。
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  getDefaultVisualizationProviderFacade,
  NATIVE_BOARD_PROVIDER_ID,
  type VisualizationProject
} from '@/service/visualization-provider/index'
import { resolveVisualizationProviderId } from '@/service/visualization-provider/composition'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'

export function useThingsVisProjectList() {
  const { routerPushByKey } = useRouterPush()
  const route = useRoute()

  const providerId = computed(() =>
    resolveVisualizationProviderId({
      provider: route.query.provider,
      projectId: route.query.projectId
    })
  )
  const provider = getDefaultVisualizationProviderFacade({ providerId: providerId.value })
  const providerError = computed(() => provider.selectionError)
  const projectCapabilities = computed(
    () =>
      provider.capabilities?.projects ?? {
        list: false,
        create: false,
        update: false,
        delete: false
      }
  )
  const providerBlockedMessage = computed(() => {
    if (providerError.value?.code === 'external-blocked') {
      return $t('rdi.thingsvis.externalProviderDisabledDescription')
    }

    return providerError.value?.message || $t('rdi.thingsvis.loadProjectsFailed')
  })
  const isNativeProvider = computed(() => providerId.value === NATIVE_BOARD_PROVIDER_ID)
  const isFirstDeviceOnboarding = computed(() => route.query.onboarding === 'first-device')
  const onboardingDashboardQuery = computed((): Record<string, string> =>
    isFirstDeviceOnboarding.value
      ? { onboarding: 'first-device', ...(isNativeProvider.value ? { provider: 'native' } : {}) }
      : isNativeProvider.value
        ? { provider: 'native' }
        : {}
  )

  // State
  const loading = ref(false)
  const allProjects = ref<VisualizationProject[]>([])
  const searchKeyword = ref('')
  const projects = computed(() => {
    const keyword = searchKeyword.value.trim().toLowerCase()
    if (!keyword) return allProjects.value

    return allProjects.value.filter((item) => item.name.toLowerCase().includes(keyword))
  })

  /** Fetch project list */
  const fetchProjects = async (onError?: () => void) => {
    loading.value = true
    try {
      if (providerError.value) return

      const result = await provider.execute((current) => current.listProjects({ page: 1, limit: 100 }))
      if (result.ok) {
        allProjects.value = result.data.items
      } else {
        onError?.()
      }
    } finally {
      loading.value = false
    }
  }

  /** Enter project dashboard list */
  const enterProject = (projectId: string) => {
    routerPushByKey('visualization_thingsvis-dashboards', {
      query: {
        projectId,
        ...(isNativeProvider.value ? { provider: 'native' } : {}),
        ...onboardingDashboardQuery.value
      }
    })
  }

  return {
    route,
    provider,
    providerError,
    projectCapabilities,
    providerBlockedMessage,
    isNativeProvider,
    isFirstDeviceOnboarding,
    onboardingDashboardQuery,
    loading,
    allProjects,
    searchKeyword,
    projects,
    fetchProjects,
    enterProject
  }
}

export type ThingsVisProjectList = ReturnType<typeof useThingsVisProjectList>
