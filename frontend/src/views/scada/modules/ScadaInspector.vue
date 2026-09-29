<!--
  文件用途：SCADA 编辑器右侧属性检查器 + 版本列表。
  核心逻辑：几何字段统一由 geometryFields 驱动（替代原先四段重复的 NInputNumber），
    修改通过 update-node 事件交给父组件的 useCanvasEditor，本组件不直接改画布。
-->
<script setup lang="ts">
import { NButton, NCollapse, NCollapseItem, NEmpty, NForm, NFormItem, NInputNumber, NSpace, NTag } from 'naive-ui'

import type { ScadaDocumentVersion } from '@/service/api/scada'
import type { ScadaCanvasNode } from '../core/canvasDocument'

type GeometryKey = 'x' | 'y' | 'width' | 'height'

const props = defineProps<{
  node: ScadaCanvasNode | null
  versions: ScadaDocumentVersion[]
}>()

const emit = defineEmits<{
  (event: 'update-node', id: string, patch: Partial<Pick<ScadaCanvasNode, GeometryKey>>): void
  (event: 'bring-to-front', id: string): void
  (event: 'remove-node', id: string): void
  (event: 'rollback', version: number): void
}>()

const geometryFields: ReadonlyArray<{ key: GeometryKey; label: string }> = [
  { key: 'x', label: 'X' },
  { key: 'y', label: 'Y' },
  { key: 'width', label: 'W' },
  { key: 'height', label: 'H' }
]

function onGeometryChange(key: GeometryKey, value: number | null) {
  if (value === null || !props.node) return
  emit('update-node', props.node.id, { [key]: value })
}
</script>

<template>
  <aside class="scada-inspector">
    <NForm v-if="node" label-placement="left" size="small">
      <NFormItem v-for="field in geometryFields" :key="field.key" :label="field.label">
        <NInputNumber :value="node[field.key]" @update:value="(v: number | null) => onGeometryChange(field.key, v)" />
      </NFormItem>
      <NSpace>
        <NButton size="tiny" @click="emit('bring-to-front', node.id)">front</NButton>
        <NButton size="tiny" type="error" @click="emit('remove-node', node.id)">delete</NButton>
      </NSpace>
    </NForm>
    <NEmpty v-else description="select a node to edit" />

    <NCollapse class="scada-inspector__versions">
      <NCollapseItem title="versions" name="versions">
        <NSpace vertical :size="4">
          <NSpace v-for="version in versions" :key="version.id" align="center">
            <NTag size="small">v{{ version.version }}</NTag>
            <NButton size="tiny" @click="emit('rollback', version.version)">rollback</NButton>
          </NSpace>
        </NSpace>
      </NCollapseItem>
    </NCollapse>
  </aside>
</template>

<style scoped>
.scada-inspector {
  width: 200px;
  flex: 0 0 200px;
}

.scada-inspector__versions {
  margin-top: 12px;
}
</style>
