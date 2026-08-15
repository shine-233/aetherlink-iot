import { createVisualizationProviderFacade, type VisualizationProviderSelection } from './facade'
import { legacyThingsVisProvider } from './legacy-thingsvis-adapter'
import { nativeBoardProvider } from './native-board-provider'
import {
  LEGACY_THINGSVIS_PROVIDER_ID,
  NATIVE_BOARD_PROJECT_ID,
  NATIVE_BOARD_PROVIDER_ID
} from './provider-ids'
import { VisualizationProviderRegistry } from './registry'

const registry = new VisualizationProviderRegistry()

export function registerDefaultVisualizationProviders(): VisualizationProviderRegistry {
  registry.register(nativeBoardProvider)
  registry.register(legacyThingsVisProvider)
  return registry
}

export function getDefaultVisualizationProviderFacade(selection: VisualizationProviderSelection = {}) {
  const providerId = selection.providerId === undefined ? NATIVE_BOARD_PROVIDER_ID : selection.providerId
  const externalEnabled = import.meta.env.VITE_ENABLE_THINGSVIS_COMPAT === 'Y'
  const effectiveSelection = providerId === LEGACY_THINGSVIS_PROVIDER_ID && !externalEnabled
    ? { ...selection, context: { ...selection.context, available: false } }
    : selection

  return createVisualizationProviderFacade(registerDefaultVisualizationProviders(), effectiveSelection)
}

export function getDefaultVisualizationProviderRegistry() {
  return registerDefaultVisualizationProviders()
}

type RouteQueryValue = unknown

function firstRouteQueryValue(value: RouteQueryValue): string {
  if (Array.isArray(value)) return firstRouteQueryValue(value[0])
  return typeof value === 'string' ? value.trim() : ''
}

/**
 * Resolve the provider for compatibility routes without probing an optional service.
 * Native boards are selected only by an explicit local marker or their built-in
 * project id; every other route keeps the legacy external provider contract.
 */
export function resolveVisualizationProviderId(selection: {
  provider?: RouteQueryValue
  projectId?: RouteQueryValue
}): string {
  const provider = firstRouteQueryValue(selection.provider).toLowerCase()
  const projectId = firstRouteQueryValue(selection.projectId)
  const externalEnabled = import.meta.env.VITE_ENABLE_THINGSVIS_COMPAT === 'Y'

  if (provider === 'native' || provider === 'local' || provider === NATIVE_BOARD_PROVIDER_ID) {
    return NATIVE_BOARD_PROVIDER_ID
  }
  if (
    provider === 'legacy' ||
    provider === 'external' ||
    provider === 'thingsvis' ||
    provider === LEGACY_THINGSVIS_PROVIDER_ID
  ) {
    return LEGACY_THINGSVIS_PROVIDER_ID
  }
  if (projectId === NATIVE_BOARD_PROJECT_ID) {
    return NATIVE_BOARD_PROVIDER_ID
  }
  if (projectId) return LEGACY_THINGSVIS_PROVIDER_ID

  // A route without a provider is the normal product path. Keep it
  // self-contained in the default build, and preserve the historical
  // external default only for the explicitly enabled compatibility profile.
  return externalEnabled ? LEGACY_THINGSVIS_PROVIDER_ID : NATIVE_BOARD_PROVIDER_ID
}
