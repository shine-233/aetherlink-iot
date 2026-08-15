<script setup lang="ts">
import { LocalVisualizationViewer } from '@/components/local-visualization-viewer'
import type { VisualizationDashboardSchema } from '@/service/visualization-provider/index'

const props = defineProps<{
  id: string
  mode?: string
  schema?: VisualizationDashboardSchema | null
}>()
</script>

<template>
  <LocalVisualizationViewer v-if="props.schema" :dashboard="props.schema.rendererData ?? props.schema" />
</template>
