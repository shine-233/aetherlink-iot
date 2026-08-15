import { defineAsyncComponent } from 'vue'
import {
  LEGACY_THINGSVIS_PROVIDER_ID,
  NATIVE_BOARD_PROVIDER_ID
} from '@/service/visualization-provider/provider-ids'
import { VisualizationRendererRegistry } from './renderer-registry'

// Keep renderer implementations behind async boundaries so each loads in its own chunk.
const LocalVisualizationRenderer = defineAsyncComponent(() => import('./LocalVisualizationRenderer.vue'))
const LegacyThingsVisRenderer = defineAsyncComponent(() => import('./LegacyThingsVisRenderer.vue'))

const registry = new VisualizationRendererRegistry()

export function registerDefaultVisualizationRenderers(): VisualizationRendererRegistry {
  registry.register(NATIVE_BOARD_PROVIDER_ID, LocalVisualizationRenderer)
  registry.register(LEGACY_THINGSVIS_PROVIDER_ID, LegacyThingsVisRenderer)
  return registry
}

export function registerLocalVisualizationRenderer(providerId: string): boolean {
  registerDefaultVisualizationRenderers()
  return registry.register(providerId, LocalVisualizationRenderer)
}

export function getDefaultVisualizationRendererRegistry() {
  return registerDefaultVisualizationRenderers()
}
