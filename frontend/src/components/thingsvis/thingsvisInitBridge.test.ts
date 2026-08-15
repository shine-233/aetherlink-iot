import { describe, expect, it, vi } from 'vitest'

import {
  buildThingsVisInitConfig,
  buildThingsVisInitMessage,
  dashboardDataFromSchema,
  hasCompleteDashboardSchema,
  loadDashboardPayloadForInit
} from './thingsvisInitBridge'

describe('thingsvisInitBridge', () => {
  it('recognizes complete schema payloads and normalizes schema dashboard data', () => {
    const schema = {
      id: 'dash-1',
      name: 'RDI dashboard',
      thumbnail: 'thumb.png',
      canvasConfig: { background: '#fff' },
      nodes: [{ id: 'node-1' }],
      dataSources: [{ id: '__platform_dev-1__' }],
      variables: [{ name: 'site', value: 'north' }]
    }

    expect(hasCompleteDashboardSchema(schema)).toBe(true)
    expect(dashboardDataFromSchema('fallback-id', schema)).toEqual({
      id: 'dash-1',
      name: 'RDI dashboard',
      thumbnail: 'thumb.png',
      canvasConfig: { background: '#fff' },
      nodes: [{ id: 'node-1' }],
      dataSources: [{ id: '__platform_dev-1__' }],
      variables: [{ name: 'site', value: 'north' }]
    })
  })

  it('builds normalized preload payload from schema without fetching', async () => {
    const fetchDashboardWithRetry = vi.fn()
    const sanitizeDataSourcesForHostSave = vi.fn((_mode, _nodes, dataSources) => dataSources)
    const normalizeDashboardConfig = <T>(config: T) => JSON.parse(JSON.stringify(config)) as T

    const payload = await loadDashboardPayloadForInit({
      propsId: 'dashboard-1',
      mode: 'editor',
      schema: {
        name: 'Schema dashboard',
        canvasConfig: { background: '#fff' },
        nodes: [{ id: 'node-1' }],
        dataSources: [{ id: '__platform_dev-1__' }],
        variables: []
      },
      fetchDashboardWithRetry,
      normalizeDashboardConfig,
      sanitizeDataSourcesForHostSave
    })

    expect(fetchDashboardWithRetry).not.toHaveBeenCalled()
    expect(payload).toEqual({
      meta: {
        id: 'dashboard-1',
        name: 'Schema dashboard',
        thumbnail: null
      },
      canvas: { background: '#fff' },
      nodes: [{ id: 'node-1' }],
      dataSources: [{ id: '__platform_dev-1__' }],
      variables: []
    })
  })

  it('falls back to fetch and reports preload failures via injected callbacks', async () => {
    const onPreloadUnavailable = vi.fn()
    const onPreloadError = vi.fn()
    const sanitizeDataSourcesForHostSave = vi.fn((_mode, _nodes, dataSources) => dataSources)
    const normalizeDashboardConfig = <T>(config: T) => config

    const missing = await loadDashboardPayloadForInit({
      propsId: 'dashboard-2',
      mode: 'viewer',
      schema: null,
      fetchDashboardWithRetry: vi.fn().mockResolvedValue({ data: null, error: { status: 404 } }),
      normalizeDashboardConfig,
      sanitizeDataSourcesForHostSave,
      onPreloadUnavailable,
      onPreloadError
    })

    expect(missing).toBeNull()
    expect(onPreloadUnavailable).toHaveBeenCalledWith('dashboard-2', { status: 404 })
    expect(onPreloadError).not.toHaveBeenCalled()

    const crashed = await loadDashboardPayloadForInit({
      propsId: 'dashboard-3',
      mode: 'viewer',
      schema: null,
      fetchDashboardWithRetry: vi.fn().mockRejectedValue(new Error('boom')),
      normalizeDashboardConfig,
      sanitizeDataSourcesForHostSave,
      onPreloadUnavailable,
      onPreloadError
    })

    expect(crashed).toBeNull()
    expect(onPreloadError).toHaveBeenCalled()
  })

  it('builds init config and message payloads', () => {
    const config = buildThingsVisInitConfig({
      token: 'thingsvis-token',
      platformToken: 'platform-token',
      thingsvisApiBaseUrl: 'https://thingsvis.test/api',
      platformApiBaseUrl: 'https://platform.test/api',
      runtimeDeviceId: 'dev-1'
    })

    expect(config).toEqual({
      mode: 'app',
      saveTarget: 'host',
      token: 'thingsvis-token',
      platformToken: 'platform-token',
      thingsvisApiBaseUrl: 'https://thingsvis.test/api',
      platformApiBaseUrl: 'https://platform.test/api',
      deviceId: 'dev-1'
    })

    expect(buildThingsVisInitMessage({ meta: { id: 'dash-1' } }, 250, config)).toEqual({
      type: 'tv:init',
      payload: {
        platformBufferSize: 250,
        data: { meta: { id: 'dash-1' } },
        config
      }
    })
  })
})
