<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { LocalVisualizationViewer, normalizeLocalDashboard } from '@/components/local-visualization-viewer'
import { getDefaultVisualizationProviderFacade } from '@/service/visualization-provider/composition'

const route = useRoute()
const providerFacade = getDefaultVisualizationProviderFacade()
const dashboard = ref<unknown | null>(null)
const loading = ref(false)
const failed = ref(false)
let requestSequence = 0

const boardId = computed(() => {
  const id = route.query.id
  return typeof id === 'string' && id.trim() ? id.trim() : ''
})

function isCurrentRequest(sequence: number, id: string) {
  return sequence === requestSequence && id === boardId.value
}

async function loadBoard() {
  const id = boardId.value
  const sequence = ++requestSequence

  dashboard.value = null
  failed.value = false
  loading.value = Boolean(id)

  if (!id) {
    failed.value = true
    return
  }

  try {
    const result = await providerFacade.execute(provider => provider.getDashboard(id))
    if (!isCurrentRequest(sequence, id)) return
    if (!result.ok || result.data.rendererData === undefined) {
      failed.value = true
      return
    }

    const rendererData = result.data.rendererData
    if (!normalizeLocalDashboard(rendererData).ok) {
      failed.value = true
      return
    }

    dashboard.value = rendererData
  } catch {
    if (!isCurrentRequest(sequence, id)) return
    dashboard.value = null
    failed.value = true
  } finally {
    if (isCurrentRequest(sequence, id)) {
      loading.value = false
    }
  }
}

watch(boardId, () => void loadBoard(), { immediate: true })

onBeforeUnmount(() => {
  requestSequence += 1
})
</script>

<template>
  <div class="h-full w-full bg-white">
    <LocalVisualizationViewer v-if="dashboard" :dashboard="dashboard" :fields="{}" />
    <div v-else class="flex h-full items-center justify-center text-gray-400" role="status">
      {{ loading ? 'Loading dashboard...' : failed ? 'Unable to load dashboard' : '' }}
    </div>
  </div>
</template>
