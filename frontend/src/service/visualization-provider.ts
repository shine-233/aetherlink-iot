import type { VisualizationProvider } from './visualization-provider/contracts'
import { legacyThingsVisProvider } from './visualization-provider/legacy-thingsvis-adapter'
import { nativeBoardProvider } from './visualization-provider/native-board-provider'

export * from './visualization-provider/index'

// Keep the historical entry point while making the self-hosted provider the
// default. The external alias remains available so existing callers can opt in
// without importing ThingsVis API details directly.
export const VISUALIZATION_PROVIDER_KINDS = {
  local: 'local',
  external: 'external'
} as const

export type VisualizationProviderKind = (typeof VISUALIZATION_PROVIDER_KINDS)[keyof typeof VISUALIZATION_PROVIDER_KINDS]

export const localVisualizationProvider = nativeBoardProvider
export const externalVisualizationProvider = legacyThingsVisProvider

export function getVisualizationProvider(kind: unknown = VISUALIZATION_PROVIDER_KINDS.local): VisualizationProvider | null {
  if (kind === VISUALIZATION_PROVIDER_KINDS.local) return localVisualizationProvider
  if (kind === VISUALIZATION_PROVIDER_KINDS.external) return externalVisualizationProvider
  return null
}
