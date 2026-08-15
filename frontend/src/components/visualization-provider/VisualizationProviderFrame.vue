<script setup lang="ts">
import { computed } from 'vue'
import type { VisualizationDashboardSchema, VisualizationProviderContext } from '@/service/visualization-provider/index'
import {
  getDefaultVisualizationProviderFacade,
  LEGACY_THINGSVIS_PROVIDER_ID,
  NATIVE_BOARD_PROVIDER_ID
} from '@/service/visualization-provider/index'
import { getDefaultVisualizationRendererRegistry } from './composition'

const props = withDefaults(defineProps<{
  id: string
  mode?: string
  schema?: VisualizationDashboardSchema | null
  providerId?: string | null
  context?: Partial<VisualizationProviderContext>
  expectedOwnerId?: string
}>(), {
  mode: 'viewer',
  schema: null,
  context: () => ({ available: true, authenticated: true })
})

// Keep unqualified dashboards self-contained; legacy ThingsVis remains available only when explicitly selected.
const selectedProviderId = computed(() => props.providerId === undefined ? NATIVE_BOARD_PROVIDER_ID : props.providerId)

const emit = defineEmits<{
  hostSaveSuccess: [payload: { id: string; name?: string }]
}>()

const providerSelection = computed(() => {
  const providerId = selectedProviderId.value
  const facade = getDefaultVisualizationProviderFacade({
    providerId,
    context: props.context,
    expectedOwnerId: props.expectedOwnerId
  })
  return { providerId, facade }
})

const selectionError = computed(() => providerSelection.value.facade.selectionError)
const providerStatus = computed(() => {
  if (!selectionError.value) return selectedProviderId.value === LEGACY_THINGSVIS_PROVIDER_ID ? 'optional-external' : 'local-default'
  return selectedProviderId.value === LEGACY_THINGSVIS_PROVIDER_ID ? 'blocked-external' : 'blocked-local'
})
const renderer = computed(() => {
  const { providerId, facade } = providerSelection.value
  if (facade.selectionError || !providerId) return null
  return getDefaultVisualizationRendererRegistry().get(providerId) ?? null
})
</script>

<template>
  <div
    v-if="selectionError"
    role="alert"
    class="visualization-provider-blocked"
    :data-provider-error="selectionError.code"
    :data-provider-status="providerStatus"
  >
    {{ selectionError.message }}
  </div>
  <component
    :is="renderer"
    v-else-if="renderer"
    :id="id"
    :mode="mode"
    :schema="schema"
    @host-save-success="emit('hostSaveSuccess', $event)"
  />
</template>
