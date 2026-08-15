<script setup lang="ts">
import { computed } from 'vue'
import { resolveMetric, resolveText } from './data'
import LocalEChartsWidget from './LocalEChartsWidget.vue'
import type {
  ChartWidgetConfig,
  LocalViewerFields,
  MetricWidgetConfig,
  NormalizedLocalWidget,
  TextWidgetConfig
} from './types'

const props = defineProps<{
  widget: NormalizedLocalWidget
  fields: LocalViewerFields
}>()

const text = computed(() =>
  props.widget.type === 'text'
    ? resolveText(props.widget.config as TextWidgetConfig, props.fields)
    : null
)
const metric = computed(() =>
  props.widget.type === 'metric'
    ? resolveMetric(props.widget.config as MetricWidgetConfig, props.fields)
    : null
)
</script>

<template>
  <div class="local-widget" :data-widget-id="widget.id" :data-widget-type="widget.type">
    <div v-if="widget.type === 'text'" class="local-text-widget" :class="{ unavailable: !text?.available }">
      {{ text?.text }}
    </div>
    <div v-else-if="widget.type === 'metric'" class="local-metric-widget" :class="{ unavailable: !metric?.available }">
      <span class="local-metric-label">{{ metric?.label }}</span>
      <span class="local-metric-value">{{ metric?.value }}{{ metric?.unit }}</span>
    </div>
    <LocalEChartsWidget
      v-else-if="widget.type === 'line-chart' || widget.type === 'bar-chart'"
      :type="widget.type"
      :config="widget.config as ChartWidgetConfig"
      :fields="fields"
    />
    <div v-else class="local-widget-unsupported" role="status">
      Unsupported widget: {{ widget.originalType }}
    </div>
  </div>
</template>

<style scoped>
.local-widget {
  box-sizing: border-box;
  width: 100%;
  height: 100%;
  overflow: hidden;
  border: 1px solid rgba(128, 128, 128, 0.22);
  border-radius: 6px;
  background: var(--n-color, #fff);
}

.local-text-widget,
.local-metric-widget,
.local-widget-unsupported {
  box-sizing: border-box;
  height: 100%;
  padding: 12px;
}

.local-text-widget {
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.local-metric-widget {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
}

.local-metric-label {
  color: #6b7280;
  font-size: 13px;
}

.local-metric-value {
  overflow: hidden;
  font-size: 28px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.unavailable,
.local-widget-unsupported {
  color: #8c8c8c;
}
</style>
