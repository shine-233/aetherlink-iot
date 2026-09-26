<!--
文件用途: 承载ThingsVis 预览相关的可视化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NAlert, NButton } from 'naive-ui'
import VisualizationProviderFrame from '@/components/visualization-provider/VisualizationProviderFrame.vue'
import { $t } from '@/locales'
import { resolveVisualizationProviderId } from '@/service/visualization-provider/composition'
import { NATIVE_BOARD_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'
import {
  getDefaultVisualizationProviderFacade,
  type VisualizationDashboardSchema
} from '@/service/visualization-provider/index'
import { createCarouselTimer, parseCarouselQuery, type CarouselTimer } from './carousel'
const PREVIEW_FRAME_IDLE_TIMEOUT_MS = 1200
const PREVIEW_FRAME_FALLBACK_DELAY_MS = 160

const route = useRoute()
const providerId = computed(() =>
  resolveVisualizationProviderId({
    provider: route.query.provider,
    projectId: route.query.projectId
  })
)
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

// ---------------------------------------------------------------------------
// TP-22 大屏轮播：?tokens=a,b&interval=15 时进入轮播投屏模式。
// 数据面走 provider.getDashboardsByShareTokens（公开 share token 凭证语义，
// 不认内部 board id）；切换节奏是纯展示职责，落在 carousel.ts 计时器里。
// ---------------------------------------------------------------------------
const carouselQuery = computed(() => parseCarouselQuery(route.query as Record<string, unknown>))
const carouselPlaylist = ref<VisualizationDashboardSchema[]>([])
const carouselMissingTokens = ref<string[]>([])
const carouselPlaylistError = ref(false)
const carouselActiveIndex = ref(0)
let carouselTimer: CarouselTimer | null = null
let carouselRequestSequence = 0
const carouselRootRef = ref<HTMLElement | null>(null)
const isFullscreen = ref(false)

const carouselActiveSchema = computed(() => carouselPlaylist.value[carouselActiveIndex.value] ?? null)
const isCarouselMode = computed(() => carouselQuery.value !== null)

function onFullscreenChange() {
  isFullscreen.value = document.fullscreenElement === carouselRootRef.value
}

/** 全屏 API：必须由用户手势触发（浏览器策略），拒绝时保持轮播不受影响。 */
async function toggleFullscreen() {
  try {
    if (document.fullscreenElement) {
      await document.exitFullscreen()
    } else {
      await carouselRootRef.value?.requestFullscreen()
    }
  } catch (error) {
    console.warn('Fullscreen request rejected', error)
  }
}

async function loadCarouselPlaylist() {
  const tokens = carouselQuery.value?.tokens
  if (!tokens || tokens.length === 0) return
  const requestSequence = ++carouselRequestSequence

  carouselTimer?.stop()
  carouselTimer = null
  carouselPlaylist.value = []
  carouselMissingTokens.value = []
  carouselPlaylistError.value = false
  carouselActiveIndex.value = 0

  const result = await provider.execute(async (current) => {
    if (!current.getDashboardsByShareTokens) {
      return {
        ok: false as const,
        error: {
          code: 'unsupported-operation' as const,
          message: 'carousel playback requires native boards'
        }
      }
    }
    return current.getDashboardsByShareTokens(tokens)
  })
  if (requestSequence !== carouselRequestSequence || !carouselQuery.value) return
  if (!result.ok) {
    carouselPlaylistError.value = true
    return
  }
  carouselPlaylist.value = result.data.items
  carouselMissingTokens.value = result.data.missingTokens
  carouselTimer = createCarouselTimer({
    intervalMs: carouselQuery.value.intervalSeconds * 1000,
    count: () => carouselPlaylist.value.length,
    onAdvance: (index) => {
      carouselActiveIndex.value = index
    }
  })
  carouselTimer.start()
}

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
    const result = await provider.execute((current) => {
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
    // 轮播模式由轮播 watcher 接管：tokens 与 id 同时出现时轮播优先。
    if (carouselQuery.value) return
    schedulePreviewFrameMount()
    void loadDashboard()
  },
  { immediate: true }
)

watch(
  carouselQuery,
  () => {
    if (!carouselQuery.value) return
    void loadCarouselPlaylist()
  },
  { immediate: true }
)

// 轮播模式下标签页标题跟随当前屏，投屏器/录屏时能识别正在播哪块看板。
watch(carouselActiveSchema, (schema) => {
  if (carouselQuery.value && schema?.name) {
    document.title = `${schema.name} - ${$t('rdi.thingsvis.viewer')}`
  }
})

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
})

