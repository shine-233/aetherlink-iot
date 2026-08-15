<script setup lang="ts">
import { computed } from 'vue'
import { GridLayoutPlus } from '@/components/common/grid'
import type { GridLayoutPlusItem } from '@/components/common/grid'
import { normalizeLocalDashboard, normalizeLocalViewerFields } from './normalizer'
import LocalWidgetRenderer from './LocalWidgetRenderer.vue'
import type { LocalViewerFields, NormalizedLocalWidget } from './types'

const props = withDefaults(
  defineProps<{
    dashboard: unknown
    fields?: unknown
  }>(),
  {
    fields: () => Object.freeze({}) satisfies LocalViewerFields
  }
)

const normalized = computed(() => normalizeLocalDashboard(props.dashboard))
const normalizedFields = computed(() => normalizeLocalViewerFields(props.fields))
const dashboard = computed(() => (normalized.value.ok ? normalized.value.dashboard : null))
const fields = computed(() => (normalizedFields.value.ok ? normalizedFields.value.fields : null))
const gridLayout = computed<GridLayoutPlusItem[]>(() =>
  (dashboard.value?.widgets ?? []).map(widget => ({ ...widget, i: widget.id }))
)
const widgetById = computed(() => new Map((dashboard.value?.widgets ?? []).map(widget => [widget.id, widget])))
const widgetFor = (item: GridLayoutPlusItem): NormalizedLocalWidget | undefined => widgetById.value.get(item.i)
const gridConfig = computed(() => ({
  colNum: dashboard.value?.columns ?? 24,
  rowHeight: dashboard.value?.rowHeight ?? 60,
  isDraggable: false,
  isResizable: false,
  staticGrid: true,
  responsive: false
}))
</script>

<template>
  <div class="local-visualization-viewer">
    <div v-if="!dashboard" class="local-viewer-invalid" role="alert">
      Invalid local dashboard
    </div>
    <div v-else-if="!fields" class="local-viewer-invalid" role="alert">
      Invalid local viewer fields
    </div>
    <div v-else-if="gridLayout.length === 0" class="local-viewer-empty" role="status" data-testid="local-viewer-empty">
      <strong>This board has no widgets yet</strong>
      <span>Add a widget in the board editor to start building this view.</span>
    </div>
    <GridLayoutPlus
      v-else
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
        <LocalWidgetRenderer v-if="widgetFor(item)" :widget="widgetFor(item)!" :fields="fields" />
      </template>
    </GridLayoutPlus>
  </div>
</template>

<style scoped>
.local-visualization-viewer {
  width: 100%;
  height: 100%;
  min-height: 120px;
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
