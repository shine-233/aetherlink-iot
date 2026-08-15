<!--
  文件说明：
  - 实现 ThingsVis 可视化运行时在 AetherLink 前端中的 iframe 嵌入与宿主桥接。
  - 负责令牌传递、初始化加载、消息分发、设备目录、实时数据与保存回传。
  维护提示：
  - host key、消息类型、saveTarget 和兼容别名属于跨系统契约，改动要同步检查宿主与 iframe 两侧。
  - 本文件当前主要保留 bridge 装配和 iframe 宿主入口；初始化生命周期与 transport/message 壳已拆到独立模块。
-->
<template>
  <div ref="frameContainerRef" class="thingsvis-frame-container" :style="containerInlineStyle" @wheel.passive="handleFrameWheel">
    <NAlert v-if="activeDiagnostic" type="warning" class="thingsvis-diagnostic-alert" closable @close="clearDiagnostic">
      <template #header>{{ $t('page.thingsvis.hostDiagnosticTitle') }}</template>
      <div class="thingsvis-diagnostic-content">
        <div class="thingsvis-diagnostic-message">
          {{ $t('page.thingsvis.hostDiagnosticDescription') }}
        </div>
        <div class="thingsvis-diagnostic-detail">
          <span>{{ activeDiagnostic.scope }}</span>
          <span>{{ activeDiagnostic.message }}</span>
        </div>
        <NButton size="tiny" type="warning" secondary @click="retryFrame">
          {{ $t('page.thingsvis.retry') }}
        </NButton>
      </div>
    </NAlert>
    <iframe
      v-if="url"
      :key="frameReloadKey"
      ref="iframeRef"
      :src="url"
      class="thingsvis-frame"
      :style="frameInlineStyle"
      frameborder="0"
      :allow="thingsVisIframeAllow"
      allowfullscreen
      @load="handleIframeLoad"
    ></iframe>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { NAlert, NButton } from 'naive-ui'
import { $t } from '@/locales'
import {
  createThingsVisContentHeightReporter,
  type ThingsVisContentHeightReporter
} from '@/components/thingsvis/thingsvisContentHeightReporter'
import { createThingsVisFrameTransportBridge } from '@/components/thingsvis/thingsvisFrameTransportBridge'
import type { ThingsVisHostDiagnostic } from '@/components/thingsvis/thingsvisHostErrorPayload'
import { useThingsVisAppFrameHostRuntime, type ThingsVisAppFrameSchema } from './useThingsVisAppFrameHostRuntime'
import { createLogger } from '@/utils/logger'

const logger = createLogger('AppFrame')

const props = defineProps<{
  id: string
  mode?: string
  schema?: ThingsVisAppFrameSchema
}>()
const emit = defineEmits<{
  hostSaveSuccess: [payload: { id: string; name?: string }]
}>()

const token = ref('')
const url = ref('')
const iframeRef = ref<HTMLIFrameElement>()
const frameContainerRef = ref<HTMLElement>()
const frameReloadKey = ref(0)
const activeDiagnostic = ref<ThingsVisHostDiagnostic | null>(null)
const frameContentHeight = ref<number | null>(null)
let contentHeightReporter: ThingsVisContentHeightReporter | null = null
const frameTransportBridge = createThingsVisFrameTransportBridge({
  iframeRef,
  url
})

const MIN_FRAME_CONTENT_HEIGHT = 320
const MAX_FRAME_CONTENT_HEIGHT = 6000
const thingsVisIframeAllow = 'fullscreen; autoplay; clipboard-write; camera; microphone; encrypted-media; picture-in-picture'

type ThingsVisWheelPayload = {
  source: 'thingsvis-app-frame'
  mode: string
  deltaX: number
  deltaY: number
  deltaZ: number
  deltaMode: number
  clientX: number
  clientY: number
  ctrlKey: boolean
  shiftKey: boolean
  altKey: boolean
  metaKey: boolean
}

let pendingWheelPayload: ThingsVisWheelPayload | null = null
let pendingWheelFrame = 0

const frameHeight = computed(() => {
  if (!frameContentHeight.value) return undefined
  return `${frameContentHeight.value}px`
})

