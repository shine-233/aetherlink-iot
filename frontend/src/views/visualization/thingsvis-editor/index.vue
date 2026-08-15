<!--
文件用途: 承载ThingsVis 编辑器相关的可视化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NAlert, NButton, NBreadcrumb, NBreadcrumbItem } from 'naive-ui'
import VisualizationProviderFrame from '@/components/visualization-provider/VisualizationProviderFrame.vue'
import { $t } from '@/locales'
import { useRouterPush } from '@/hooks/common/router'
import { resolveVisualizationProviderId } from '@/service/visualization-provider/composition'
import {
  getDefaultVisualizationProviderFacade,
  type VisualizationDashboardSchema
} from '@/service/visualization-provider/index'
import { NATIVE_BOARD_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'

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

const dashboardId = computed(() => String(route.query.id || '').trim())
const currentProjectId = computed(() => {
  const routeProjectId = String(route.query.projectId || '').trim()
  return routeProjectId || dashboardSchema.value?.projectId || ''
})
const projectTitle = ref('')
const dashboardSchema = ref<VisualizationDashboardSchema | null>(null)
const selectionError = ref(false)
let dashboardRequestSequence = 0

/** Load the dashboard title for breadcrumb display. */
const loadDashboardInfo = async () => {
  const currentDashboardId = dashboardId.value
  const requestSequence = ++dashboardRequestSequence
  projectTitle.value = ''
  dashboardSchema.value = null
  selectionError.value = false

  if (!currentDashboardId) return

  try {
    const result = await provider.execute(current => current.getDashboard(currentDashboardId))
    if (requestSequence !== dashboardRequestSequence || dashboardId.value !== currentDashboardId) return
    if (!result.ok || !result.data) {
      selectionError.value = true
      return
    }

    projectTitle.value = result.data.name
    dashboardSchema.value = result.data
  } catch (error) {
    if (requestSequence !== dashboardRequestSequence || dashboardId.value !== currentDashboardId) return
    console.warn('Failed to load dashboard title', error)
    selectionError.value = true
  }
}

const goBack = () => {
  if (currentProjectId.value) {
    routerPushByKey('visualization_thingsvis-dashboards', {
      query: {
        projectId: currentProjectId.value,
        ...(providerId.value === NATIVE_BOARD_PROVIDER_ID ? { provider: 'native' } : {})
      }
    })
    return
  }

  routerPushByKey('visualization_thingsvis')
}

const openNativeEditor = () => {
  if (!dashboardId.value) return
  routerPushByKey('visualization_native-board-editor', {
    query: { id: dashboardId.value }
  })
}

watch(
  dashboardId,
  () => {
    void loadDashboardInfo()
  },
  { immediate: true }
)
</script>

<template>
  <div class="h-full w-full flex flex-col">
    <!-- Top navigation bar -->
    <div class="flex items-center justify-between gap-4 border-b border-gray-200 bg-white px-4 py-2 min-h-12">
      <NBreadcrumb>
        <NBreadcrumbItem class="cursor-pointer" @click="goBack">
          {{ $t('rdi.thingsvis.dashboardList') }}
        </NBreadcrumbItem>
        <NBreadcrumbItem>
          {{ projectTitle || $t('common.loading') }}
        </NBreadcrumbItem>
      </NBreadcrumb>

      <NButton text @click="goBack">
        {{ $t('common.back') }}
      </NButton>
    </div>

    <!-- Native dashboards have a real local editor; the compatibility route is
         intentionally a bridge so legacy links do not open a read-only viewer. -->
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
    <div v-else-if="providerId === NATIVE_BOARD_PROVIDER_ID" class="flex flex-1 items-center justify-center bg-white p-6">
      <div class="max-w-xl text-center" role="status">
        <div class="text-lg font-semibold">{{ $t('custom.nativeBoardEditor.title') }}</div>
        <div class="mt-2 text-sm text-gray-500">{{ $t('custom.nativeBoards.subtitle') }}</div>
        <NButton class="mt-4" type="primary" :disabled="!dashboardId" @click="openNativeEditor">
          {{ $t('custom.nativeBoards.edit') }}
        </NButton>
      </div>
    </div>
    <div v-else class="relative flex-1 overflow-auto bg-white">
      <VisualizationProviderFrame
        v-if="dashboardId && dashboardSchema && !selectionError"
        :id="dashboardId"
        :schema="dashboardSchema"
        :provider-id="providerId"
        mode="editor"
      />
      <div v-else class="flex h-full items-center justify-center text-gray-400" role="status">
        {{ selectionError ? $t('rdi.thingsvis.unableToLoadDashboard') : $t('common.loading') }}
      </div>
    </div>
  </div>
</template>
