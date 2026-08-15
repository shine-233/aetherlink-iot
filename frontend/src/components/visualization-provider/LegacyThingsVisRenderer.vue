<script setup lang="ts">
import ThingsVisAppFrame from '@/components/thingsvis/ThingsVisAppFrame.vue'
import type { VisualizationDashboardSchema } from '@/service/visualization-provider/index'

defineProps<{
  id: string
  mode?: string
  schema?: VisualizationDashboardSchema | null
}>()

const emit = defineEmits<{
  hostSaveSuccess: [payload: { id: string; name?: string }]
}>()
</script>

<template>
  <ThingsVisAppFrame
    :id="id"
    :mode="mode"
    :schema="schema"
    @host-save-success="emit('hostSaveSuccess', $event)"
  />
</template>