const containerInlineStyle = computed(() => {
  return frameHeight.value ? { height: frameHeight.value } : undefined
})

const frameInlineStyle = computed(() => {
  return frameHeight.value ? { height: frameHeight.value } : undefined
})

function showDiagnostic(diagnostic: ThingsVisHostDiagnostic) {
  activeDiagnostic.value = diagnostic
}

function clearDiagnostic() {
  activeDiagnostic.value = null
}

function retryFrame() {
  clearDiagnostic()
  if (!url.value) {
    window.location.reload()
    return
  }

  frameReloadKey.value += 1
}

function handleFrameContentHeight(height: number) {
  if (!Number.isFinite(height)) return

  frameContentHeight.value = Math.ceil(
    Math.min(MAX_FRAME_CONTENT_HEIGHT, Math.max(MIN_FRAME_CONTENT_HEIGHT, height))
  )
  contentHeightReporter?.report(frameContentHeight.value, {
    source: 'thingsvis-app-frame',
    mode: props.mode || 'viewer'
  })
}

function handleFrameWheel(event: WheelEvent) {
  pendingWheelPayload = {
    source: 'thingsvis-app-frame',
    mode: props.mode || 'viewer',
    deltaX: event.deltaX,
    deltaY: event.deltaY,
    deltaZ: event.deltaZ,
    deltaMode: event.deltaMode,
    clientX: event.clientX,
    clientY: event.clientY,
    ctrlKey: event.ctrlKey,
    shiftKey: event.shiftKey,
    altKey: event.altKey,
    metaKey: event.metaKey
  }

  if (pendingWheelFrame) return

  pendingWheelFrame = window.requestAnimationFrame(() => {
    pendingWheelFrame = 0
    if (!pendingWheelPayload) return

    frameTransportBridge.postToThingsVis('tv:wheel', pendingWheelPayload)
    pendingWheelPayload = null
  })
}

const { handleIframeLoad } = useThingsVisAppFrameHostRuntime({
  iframeRef,
  token,
  url,
  frameTransportBridge,
  getContext: () => ({
    id: props.id,
    mode: props.mode,
    schema: props.schema
  }),
  onDiagnostic: showDiagnostic,
  onRecovered: clearDiagnostic,
  onFrameContentHeight: handleFrameContentHeight,
  emitHostSaveSuccess: (payload) => emit('hostSaveSuccess', payload),
  logger,
  message: (window as any).$message,
  fallbackAlert: (message) => alert(message),
  openPreview: (href) => window.open(href, '_blank')
})

onMounted(() => {
  contentHeightReporter = createThingsVisContentHeightReporter({
    getElement: () => frameContainerRef.value,
    getExtraPayload: () => ({
      source: 'thingsvis-app-frame-shell',
      mode: props.mode || 'viewer'
    })
  })
  contentHeightReporter.start()
})

onBeforeUnmount(() => {
  if (pendingWheelFrame) {
    window.cancelAnimationFrame(pendingWheelFrame)
    pendingWheelFrame = 0
  }
  pendingWheelPayload = null
  contentHeightReporter?.stop()
  contentHeightReporter = null
})
</script>

<style scoped>
.thingsvis-frame-container {
  width: 100%;
  height: 100%;
  min-height: clamp(320px, 48vh, 560px);
  position: relative;
}

.thingsvis-diagnostic-alert {
  position: absolute;
  z-index: 2;
  top: 16px;
  right: 16px;
  left: 16px;
  max-width: min(760px, calc(100% - 32px));
  box-shadow: 0 14px 40px rgb(15 23 42 / 14%);
}

.thingsvis-diagnostic-content {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  align-items: center;
}

.thingsvis-diagnostic-message {
  flex: 1 1 260px;
}

.thingsvis-diagnostic-detail {
  display: flex;
  flex: 1 1 100%;
  gap: 8px;
  font-size: 12px;
  color: rgb(71 85 105);
  overflow-wrap: anywhere;
}

.thingsvis-diagnostic-detail span:first-child {
  flex: 0 0 auto;
  font-weight: 600;
}

.thingsvis-diagnostic-detail span:last-child {
  min-width: 0;
}

.thingsvis-frame {
  width: 100%;
  height: 100%;
  display: block;
}
</style>
