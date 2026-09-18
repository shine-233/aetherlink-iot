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

  it('normalizes timewindow, responsive, and chart styling configurations', () => {
    const input = {
      version: 1,
      columns: 24,
      rowHeight: 60,
      responsive: true,
      timewindow: {
        type: 'realtime',
        realtime: { interval: '1h' },
        aggregation: { func: 'avg', interval: 60000 }
      },
      widgets: [
        {
          id: 'chart-1',
          x: 0,
          y: 0,
          w: 12,
          h: 6,
          type: 'line-chart',
          config: {
            title: 'CPU Usage',
            categories: ['10:00', '10:01'],
            values: [45.2, 58.1],
            chartStyle: 'area',
            colorTheme: '#3b82f6',
            yMin: 0,
            yMax: 100,
            threshold: { enabled: true, value: 80, label: 'Warning', color: '#ef4444' },
            timewindow: { type: 'realtime', realtime: { interval: '15m' } }
          },
          timewindow: { type: 'realtime', realtime: { interval: '30m' } }
        }
      ]
    }

    const result = normalizeLocalDashboard(input)
    expect(result.ok).toBe(true)
    if (result.ok) {
      expect(result.dashboard.responsive).toBe(true)
      expect(result.dashboard.timewindow?.type).toBe('realtime')
      expect(result.dashboard.widgets[0].timewindow?.type).toBe('realtime')
      const cfg = result.dashboard.widgets[0].config as any
      expect(cfg.chartStyle).toBe('area')
      expect(cfg.colorTheme).toBe('#3b82f6')
      expect(cfg.yMin).toBe(0)
      expect(cfg.yMax).toBe(100)
      expect(cfg.threshold?.enabled).toBe(true)
      expect(cfg.threshold?.value).toBe(80)
    }
  })

  it('normalizes widgets with entityRelation and auto-derives dynamic field keys', () => {
    const input = {
      version: 1,
      columns: 12,
      rowHeight: 50,
      widgets: [
        {
          id: 'rel_metric',
          x: 0,
          y: 0,
          w: 4,
          h: 2,
          type: 'metric',
          config: {
            label: 'Avg Temperature',
            entityRelation: {
              enabled: true,
              rootType: 'device',
              rootId: 'gw-001',
              direction: 'from',
              relationType: 'Contains',
              targetType: 'device',
              targetKey: 'temperature',
              aggregation: 'avg'
            }
          }
        },
        {
          id: 'rel_chart',
          x: 4,
          y: 0,
          w: 8,
          h: 4,
          type: 'line-chart',
          config: {
            title: 'Dynamic Relation Chart',
            entityRelation: {
              enabled: true,
              rootType: 'device',
              rootId: 'gw-001',
              direction: 'from',
              relationType: 'Monitors',
              targetType: 'device',
              targetKey: 'humidity'
            }
          }
        }
      ]
    }

    const result = normalizeLocalDashboard(input)
    expect(result.ok).toBe(true)
    if (result.ok) {
      const metricWidget = result.dashboard.widgets[0]
      const metricCfg = metricWidget.config as any
      expect(metricCfg.label).toBe('Avg Temperature')
      expect(metricCfg.field).toBe('__rel_device_gw-001_from_Contains_temperature')
      expect(metricCfg.entityRelation?.enabled).toBe(true)
      expect(metricCfg.entityRelation?.aggregation).toBe('avg')

      const chartWidget = result.dashboard.widgets[1]
      const chartCfg = chartWidget.config as any
      expect(chartCfg.valueField).toBe('__rel_device_gw-001_from_Monitors_humidity')
      expect(chartCfg.categoryField).toBe('__rel_device_gw-001_from_Monitors_humidity_cats')
      expect(chartCfg.entityRelation?.enabled).toBe(true)
    }
  })

  it('rejects invalid entityRelation fields', () => {
    const badRootType = dashboard({
      ...textWidget,
      config: {
        text: 'test',
        entityRelation: {
          enabled: true,
          rootType: 'invalid_type',
          rootId: '1',
          direction: 'from',
          relationType: 'Contains',
          targetType: 'device',
          targetKey: 'temp'
        }
      }
    })
    expect(normalizeLocalDashboard(badRootType).ok).toBe(false)

    const badDirection = dashboard({
      ...textWidget,
      config: {
        text: 'test',
        entityRelation: {
          enabled: true,
          rootType: 'device',
          rootId: '1',
          direction: 'sideways',
          relationType: 'Contains',
          targetType: 'device',
          targetKey: 'temp'
        }
      }
    })
    expect(normalizeLocalDashboard(badDirection).ok).toBe(false)

    const badAggregation = dashboard({
      ...textWidget,
      config: {
        text: 'test',
        entityRelation: {
          enabled: true,
          rootType: 'device',
          rootId: '1',
          direction: 'from',
          relationType: 'Contains',
          targetType: 'device',
          targetKey: 'temp',
          aggregation: 'median'
        }
      }
    })
    expect(normalizeLocalDashboard(badAggregation).ok).toBe(false)
  })

  it('normalizes valid html widget with html, css and field bindings', () => {
    const htmlWidget = {
      id: 'html-card',
      x: 0,
      y: 0,
      w: 6,
      h: 4,
      type: 'html',
      config: {
        html: '<div class="card"><h3>Title: {{name}}</h3><p>Status: OK</p></div>',
        css: '.card { color: #333; }',
        field: 'name',
        fallback: 'No Name'
      }
    }
    const result = normalizeLocalDashboard(dashboard(htmlWidget))
    expect(result).toMatchObject({
      ok: true,
      dashboard: {
        widgets: [
          {
            id: 'html-card',
            type: 'html',
            originalType: 'html',
            config: {
              html: '<div class="card"><h3>Title: {{name}}</h3><p>Status: OK</p></div>',
              css: '.card { color: #333; }',
              field: 'name',
              fallback: 'No Name'
            }
          }
        ]
      }
    })
  })

  it('accepts html-container and html-card as aliases for html', () => {
    const result = normalizeLocalDashboard(dashboard({
      id: 'h1',
      x: 0,
      y: 0,
      w: 6,
      h: 4,
      type: 'html-container',
      config: { html: '<div>Hello</div>' }
    }))
    expect(result).toMatchObject({
      ok: true,
      dashboard: { widgets: [{ id: 'h1', type: 'html', originalType: 'html-container' }] }
    })
  })

  it('rejects executable javascript schemes in html or css at normalize time', () => {
    const xssHtml = dashboard({
      id: 'h2',
      x: 0,
      y: 0,
      w: 6,
      h: 4,
      type: 'html',
      config: { html: '<a href="javascript:alert(1)">Click</a>' }
    })
    expect(normalizeLocalDashboard(xssHtml).ok).toBe(false)

    const xssCss = dashboard({
      id: 'h3',
      x: 0,
      y: 0,
      w: 6,
      h: 4,
      type: 'html',
      config: { html: '<div>Text</div>', css: 'background: url("javascript:alert(1)")' }
    })
    expect(normalizeLocalDashboard(xssCss).ok).toBe(false)
  })

  it('rejects html strings exceeding htmlLength limit', () => {
    const hugeHtml = dashboard({
      id: 'h4',
      x: 0,
      y: 0,
      w: 6,
      h: 4,
      type: 'html',
      config: { html: 'a'.repeat(LOCAL_VIEWER_LIMITS.htmlLength + 1) }
    })
    expect(normalizeLocalDashboard(hugeHtml).ok).toBe(false)
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
