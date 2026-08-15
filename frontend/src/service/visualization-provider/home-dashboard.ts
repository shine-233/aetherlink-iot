import { probeThingsVisHomeDashboard, type ThingsVisHomeProbeResult } from '@/service/api/thingsvis'
import { getDefaultVisualizationProviderFacade } from './composition'
import type { VisualizationProviderFacade } from './facade'
import { LEGACY_THINGSVIS_PROVIDER_ID, NATIVE_BOARD_PROVIDER_ID } from './provider-ids'
import type {
  VisualizationDashboardNode,
  VisualizationError,
  VisualizationProvider,
  VisualizationResult
} from './contracts'

/**
 * The small dashboard shape the Home page needs to render a viewer.
 * Provider-specific fields stay behind this seam; CRUD callers continue to
 * use the full VisualizationDashboardSchema contract.
 */
export interface VisualizationHomeDashboard {
  id: string
  name: string
  canvasConfig: Record<string, unknown>
  nodes: VisualizationDashboardNode[]
  dataSources: unknown[]
  variables?: unknown[]
  thumbnail?: string | null
  rendererData?: unknown
  providerId?: string
}

export interface VisualizationHomeProbeResult {
  reachable: boolean
  status: number
  dashboard: VisualizationHomeDashboard | null
}

export type VisualizationHomeLoadResult = VisualizationResult<VisualizationHomeDashboard | null>

type HomeProviderFacade = Pick<VisualizationProviderFacade, 'execute' | 'selectionError'>

export interface VisualizationHomeDashboardResolver {
  probe(): Promise<VisualizationHomeProbeResult>
  load(): Promise<VisualizationHomeLoadResult>
}

export interface VisualizationHomeDashboardResolverDependencies {
  provider: HomeProviderFacade
  probe: () => Promise<ThingsVisHomeProbeResult>
  providerId?: string
  tenantId?: string
}

export interface VisualizationHomeDashboardOptions {
  tenantId?: string
}

const providerSelectionFailure = (error: VisualizationError): VisualizationHomeLoadResult => ({
  ok: false,
  error
})

function isRecord(value: unknown): value is Record<string, any> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function normalizeHomeDashboard(value: unknown, providerId?: string): VisualizationHomeDashboard | null {
  if (!isRecord(value)) return null
  if (typeof value.id !== 'string' || !value.id || typeof value.name !== 'string') return null
  if (!isRecord(value.canvasConfig) || !Array.isArray(value.nodes) || !Array.isArray(value.dataSources)) return null

  const dashboard: VisualizationHomeDashboard = {
    id: value.id,
    name: value.name,
    canvasConfig: value.canvasConfig,
    nodes: value.nodes as VisualizationDashboardNode[],
    dataSources: value.dataSources,
    variables: Array.isArray(value.variables) ? value.variables : undefined,
    thumbnail: typeof value.thumbnail === 'string' || value.thumbnail === null ? value.thumbnail : null
  }

  if ('rendererData' in value) dashboard.rendererData = value.rendererData
  if (providerId) dashboard.providerId = providerId
  return dashboard
}

function unavailableProbe(error: VisualizationError | null): VisualizationHomeProbeResult {
  return {
    reachable: false,
    status: error?.status ?? 0,
    dashboard: null
  }
}

export function createVisualizationHomeDashboardResolver(
  dependencies: VisualizationHomeDashboardResolverDependencies
): VisualizationHomeDashboardResolver {
  return {
    async probe() {
      if (dependencies.provider.selectionError) {
        return unavailableProbe(dependencies.provider.selectionError)
      }

      try {
        const result = await dependencies.probe()
        return {
          reachable: result.reachable,
          status: result.status,
          dashboard: normalizeHomeDashboard(result.dashboard, dependencies.providerId)
        }
      } catch {
        return unavailableProbe(null)
      }
    },

    async load() {
      if (dependencies.provider.selectionError) {
        return providerSelectionFailure(dependencies.provider.selectionError)
      }

      const result = await dependencies.provider.execute((provider: VisualizationProvider) => provider.getHomeDashboard(
        dependencies.tenantId ? { tenantId: dependencies.tenantId } : undefined
      ))
      if (!result.ok) return result
      return { ok: true, data: normalizeHomeDashboard(result.data, dependencies.providerId) }
    }
  }
}

async function probeNativeHomeDashboard(provider: HomeProviderFacade, tenantId?: string): Promise<ThingsVisHomeProbeResult> {
  if (provider.selectionError) {
    return { reachable: false, status: provider.selectionError.status ?? 0, dashboard: null }
  }

  try {
    const result = await provider.execute((selected: VisualizationProvider) => selected.getHomeDashboard(
      tenantId ? { tenantId } : undefined
    ))
    if (!result.ok) {
      return { reachable: false, status: result.error.status ?? 0, dashboard: null }
    }

    return {
      reachable: true,
      status: 200,
      dashboard: result.data
        ? ({ ...result.data, isPublished: result.data.published } as unknown as ThingsVisHomeProbeResult['dashboard'])
        : null
    }
  } catch {
    return { reachable: false, status: 0, dashboard: null }
  }
}

function getDefaultResolver(options: VisualizationHomeDashboardOptions = {}) {
  const useExternalProvider = import.meta.env.VITE_ENABLE_THINGSVIS_COMPAT === 'Y'
  const providerId = useExternalProvider ? LEGACY_THINGSVIS_PROVIDER_ID : NATIVE_BOARD_PROVIDER_ID
  const provider = getDefaultVisualizationProviderFacade({ providerId })

  return createVisualizationHomeDashboardResolver({
    provider,
    providerId,
    tenantId: options.tenantId,
    probe: useExternalProvider ? probeThingsVisHomeDashboard : () => probeNativeHomeDashboard(provider, options.tenantId)
  })
}

export function probeVisualizationHomeDashboard(
  options: VisualizationHomeDashboardOptions = {}
): Promise<VisualizationHomeProbeResult> {
  return getDefaultResolver(options).probe()
}

export function loadVisualizationHomeDashboard(
  options: VisualizationHomeDashboardOptions = {}
): Promise<VisualizationHomeLoadResult> {
  const resolver = getDefaultResolver(options)
  return resolver.load()
}
