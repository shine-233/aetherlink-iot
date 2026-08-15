import { describe, expect, it, vi } from 'vitest'

import {
  attachPlatformDeviceTemplateAssets,
  buildDeviceWidgetPresets,
  fetchTemplateEntry,
  fetchTemplatePresets,
  loadPlatformDeviceTemplateAssets
} from './thingsvisDeviceTemplateBridge'

describe('thingsvisDeviceTemplateBridge', () => {
  it('builds widget presets from nodes and stored presets', () => {
    const result = buildDeviceWidgetPresets(
      'tpl-1',
      JSON.stringify({
        nodes: [{ id: 'node-a', type: 'value', props: { title: 'Temperature' } }],
        device_widget_presets: {
          favorite: [{ id: 'preset-1', name: 'Stored preset', widget: { type: 'gauge' } }]
        }
      })
    )

    expect(result).toEqual([
      expect.objectContaining({ id: 'tpl-1-web-node-node-a', name: 'Temperature' }),
      expect.objectContaining({ id: 'tpl-1-stored-preset-1', name: 'Stored preset' })
    ])
  })

  it('fetches thing-model presets from the detail payload', async () => {
    const result = await fetchTemplatePresets({
      templateId: 'tpl-1',
      loadTemplate: vi.fn().mockResolvedValue({
        data: {
          web_chart_config: JSON.stringify({
            nodes: [{ id: 'node-a', props: { title: 'Preset A' } }]
          })
        }
      })
    })

    expect(result).toEqual([expect.objectContaining({ id: 'tpl-1-web-node-node-a', name: 'Preset A' })])
  })

  it('fetches a thing-model entry from telemetry/attribute/command/event pages', async () => {
    const result = await fetchTemplateEntry({
      templateId: 'tpl-1',
      pageSize: 1000,
      loadTelemetry: vi.fn().mockResolvedValue({ data: { list: [{ key: 'temp', name: 'Temperature' }] } }),
      loadAttributes: vi.fn().mockResolvedValue({ data: { list: [{ key: 'setpoint', name: 'Setpoint' }] } }),
      loadCommands: vi.fn().mockResolvedValue({ data: { list: [{ key: 'reboot', name: 'Reboot' }] } }),
      loadEvents: vi.fn().mockResolvedValue({ data: { list: [{ key: 'alert', name: 'Alert' }] } }),
      unwrapList: (payload) => (Array.isArray(payload?.list) ? payload.list : [])
    })

    expect(result.fields).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: 'temp', dataType: 'telemetry' }),
        expect.objectContaining({ id: 'setpoint', dataType: 'attribute' }),
        expect.objectContaining({ id: 'reboot', dataType: 'command' }),
        expect.objectContaining({ id: 'alert', dataType: 'event' })
      ])
    )
  })

  it('loads and attaches thing-model assets for platform devices', async () => {
    const assets = await loadPlatformDeviceTemplateAssets({
      devices: [{ templateId: 'tpl-1' }, { templateId: 'tpl-1' }, { templateId: 'tpl-2' }],
      loadTemplatePresets: vi
        .fn()
        .mockResolvedValueOnce([{ id: 'preset-1' }])
        .mockResolvedValueOnce([{ id: 'preset-2' }]),
      loadTemplateEntry: vi
        .fn()
        .mockResolvedValueOnce({ fields: [{ id: 'temp' }] })
        .mockResolvedValueOnce({ fields: [{ id: 'setpoint' }] })
    })

    const device = attachPlatformDeviceTemplateAssets(
      {
        deviceId: 'dev-1',
        templateId: 'tpl-1',
        fields: [],
        presets: []
      },
      assets
    )

    expect(device).toMatchObject({
      templateId: 'tpl-1',
      fields: [{ id: 'temp' }],
      presets: [{ id: 'preset-1' }]
    })
  })

  it('starts presets and entry loading together for each thing model', async () => {
    const pending: Array<() => void> = []
    const createPendingResult = <T>(value: T) =>
      new Promise<T>((resolve) => {
        pending.push(() => resolve(value))
      })
    const loadTemplatePresets = vi.fn((templateId: string) => createPendingResult([{ id: `preset-${templateId}` }]))
    const loadTemplateEntry = vi.fn((templateId: string) =>
      createPendingResult({ fields: [{ id: `field-${templateId}` }] })
    )

    const request = loadPlatformDeviceTemplateAssets({
      devices: [{ templateId: 'tpl-1' }, { templateId: 'tpl-2' }],
      loadTemplatePresets,
      loadTemplateEntry
    })
    await Promise.resolve()

    expect(loadTemplatePresets).toHaveBeenCalledWith('tpl-1')
    expect(loadTemplatePresets).toHaveBeenCalledWith('tpl-2')
    expect(loadTemplateEntry).toHaveBeenCalledWith('tpl-1')
    expect(loadTemplateEntry).toHaveBeenCalledWith('tpl-2')

    pending.forEach((resolve) => resolve())
    await expect(request).resolves.toMatchObject({
      fieldsByTemplateId: expect.any(Map),
      presetsByTemplateId: expect.any(Map)
    })
  })
})