onBeforeUnmount(() => {
  clearPreviewFrameMountSchedule()
  carouselTimer?.stop()
  carouselTimer = null
  document.removeEventListener('fullscreenchange', onFullscreenChange)
})
</script>

<template>
  <div ref="carouselRootRef" class="h-full w-full bg-white">
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
    <!-- TP-22 轮播投屏模式：?tokens=... 激活，定时切换 + 全屏。 -->
    <div v-else-if="isCarouselMode" class="h-full w-full bg-white">
      <div
        v-if="carouselPlaylistError"
        class="flex h-full items-center justify-center text-gray-400"
        data-testid="tv-carousel-error"
      >
        <div class="text-center">
          <p class="text-lg">{{ $t('rdi.thingsvis.unableToLoadDashboard') }}</p>
          <p class="mt-2 text-sm opacity-70">{{ $t('rdi.thingsvis.carouselUnsupported') }}</p>
        </div>
      </div>
      <div
        v-else-if="!carouselActiveSchema"
        class="flex h-full items-center justify-center text-gray-400"
        data-testid="tv-carousel-empty"
      >
        <div class="text-center">
          <p class="text-lg">{{ $t('rdi.thingsvis.unableToLoadDashboard') }}</p>
          <p v-if="carouselMissingTokens.length" class="mt-2 text-sm opacity-70">
            {{ $t('rdi.thingsvis.carouselMissingTokens', { count: carouselMissingTokens.length }) }}
          </p>
        </div>
      </div>
      <div v-else class="relative h-full w-full">
        <VisualizationProviderFrame
          :id="carouselActiveSchema.id"
          :key="carouselActiveSchema.id"
          :schema="carouselActiveSchema"
          :provider-id="providerId"
          mode="viewer"
          class="h-full w-full"
        />
        <div class="tv-carousel-hud">
          <span data-testid="tv-carousel-position">{{ carouselActiveIndex + 1 }} / {{ carouselPlaylist.length }}</span>
          <NButton
            size="tiny"
            quaternary
            data-testid="tv-carousel-fullscreen"
            @click="toggleFullscreen"
          >
            {{ isFullscreen ? $t('rdi.thingsvis.carouselExitFullscreen') : $t('rdi.thingsvis.carouselFullscreen') }}
          </NButton>
        </div>
        <p
          v-if="carouselMissingTokens.length"
          data-testid="tv-carousel-missing"
          class="tv-carousel-missing"
          role="status"
        >
          {{ $t('rdi.thingsvis.carouselMissingTokens', { count: carouselMissingTokens.length }) }}
        </p>
      </div>
    </div>
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
          <p class="text-lg">
            {{ selectionError ? $t('rdi.thingsvis.unableToLoadDashboard') : $t('rdi.thingsvis.viewer') }}
          </p>
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

/* TP-22 轮播 HUD：半悬浮角落信息，投屏时低干扰可读。
   悬浮层刻意用固定的暗底白字（叠在任意内容的看板上都要可读），
   警示字色走设计令牌 --warning-color，不引入新的硬编码 hex。 */
.tv-carousel-hud {
  position: absolute;
  right: 16px;
  bottom: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgb(0 0 0 / 0.55);
  color: white;
  font-size: 13px;
}

.tv-carousel-missing {
  position: absolute;
  left: 16px;
  bottom: 16px;
  margin: 0;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgb(0 0 0 / 0.55);
  color: rgb(var(--warning-color));
  font-size: 13px;
}
</style>
