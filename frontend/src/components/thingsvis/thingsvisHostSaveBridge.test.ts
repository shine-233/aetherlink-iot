import { describe, expect, it } from 'vitest'
import { buildHostSaveUpdatePayload } from '@/components/thingsvis/thingsvisHostSaveBridge'

describe('thingsvisHostSaveBridge', () => {
  const options = {
    mode: 'editor',
    normalizeCanvasBackground: (background: unknown) =>
      typeof background === 'string' ? { color: background } : (background as Record<string, unknown>),
    normalizeDashboardConfig: <T>(config: T) => JSON.parse(JSON.stringify(config)) as T
  }

  it('keeps the request-save payload contract with config.dataSources', () => {
    const payload = buildHostSaveUpdatePayload(
      {
        config: {
          meta: { name: 'Saved dashboard', thumbnail: 'saved-thumb.png' },
          canvas: { background: '#abcdef' },
          nodes: [{ id: 'node-1', props: { value: '{{ ds.__platform_dev-1__.data.temp }}' } }],
          dataSources: [
            { id: '__platform_dev-1__', type: 'PLATFORM_FIELD', __editorAutoManual: true, mode: 'manual' },
            { id: '__platform_unused__', type: 'PLATFORM_FIELD' },
            { id: 'external-api', type: 'HTTP' }
          ]
        }
      },
      options
    )

    expect(payload).toMatchObject({
      name: 'Saved dashboard',
      thumbnail: 'saved-thumb.png',
      canvasConfig: { background: { color: '#abcdef' } },
      dataSources: [
        { id: '__platform_dev-1__', type: 'PLATFORM_FIELD' },
        { id: 'external-api', type: 'HTTP' }
      ]
    })
  })

  it('accepts the legacy trigger-save payload contract with root dataBindings', () => {
    const payload = buildHostSaveUpdatePayload(
      {
        meta: { name: 'Legacy dashboard' },
        thumbnail: 'legacy-thumb.png',
        canvas: { background: '#112233' },
        nodes: [{ id: 'node-1', props: { value: '{{ ds.__platform_dev-1__.data.temp }}' } }],
        dataBindings: [
          { id: '__platform_dev-1__', type: 'PLATFORM_FIELD', __editorAutoManual: true, mode: 'manual' },
          { id: '__platform_unused__', type: 'PLATFORM_FIELD' },
          { id: 'legacy-http', type: 'HTTP' }
        ]
      },
      options
    )

    expect(payload).toMatchObject({
      name: 'Legacy dashboard',
      thumbnail: 'legacy-thumb.png',
      canvasConfig: { background: { color: '#112233' } },
      dataSources: [
        { id: '__platform_dev-1__', type: 'PLATFORM_FIELD' },
        { id: 'legacy-http', type: 'HTTP' }
      ]
    })
  })
})
