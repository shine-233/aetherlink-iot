<!--
  文件用途：SCADA 画布上单个 widget 的网格卡片（布局数值编辑 + 删除 + 控制命令）。
  核心逻辑：纯展示组件，所有修改通过事件交给父组件的 useScadaEditorWorkflow。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NInputNumber } from 'naive-ui'
import type { ScadaWidgetCommand, ScadaWidgetInstance } from './scada-model'

type LayoutKey = keyof ScadaWidgetInstance['layout']

const props = defineProps<{
  widget: ScadaWidgetInstance
  commands: ScadaWidgetCommand[]
  editable: boolean
}>()

const emit = defineEmits<{
  (event: 'remove', id: string): void
  (event: 'update-layout', id: string, patch: Partial<ScadaWidgetInstance['layout']>): void
  (event: 'control', widget: ScadaWidgetInstance, command: string): void
}>()

// x/y 缺省回 0，w/h 缺省回 1（跨度至少 1 格）。
const layoutFields: ReadonlyArray<{ key: LayoutKey; fallback: number }> = [
  { key: 'x', fallback: 0 },
  { key: 'y', fallback: 0 },
  { key: 'w', fallback: 1 },
  { key: 'h', fallback: 1 }
]

const gridStyle = computed(() => ({
  gridColumn: `${props.widget.layout.x + 1} / span ${props.widget.layout.w}`,
  gridRow: `${props.widget.layout.y + 1} / span ${props.widget.layout.h}`
}))

function onLayout(key: LayoutKey, value: number | null, fallback: number) {
  emit('update-layout', props.widget.id, { [key]: value ?? fallback })
}
</script>

<template>
  <div class="canvas-item" :style="gridStyle">
    <div class="row">
      <strong>{{ widget.widget_type }}@{{ widget.version }}</strong>
      <NButton size="tiny" :disabled="!editable" @click="emit('remove', widget.id)">{{ $t('common.delete') }}</NButton>
    </div>
    <div class="row">
      <NInputNumber
        v-for="field in layoutFields"
        :key="field.key"
        size="tiny"
        :value="widget.layout[field.key]"
        @update:value="(v: number | null) => onLayout(field.key, v, field.fallback)"
      />
    </div>
    <div class="row">
      <NButton
        v-for="cmd in commands"
        :key="cmd.name"
        size="tiny"
        :type="cmd.requires_confirmation ? 'warning' : 'default'"
        @click="emit('control', widget, cmd.name)"
      >
        {{ cmd.name }}{{ cmd.requires_confirmation ? ' ⚠' : '' }}
      </NButton>
    </div>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.canvas-item {
  padding: 6px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--card-color);
  overflow: hidden;
}
</style>
