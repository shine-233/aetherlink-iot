// 文件用途：承载首页引导相关的后台探测状态（部署健康、场景自动化）。
// 核心逻辑：各自维护加载态与在途请求去重，为客户引导进度提供健康与自动化判定输入。
// 关键注意事项：探测失败时回退为不可用默认值，不阻断页面主链路。
import { computed, ref } from 'vue'
import { sceneAutomationsGet } from '@/service/api/automation'
import { $t } from '@/locales'
import {
  fetchDeploymentHealthReport,
  normalizeDeploymentHealth,
  type DeploymentHealthReport
} from './homeDeploymentHealth'

const unwrapListResponse = (response: any): any[] => {
  const data = response?.data?.data ?? response?.data ?? response ?? {}
  const list = data?.list ?? data?.records ?? data?.data ?? []
  return Array.isArray(list) ? list : []
}

export function useHomeGuideProbes() {
  let deploymentHealthRefreshPromise: Promise<void> | null = null
  let automationGuideRefreshPromise: Promise<void> | null = null

  const deploymentHealthLoading = ref(false)
  const deploymentHealth = ref<DeploymentHealthReport | null>(null)
  const deploymentHealthRows = computed(() => normalizeDeploymentHealth(deploymentHealth.value, $t))
  const deploymentHealthOk = computed(
    () =>
      deploymentHealth.value?.status === 'ok' &&
      deploymentHealthRows.value.length > 0 &&
      deploymentHealthRows.value.every((row) => row.ok)
  )

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

  const automationGuideLoading = ref(false)
  const hasSceneAutomation = ref(false)

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

  return {
    deploymentHealthLoading,
    deploymentHealthRows,
    deploymentHealthOk,
    refreshDeploymentHealth,
    automationGuideLoading,
    hasSceneAutomation,
    refreshAutomationGuideState
  }
}
