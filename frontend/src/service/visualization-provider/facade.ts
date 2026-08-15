import type {
  VisualizationError,
  VisualizationProvider,
  VisualizationProviderContext,
  VisualizationProviderId,
  VisualizationResult
} from './contracts'
import { VisualizationProviderRegistry } from './registry'
import { NATIVE_BOARD_PROVIDER_ID } from './provider-ids'

export interface VisualizationProviderSelection {
  providerId?: VisualizationProviderId | null
  context?: Partial<VisualizationProviderContext>
  expectedOwnerId?: string
}

const fail = (error: VisualizationError): VisualizationResult<never> => ({ ok: false, error })

export class VisualizationProviderFacade {
  constructor(
    private readonly provider: VisualizationProvider | null,
    readonly selectionError: VisualizationError | null
  ) {}

  get id(): string | null {
    return this.provider?.id ?? null
  }

  execute<T>(operation: (provider: VisualizationProvider) => Promise<VisualizationResult<T>>): Promise<VisualizationResult<T>> {
    if (this.selectionError) return Promise.resolve(fail(this.selectionError))
    if (!this.provider) {
      return Promise.resolve(fail({ code: 'unknown-provider', message: 'Visualization provider is not selected' }))
    }
    return operation(this.provider).catch(cause =>
      fail({ code: 'provider-failure', message: 'Visualization provider operation failed', cause })
    )
  }
}

export function createVisualizationProviderFacade(
  registry: VisualizationProviderRegistry,
  selection: VisualizationProviderSelection = {}
): VisualizationProviderFacade {
  const providerId = selection.providerId === undefined ? NATIVE_BOARD_PROVIDER_ID : selection.providerId
  if (!providerId) {
    return new VisualizationProviderFacade(null, {
      code: 'unknown-provider',
      message: 'Visualization provider is explicitly empty'
    })
  }

  const provider = registry.get(providerId)
  if (!provider) {
    return new VisualizationProviderFacade(null, {
      code: 'unknown-provider',
      message: `Unknown visualization provider: ${providerId}`
    })
  }

  const context = selection.context
  if (context?.available === false) {
    const externalBlocked = provider.deploymentMode === 'optional-external'
    return new VisualizationProviderFacade(null, {
      code: externalBlocked ? 'external-blocked' : 'provider-unavailable',
      message: externalBlocked
        ? `Optional external visualization provider is disabled: ${providerId}`
        : `Visualization provider is unavailable: ${providerId}`
    })
  }
  if (context?.authenticated === false) {
    return new VisualizationProviderFacade(null, {
      code: 'provider-unauthenticated',
      message: `Visualization provider is unauthenticated: ${providerId}`,
      status: 401
    })
  }
  if (selection.expectedOwnerId && context?.ownerId !== selection.expectedOwnerId) {
    return new VisualizationProviderFacade(null, {
      code: 'ownership-mismatch',
      message: `Visualization provider ownership mismatch: ${providerId}`
    })
  }

  return new VisualizationProviderFacade(provider, null)
}
