<!--
文件用途: 承载ThingsVis 菜单仪表盘相关的可视化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NAlert, NButton, NBreadcrumb, NBreadcrumbItem } from 'naive-ui'
import VisualizationProviderFrame from '@/components/visualization-provider/VisualizationProviderFrame.vue'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { resolveVisualizationProviderId } from '@/service/visualization-provider/composition'
import {
  getDefaultVisualizationProviderFacade,
  type VisualizationDashboardSchema
} from '@/service/visualization-provider/index'

const route = useRoute()
const { routerPushByKey } = useRouterPush()
const providerId = computed(() => resolveVisualizationProviderId({
  provider: route.query.provider,
  projectId: route.query.projectId
}))
const provider = getDefaultVisualizationProviderFacade({ providerId: providerId.value })
const providerSelectionError = provider.selectionError
const providerErrorTitle = computed(() =>
  providerSelectionError?.code === 'external-blocked'
    ? $t('rdi.thingsvis.externalProviderDisabledTitle')
    : $t('rdi.thingsvis.unableToLoadDashboard')
)
const providerErrorMessage = computed(() =>
  providerSelectionError?.code === 'external-blocked'
    ? $t('rdi.thingsvis.externalProviderDisabledDescription')
    : providerSelectionError?.message || $t('rdi.thingsvis.unableToLoadDashboard')
)

const dashboardSchema = ref<VisualizationDashboardSchema | null>(null)
const selectionError = ref(false)
const dashboardTitle = ref('')
let dashboardRequestSequence = 0

const dashboardId = computed(() => {
  const paramValue = route.params.dashboardId
  if (typeof paramValue === 'string' && paramValue.trim()) {
    return paramValue.trim()
  }

  const queryValue = route.query.id
  if (typeof queryValue === 'string' && queryValue.trim()) {
    return queryValue.trim()
  }

  const segments = route.path.split('/').filter(Boolean)
  if (segments.at(-1) === 'thingsvis-menu-dashboard') {
    return ''
  }
  return segments.at(-1) || ''
})

async function loadDashboard() {
  const currentDashboardId = dashboardId.value
  const requestSequence = ++dashboardRequestSequence

  dashboardSchema.value = null
  dashboardTitle.value = ''
  selectionError.value = false
  if (!currentDashboardId) return

  try {
    const result = await provider.execute(current => current.getDashboard(currentDashboardId))
    if (requestSequence !== dashboardRequestSequence || dashboardId.value !== currentDashboardId) return
    if (!result.ok || !result.data) {
      selectionError.value = true
      return
    }
    dashboardSchema.value = result.data
    dashboardTitle.value = result.data.name
  } catch (error) {
    if (requestSequence !== dashboardRequestSequence || dashboardId.value !== currentDashboardId) return
    console.warn('Failed to load embedded home dashboard', error)
    selectionError.value = true
  }
}

watch(
  dashboardId,
  () => {
    void loadDashboard()
  },
  { immediate: true }
)
</script>

<template>
  <div class="h-full w-full flex flex-col bg-[var(--layout-content-bg)]">
    <div class="flex items-center justify-between border-b border-gray-200 bg-white px-4 py-2 h-12">
      <NBreadcrumb>
        <NBreadcrumbItem class="cursor-pointer" @click="routerPushByKey('home')">
          {{ $t('rdi.thingsvis.home') }}
        </NBreadcrumbItem>
        <NBreadcrumbItem>
          {{ dashboardTitle || $t('rdi.thingsvis.dashboard') }}
        </NBreadcrumbItem>
      </NBreadcrumb>

      <NButton text @click="routerPushByKey('home')">
        {{ $t('rdi.thingsvis.backHome') }}
      </NButton>
    </div>

    <NAlert
      v-if="providerSelectionError"
      type="warning"
      class="m-4"
      role="alert"
      data-testid="thingsvis-provider-blocked"
      :data-provider-error="providerSelectionError.code"
    >
      <template #header>{{ providerErrorTitle }}</template>
      {{ providerErrorMessage }}
    </NAlert>
    <div v-else class="flex-1 overflow-auto bg-white">
      <VisualizationProviderFrame
        v-if="dashboardId && dashboardSchema && !selectionError"
        :id="dashboardId"
        :schema="dashboardSchema"
        :provider-id="providerId"
        mode="viewer"
        class="h-full w-full"
      />
      <div v-else class="flex h-full items-center justify-center text-gray-400">
        <div class="text-center" role="status">
          <p class="text-lg">{{ $t('rdi.thingsvis.unableToLoadDashboard') }}</p>
          <NButton class="mt-4" @click="routerPushByKey('home')">
            {{ $t('rdi.thingsvis.backHome') }}
          </NButton>
        </div>
      </div>
    </div>
  </div>
</template>
