import type { Component } from 'vue'
import type { VisualizationProviderId } from '@/service/visualization-provider/index'

export type VisualizationRenderer = Component

export class VisualizationRendererRegistry {
  private readonly renderers = new Map<VisualizationProviderId, VisualizationRenderer>()

  register(providerId: VisualizationProviderId, renderer: VisualizationRenderer): boolean {
    if (this.renderers.has(providerId)) return false
    this.renderers.set(providerId, renderer)
    return true
  }

  get(providerId: VisualizationProviderId): VisualizationRenderer | undefined {
    return this.renderers.get(providerId)
  }
}
