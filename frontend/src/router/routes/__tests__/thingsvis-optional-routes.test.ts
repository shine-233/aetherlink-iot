import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'

type RouteTree = {
  name?: unknown
  children?: RouteTree[]
}

type CreateRoutes = typeof import('../index')['createRoutes']

let createRoutes: CreateRoutes

beforeAll(async () => {
  vi.resetModules()
  vi.doUnmock('@/router/routes')
  ;({ createRoutes } = await import('../index'))
}, 15_000)

function flattenRoutes(routes: RouteTree[]): RouteTree[] {
  return routes.flatMap(route => [route, ...flattenRoutes(route.children || [])])
}

function routeNames() {
  const { constantVueRoutes, authRoutes } = createRoutes()
  return flattenRoutes([...(constantVueRoutes as RouteTree[]), ...(authRoutes as RouteTree[])]).map(route =>
    String(route.name)
  )
}

afterEach(() => {
  vi.unstubAllEnvs()
})

describe('optional ThingsVis compatibility routes', () => {
  it('keeps every compatibility page resolvable while selecting Native by default', async () => {
    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', '')

    const names = await routeNames()

    expect(names).toEqual(
      expect.arrayContaining([
        'visualization_thingsvis',
        'visualization_thingsvis-dashboards',
        'visualization_thingsvis-editor',
        'visualization_thingsvis-menu-dashboard',
        'visualization_thingsvis-preview'
      ])
    )
    expect(names).toContain('thingsvis-preview-standalone')
    expect(names).toEqual(
      expect.arrayContaining([
        'visualization_native-boards',
        'visualization_native-board',
        'visualization_native-board-editor'
      ])
    )
  })

  it('restores legacy ThingsVis routes only when explicitly enabled', async () => {
    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', 'Y')

    const names = await routeNames()

    expect(names).toEqual(
      expect.arrayContaining([
        'thingsvis-preview-standalone',
        'visualization_thingsvis',
        'visualization_thingsvis-dashboards',
        'visualization_thingsvis-editor',
        'visualization_thingsvis-menu-dashboard',
        'visualization_thingsvis-preview',
        'visualization_native-boards'
      ])
    )
  })
})
