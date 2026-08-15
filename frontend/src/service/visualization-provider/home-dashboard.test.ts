import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  createVisualizationHomeDashboardResolver,
  type VisualizationHomeDashboardResolverDependencies
} from './home-dashboard'

const dashboard = {
  id: 'home-1',
  name: 'Home dashboard',
  canvasConfig: { mode: 'fit', width: 1280, height: 720 },
  nodes: [{ id: 'node-1', type: 'text' }],
  dataSources: [{ id: 'source-1' }],
  variables: [{ name: 'tenant' }],
  thumbnail: null,
  ignoredProviderField: 'hidden'
}

describe('visualization home dashboard resolver', () => {
  let dependencies: VisualizationHomeDashboardResolverDependencies
  let probe: ReturnType<typeof vi.fn>
  let execute: ReturnType<typeof vi.fn>

  beforeEach(() => {
    probe = vi.fn().mockResolvedValue({ reachable: true, status: 200, dashboard })
    execute = vi.fn().mockResolvedValue({ ok: true, data: dashboard })
    dependencies = {
      provider: { selectionError: null, execute } as VisualizationHomeDashboardResolverDependencies['provider'],
      probe
    }
  })

  it('normalizes probe and provider responses to the small Home interface', async () => {
    const resolver = createVisualizationHomeDashboardResolver(dependencies)

    await expect(resolver.probe()).resolves.toEqual({
      reachable: true,
      status: 200,
      dashboard: {
        id: 'home-1',
        name: 'Home dashboard',
        canvasConfig: { mode: 'fit', width: 1280, height: 720 },
        nodes: [{ id: 'node-1', type: 'text' }],
        dataSources: [{ id: 'source-1' }],
        variables: [{ name: 'tenant' }],
        thumbnail: null
      }
    })
    await expect(resolver.load()).resolves.toMatchObject({ ok: true, data: { id: 'home-1', name: 'Home dashboard' } })
    expect(probe).toHaveBeenCalledTimes(1)
    expect(execute).toHaveBeenCalledTimes(1)
  })

  it('does not probe or call the provider when the optional provider is blocked', async () => {
    const selectionError = {
      code: 'external-blocked' as const,
      message: 'Optional external visualization provider is disabled'
    }
    const blockedDependencies = {
      ...dependencies,
      provider: { ...dependencies.provider, selectionError }
    }
    const resolver = createVisualizationHomeDashboardResolver(blockedDependencies)

    await expect(resolver.probe()).resolves.toEqual({ reachable: false, status: 0, dashboard: null })
    await expect(resolver.load()).resolves.toEqual({ ok: false, error: selectionError })
    expect(probe).not.toHaveBeenCalled()
    expect(execute).not.toHaveBeenCalled()
  })

  it('keeps reachability but rejects an incomplete probe dashboard', async () => {
    probe.mockResolvedValue({ reachable: true, status: 200, dashboard: { id: 'home-1', name: 'Incomplete' } })
    const resolver = createVisualizationHomeDashboardResolver(dependencies)

    await expect(resolver.probe()).resolves.toEqual({ reachable: true, status: 200, dashboard: null })
  })

  it('passes an explicit Native tenant context to the home provider', async () => {
    const getHomeDashboard = vi.fn().mockResolvedValue({ ok: true, data: dashboard })
    execute.mockImplementation((operation: (provider: { getHomeDashboard: typeof getHomeDashboard }) => unknown) =>
      operation({ getHomeDashboard }))
    const resolver = createVisualizationHomeDashboardResolver({
      ...dependencies,
      tenantId: 'tenant-1'
    })

    await expect(resolver.load()).resolves.toMatchObject({ ok: true, data: { id: 'home-1' } })
    expect(getHomeDashboard).toHaveBeenCalledWith({ tenantId: 'tenant-1' })
  })
})
