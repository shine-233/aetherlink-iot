import { describe, expect, it, vi } from 'vitest'
import { LOCAL_VIEWER_LIMITS } from './types'
import { normalizeLocalDashboard, normalizeLocalViewerFields } from './normalizer'

const dashboard = (widget: Record<string, unknown>) => ({ version: 1, columns: 12, rowHeight: 50, widgets: [widget] })
const textWidget = { id: 'title', x: 0, y: 0, w: 4, h: 1, type: 'text', config: { text: 'Hello' } }

describe('normalizeLocalDashboard', () => {
  it('normalizes a valid version 1 dashboard without mutating the input', () => {
    const input = dashboard(textWidget)
    const snapshot = structuredClone(input)
    const result = normalizeLocalDashboard(input)

    expect(result).toMatchObject({
      ok: true,
      dashboard: {
        version: 1,
        columns: 12,
        rowHeight: 50,
        widgets: [{ id: 'title', type: 'text', originalType: 'text' }]
      }
    })
    expect(input).toEqual(snapshot)
    expect(result.ok && Object.isFrozen(result.dashboard.widgets)).toBe(true)
  })

  it('accepts the explicit legacy layout, i, componentType and properties aliases', () => {
    const result = normalizeLocalDashboard({
      version: 1,
      layout: [{ i: 'legacy', x: 0, y: 0, w: 2, h: 2, componentType: 'line', properties: { categories: ['A'], values: [1] } }]
    })

    expect(result).toMatchObject({ ok: true, dashboard: { widgets: [{ id: 'legacy', type: 'line-chart' }] } })
  })

  it.each([
    ['unknown version', { version: 2, widgets: [] }],
    ['conflicting ids', dashboard({ ...textWidget, i: 'other' })],
    ['duplicate ids', { version: 1, widgets: [textWidget, { ...textWidget }] }],
    ['fractional layout', dashboard({ ...textWidget, x: 0.5 })],
    ['out of bounds layout', dashboard({ ...textWidget, x: 10, w: 4 })],
    ['conflicting collections', { version: 1, widgets: [], layout: [] }],
    ['remote URL', dashboard({ ...textWidget, config: { text: 'https://example.com/a' } })],
    ['script key', dashboard({ ...textWidget, config: { text: 'safe', formatter: 'x' } })],
    ['remote datasource key', dashboard({ ...textWidget, dataSource: 'local' })]
  ])('rejects %s', (_, input) => {
    expect(normalizeLocalDashboard(input).ok).toBe(false)
  })

  it('rejects functions and prototype-bearing objects', () => {
    expect(normalizeLocalDashboard(dashboard({ ...textWidget, config: { text: 'x', extra: () => 1 } })).ok).toBe(false)
    const inherited = Object.create({ version: 1 })
    inherited.widgets = []
    expect(normalizeLocalDashboard(inherited).ok).toBe(false)
  })

  it('rejects widget and chart point limits', () => {
    expect(
      normalizeLocalDashboard({
        version: 1,
        widgets: Array.from({ length: LOCAL_VIEWER_LIMITS.widgets + 1 }, (_, index) => ({ ...textWidget, id: `w-${index}` }))
      }).ok
    ).toBe(false)
    expect(
      normalizeLocalDashboard(
        dashboard({
          ...textWidget,
          type: 'bar-chart',
          config: {
            categories: Array.from({ length: LOCAL_VIEWER_LIMITS.dataPoints + 1 }, () => 'x'),
            values: Array.from({ length: LOCAL_VIEWER_LIMITS.dataPoints + 1 }, () => 1)
          }
        })
      ).ok
    ).toBe(false)
  })

  it('isolates an unknown widget type but discards all of its config', () => {
    const result = normalizeLocalDashboard(dashboard({ ...textWidget, type: 'future-safe', config: { text: 'ignored' } }))
    expect(result).toMatchObject({
      ok: true,
      dashboard: { widgets: [{ type: 'unsupported', originalType: 'future-safe', config: {} }] }
    })
  })

  it('rejects accessor properties without invoking them', () => {
    const getter = vi.fn(() => 'unsafe')
    const config = { text: 'safe' }
    Object.defineProperty(config, 'fallback', { enumerable: true, get: getter })

    expect(normalizeLocalDashboard(dashboard({ ...textWidget, config })).ok).toBe(false)
    expect(getter).not.toHaveBeenCalled()
  })
})

describe('normalizeLocalViewerFields', () => {
  it('copies and freezes bounded scalar and array fields', () => {
    const input = { status: 'online', values: [1, 2] }
    const result = normalizeLocalViewerFields(input)

    expect(result).toMatchObject({ ok: true, fields: input })
    expect(result.ok && Object.isFrozen(result.fields)).toBe(true)
    expect(result.ok && Object.isFrozen(result.fields.values)).toBe(true)
  })

  it.each([
    ['non-plain input', []],
    ['invalid field name', { 'bad field': 1 }],
    ['non-finite number', { value: Number.NaN }],
    ['nested object', { value: { nested: true } }],
    ['too many fields', Object.fromEntries(Array.from({ length: LOCAL_VIEWER_LIMITS.fields + 1 }, (_, index) => [`f${index}`, index]))],
    ['too many points', { values: Array.from({ length: LOCAL_VIEWER_LIMITS.dataPoints + 1 }, () => 1) }]
  ])('rejects %s', (_, input) => {
    expect(normalizeLocalViewerFields(input).ok).toBe(false)
  })

  it('rejects accessor fields without invoking them', () => {
    const getter = vi.fn(() => 'unsafe')
    const input = {}
    Object.defineProperty(input, 'status', { enumerable: true, get: getter })

    expect(normalizeLocalViewerFields(input).ok).toBe(false)
    expect(getter).not.toHaveBeenCalled()
  })
})
