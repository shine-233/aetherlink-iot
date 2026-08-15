<!--
文件用途: 承载ThingsVis 预览相关的可视化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NAlert } from 'naive-ui'
import VisualizationProviderFrame from '@/components/visualization-provider/VisualizationProviderFrame.vue'
import { $t } from '@/locales'
import { resolveVisualizationProviderId } from '@/service/visualization-provider/composition'
import { NATIVE_BOARD_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'
import {
  getDefaultVisualizationProviderFacade,
  type VisualizationDashboardSchema
} from '@/service/visualization-provider/index'
const PREVIEW_FRAME_IDLE_TIMEOUT_MS = 1200
const PREVIEW_FRAME_FALLBACK_DELAY_MS = 160

const route = useRoute()
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
const isPreviewFrameReady = ref(false)
let previewFrameIdleHandle: number | null = null
let previewFrameFallbackTimer: ReturnType<typeof setTimeout> | null = null
let dashboardRequestSequence = 0

const dashboardId = computed(() => {
  const queryValue = route.query.id
  if (typeof queryValue === 'string' && queryValue.trim()) {
    return queryValue.trim()
  }

  const paramValue = route.params.dashboardId
  if (typeof paramValue === 'string' && paramValue.trim()) {
    return paramValue.trim()
  }

  return ''
})
const shareToken = computed(() => {
  const value = route.query.shareToken
  return typeof value === 'string' && value.trim() ? value.trim() : ''
})

function clearPreviewFrameMountSchedule() {
  if (previewFrameIdleHandle !== null && typeof window !== 'undefined' && 'cancelIdleCallback' in window) {
    window.cancelIdleCallback(previewFrameIdleHandle)
  }
  previewFrameIdleHandle = null

  if (previewFrameFallbackTimer) {
    clearTimeout(previewFrameFallbackTimer)
    previewFrameFallbackTimer = null
  }
}

function markPreviewFrameReady() {
  clearPreviewFrameMountSchedule()
  isPreviewFrameReady.value = true
}

function schedulePreviewFrameMount() {
  clearPreviewFrameMountSchedule()
  isPreviewFrameReady.value = false

  if (!dashboardId.value) {
    return
  }

  if (typeof window === 'undefined') {
    isPreviewFrameReady.value = true
    return
  }

  if ('requestIdleCallback' in window && typeof window.requestIdleCallback === 'function') {
    previewFrameIdleHandle = window.requestIdleCallback(
      () => {
        previewFrameIdleHandle = null
        markPreviewFrameReady()
      },
      { timeout: PREVIEW_FRAME_IDLE_TIMEOUT_MS }
    )
    return
  }

  previewFrameFallbackTimer = setTimeout(() => {
    previewFrameFallbackTimer = null
    markPreviewFrameReady()
  }, PREVIEW_FRAME_FALLBACK_DELAY_MS)
}

async function loadDashboard() {
  const currentDashboardId = dashboardId.value
  const requestSequence = ++dashboardRequestSequence

  dashboardSchema.value = null
  selectionError.value = false
  if (!currentDashboardId) return

  try {
    const result = await provider.execute(current => {
      if (providerId.value === NATIVE_BOARD_PROVIDER_ID && shareToken.value && current.getDashboardByShareToken) {
        return current.getDashboardByShareToken(shareToken.value)
      }
      return current.getDashboard(currentDashboardId)
    })
    if (requestSequence !== dashboardRequestSequence || dashboardId.value !== currentDashboardId) return
    if (!result.ok || !result.data) {
      selectionError.value = true
      return
    }
    dashboardSchema.value = result.data
    document.title = `${result.data.name || $t('rdi.thingsvis.dashboard')} - ${$t('rdi.thingsvis.viewer')}`
  } catch (error) {
    if (requestSequence !== dashboardRequestSequence || dashboardId.value !== currentDashboardId) return
    console.warn('Failed to load preview dashboard', error)
    selectionError.value = true
  }
}

watch(
  dashboardId,
  () => {
    schedulePreviewFrameMount()
    void loadDashboard()
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  clearPreviewFrameMountSchedule()
})
</script>

<template>
  <div class="h-full w-full bg-white">
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
    <div v-else-if="dashboardId" class="h-full w-full overflow-auto bg-white">
      <VisualizationProviderFrame
        v-if="dashboardSchema && !selectionError && isPreviewFrameReady"
        :id="dashboardId"
        :schema="dashboardSchema"
        :provider-id="providerId"
        mode="viewer"
        class="h-full w-full"
      />
      <div v-else class="flex h-full items-center justify-center text-gray-400">
        <div class="text-center" role="status">
          <p class="text-lg">{{ selectionError ? $t('rdi.thingsvis.unableToLoadDashboard') : $t('rdi.thingsvis.viewer') }}</p>
          <p v-if="!selectionError" class="mt-2 text-sm opacity-70">{{ $t('common.loading') }}</p>
        </div>
      </div>
    </div>
    <div v-else class="flex h-full items-center justify-center text-gray-400">
      <div class="text-center">
        <p class="text-lg">{{ $t('rdi.thingsvis.unableToLoadDashboard') }}</p>
        <p class="text-sm mt-2 opacity-70">ID: {{ dashboardId }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Keep the viewer full screen. */
:global(body),
:global(#app) {
  height: 100vh;
  margin: 0;
  padding: 0;
  overflow: hidden;
}
</style>
