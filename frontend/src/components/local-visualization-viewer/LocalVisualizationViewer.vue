<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { GridLayoutPlus } from '@/components/common/grid'
import type { GridLayoutPlusItem } from '@/components/common/grid'
import { normalizeLocalDashboard, normalizeLocalViewerFields } from './normalizer'
import LocalWidgetRenderer from './LocalWidgetRenderer.vue'
import type { LocalViewerFields, NormalizedLocalWidget } from './types'
import { TimewindowSelector } from './timewindow'
import { DEFAULT_TIMEWINDOW_CONFIG } from './timewindow/timewindow-model'
import type { TimewindowConfig } from './timewindow/types'
import { adaptDashboardLayout, getBreakpointForWidth, getColumnCountForBreakpoint } from './responsive'

const props = withDefaults(
  defineProps<{
    dashboard: unknown
    fields?: unknown
    showTimewindow?: boolean
    enableResponsive?: boolean
  }>(),
  {
    fields: () => Object.freeze({}) satisfies LocalViewerFields,
    showTimewindow: undefined,
    enableResponsive: true
  }
)

const emit = defineEmits<{
  (e: 'timewindowChange', timewindow: TimewindowConfig): void
}>()

const rootRef = ref<HTMLElement | null>(null)
const containerWidth = ref<number>(1200)

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  if (rootRef.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0) {
          containerWidth.value = entry.contentRect.width
        }
      }
    })
    resizeObserver.observe(rootRef.value)
    if (rootRef.value.clientWidth > 0) {
      containerWidth.value = rootRef.value.clientWidth
    }
  }
})

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
})

const normalizedDashboardResult = computed(() => normalizeLocalDashboard(props.dashboard))
const normalizedFieldsResult = computed(() => normalizeLocalViewerFields(props.fields))
const dashboardData = computed(() =>
  normalizedDashboardResult.value.ok ? normalizedDashboardResult.value.dashboard : null
)
const viewerFields = computed(() => (normalizedFieldsResult.value.ok ? normalizedFieldsResult.value.fields : null))

const shouldShowTimewindow = computed(() => {
  if (props.showTimewindow !== undefined) return props.showTimewindow
  return Boolean(dashboardData.value?.timewindow)
})

const activeTimewindow = ref<TimewindowConfig>(DEFAULT_TIMEWINDOW_CONFIG)

watch(
  () => dashboardData.value?.timewindow,
  (newTw) => {
    if (newTw) {
      activeTimewindow.value = { ...newTw }
    }
  },
  { immediate: true }
)

function handleTimewindowUpdate(tw: TimewindowConfig) {
  activeTimewindow.value = tw
  emit('timewindowChange', tw)
}

const isResponsive = computed(() => {
  if (!props.enableResponsive) return false
  return Boolean(dashboardData.value?.responsive)
})

const currentBreakpoint = computed(() => getBreakpointForWidth(containerWidth.value))

const baseCols = computed(() => dashboardData.value?.columns ?? 24)

const effectiveCols = computed(() => {
  if (!isResponsive.value) return baseCols.value
  return getColumnCountForBreakpoint(currentBreakpoint.value)
})

const displayedWidgets = computed<readonly NormalizedLocalWidget[]>(() => {
  const original = dashboardData.value?.widgets ?? []
  if (!isResponsive.value || effectiveCols.value === baseCols.value) {
    return original
  }
  return adaptDashboardLayout(original, effectiveCols.value, baseCols.value)
})

const gridLayout = computed<GridLayoutPlusItem[]>(() =>
  displayedWidgets.value.map((widget) => ({ ...widget, i: widget.id }))
)

const widgetById = computed(() => new Map(displayedWidgets.value.map((widget) => [widget.id, widget])))
const widgetFor = (item: GridLayoutPlusItem): NormalizedLocalWidget | undefined => widgetById.value.get(item.i)

const gridConfig = computed(() => ({
  colNum: effectiveCols.value,
  rowHeight: dashboardData.value?.rowHeight ?? 60,
  isDraggable: false,
  isResizable: false,
  staticGrid: true,
  responsive: false
}))
</script>

<template>
  <div ref="rootRef" class="local-visualization-viewer">
    <div v-if="!dashboardData" class="local-viewer-invalid" role="alert">Invalid local dashboard</div>
    <div v-else-if="!viewerFields" class="local-viewer-invalid" role="alert">Invalid local viewer fields</div>
    <div v-else-if="gridLayout.length === 0" class="local-viewer-empty" role="status" data-testid="local-viewer-empty">
      <strong>This board has no widgets yet</strong>
      <span>Add a widget in the board editor to start building this view.</span>
    </div>
    <div v-else class="local-viewer-content">
      <div v-if="shouldShowTimewindow" class="local-viewer-toolbar" data-testid="viewer-toolbar">
        <div class="toolbar-left">
          <span v-if="isResponsive" class="breakpoint-indicator" data-testid="breakpoint-indicator">
            {{ currentBreakpoint.toUpperCase() }} ({{ effectiveCols }} 列)
          </span>
        </div>
        <div class="toolbar-right">
          <TimewindowSelector :model-value="activeTimewindow" @update:model-value="handleTimewindowUpdate" />
        </div>
      </div>
      <GridLayoutPlus
        :layout="gridLayout"
        :config="gridConfig"
        readonly
        :show-grid="false"
        :show-drop-zone="false"
        :show-title="false"
        :content-padding="false"
        id-key="id"
      >
        <template #default="{ item }">
          <LocalWidgetRenderer
            v-if="widgetFor(item)"
            :widget="widgetFor(item)!"
            :fields="viewerFields"
            :timewindow="activeTimewindow"
          />
        </template>
      </GridLayoutPlus>
    </div>
  </div>
</template>

<style scoped>
.local-visualization-viewer {
  width: 100%;
  height: 100%;
  min-height: 120px;
}

.local-viewer-content {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
}

.local-viewer-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  margin-bottom: 8px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
  background: rgba(248, 250, 252, 0.5);
  border-radius: 4px;
}

.breakpoint-indicator {
  font-size: 12px;
  font-weight: 600;
  padding: 2px 8px;
  background: #e0f2fe;
  color: #0369a1;
  border-radius: 4px;
}

.local-viewer-invalid {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
  color: #8c8c8c;
  border: 1px dashed rgba(128, 128, 128, 0.4);
}

.local-viewer-empty {
  display: flex;
  min-height: 220px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 8px;
  padding: 24px;
  color: #6b7280;
  border: 1px dashed rgba(128, 128, 128, 0.35);
  border-radius: 8px;
  background: rgba(248, 250, 252, 0.72);
  text-align: center;
}

.local-viewer-empty strong {
  color: #374151;
  font-size: 16px;
}
</style>
