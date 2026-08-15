import type { VisualizationProvider, VisualizationProviderId } from './contracts'

export class VisualizationProviderRegistry {
  private readonly providers = new Map<VisualizationProviderId, VisualizationProvider>()

  register(provider: VisualizationProvider): boolean {
    if (this.providers.has(provider.id)) return false
    this.providers.set(provider.id, provider)
    return true
  }

  get(id: VisualizationProviderId): VisualizationProvider | undefined {
    return this.providers.get(id)
  }

  has(id: VisualizationProviderId): boolean {
    return this.providers.has(id)
  }

  ids(): VisualizationProviderId[] {
    return [...this.providers.keys()]
  }
}
