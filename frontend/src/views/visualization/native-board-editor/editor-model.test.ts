import { describe, expect, it } from 'vitest'
import { LOCAL_VIEWER_LIMITS } from '@/components/local-visualization-viewer/types'
import {
  addWidget,
  loadEditorDashboard,
  removeWidget,
  serializeEditorDashboard,
  updateDashboardResponsive,
  updateDashboardTimewindow,
  updateWidgetConfig,
  updateWidgetLayout,
  updateWidgetTimewindow,
  validateEditorDashboard,
  type EditorDashboard
} from './editor-model'

const emptyDashboard = (): EditorDashboard => ({ version: 1, columns: 24, rowHeight: 60, widgets: [] })
const textWidget = {
  id: 'title',
  x: 0,
  y: 0,
  w: 6,
  h: 2,
  type: 'text' as const,
  config: { text: 'Safe text' }
}

describe('native board editor model', () => {
  it('loads JSON and emits a canonical dashboard', () => {
    const result = loadEditorDashboard(
      JSON.stringify({ version: 1, columns: 12, rowHeight: 50, layout: [{ i: 'legacy', x: 0, y: 0, w: 4, h: 2, componentType: 'text', properties: { text: 'Hello' } }] })
    )

    expect(result).toEqual({
      ok: true,
      dashboard: {
        version: 1,
        columns: 12,
        rowHeight: 50,
        widgets: [{ id: 'legacy', x: 0, y: 0, w: 4, h: 2, type: 'text', config: { text: 'Hello' } }]
      }
    })
  })

  it('round-trips all supported widget types through the save gate', () => {
    const input: EditorDashboard = {
      version: 1,
      columns: 24,
      rowHeight: 60,
      widgets: [
        textWidget,
        { id: 'metric', x: 6, y: 0, w: 6, h: 4, type: 'metric', config: { label: 'Temperature', field: 'temperature', unit: 'C', decimals: 1 } },
        { id: 'line', x: 0, y: 4, w: 12, h: 4, type: 'line-chart', config: { title: 'Line', categories: ['A', 'B'], values: [1, 2] } },
        { id: 'bar', x: 12, y: 4, w: 12, h: 4, type: 'bar-chart', config: { title: 'Bar', categories: ['A'], values: [3], seriesName: 'Series' } },
        { id: 'html', x: 0, y: 8, w: 8, h: 4, type: 'html', config: { html: '<div>HTML Card</div>', css: '.card { color: red; }' } }
      ]
    }

    const saved = serializeEditorDashboard(input)
    expect(saved.ok).toBe(true)
    if (!saved.ok) return
    expect(loadEditorDashboard(saved.config)).toEqual({ ok: true, dashboard: saved.dashboard })
  })

  it.each([
    ['invalid JSON', '{'],
    ['unknown widget', { version: 1, widgets: [{ ...textWidget, type: 'future-widget' }] }],
    ['remote URL', { version: 1, widgets: [{ ...textWidget, config: { text: 'https://example.com' } }] }],
    ['forbidden key', { version: 1, widgets: [{ ...textWidget, config: { text: 'safe', formatter: 'x' } }] }],
    ['out of bounds', { version: 1, columns: 12, widgets: [{ ...textWidget, x: 10, w: 4 }] }],
    ['unsafe field', { version: 1, widgets: [{ ...textWidget, type: 'metric', config: { label: 'M', field: 'bad field' } }] }],
    ['mismatched chart arrays', { version: 1, widgets: [{ ...textWidget, type: 'line-chart', config: { categories: ['A'], values: [1, 2] } }] }],
    ['field-bound chart', { version: 1, widgets: [{ ...textWidget, type: 'bar-chart', config: { categoryField: 'labels', valueField: 'values' } }] }]
  ])('rejects %s', (_, input) => {
    expect(loadEditorDashboard(input).ok).toBe(false)
  })

  it('adds each supported widget with legal unique IDs and bounded layouts', () => {
    let dashboard = emptyDashboard()
    for (const type of ['text', 'metric', 'line-chart', 'bar-chart', 'html'] as const) {
      const result = addWidget(dashboard, type, () => `widget${dashboard.widgets.length}`)
      expect(result.ok).toBe(true)
      if (!result.ok) return
      dashboard = result.dashboard
    }

    expect(dashboard.widgets.map(widget => widget.type)).toEqual(['text', 'metric', 'line-chart', 'bar-chart', 'html'])
    expect(new Set(dashboard.widgets.map(widget => widget.id)).size).toBe(5)
    expect(dashboard.widgets.every(widget => /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/.test(widget.id))).toBe(true)
    expect(validateEditorDashboard(dashboard).ok).toBe(true)
  })

  it('retries duplicate IDs and rejects invalid factories', () => {
    const dashboard: EditorDashboard = { ...emptyDashboard(), widgets: [textWidget] }
    let calls = 0
    const added = addWidget(dashboard, 'metric', () => (calls++ === 0 ? 'title' : 'unique'))
    expect(added.ok && added.dashboard.widgets[1].id).toBe('unique')
    expect(addWidget(dashboard, 'metric', () => 'bad id').ok).toBe(false)
  })

  it('enforces the widget and row limits', () => {
    const full: EditorDashboard = {
      ...emptyDashboard(),
      widgets: Array.from({ length: LOCAL_VIEWER_LIMITS.widgets }, (_, index) => ({ ...textWidget, id: `w${index}` }))
    }
    expect(addWidget(full, 'text').ok).toBe(false)

    const bottom: EditorDashboard = {
      ...emptyDashboard(),
      widgets: [{ ...textWidget, y: LOCAL_VIEWER_LIMITS.rows - 2 }]
    }
    expect(addWidget(bottom, 'metric', () => 'next').ok).toBe(false)
  })

  it('updates and removes widgets only through the normalizer gate', () => {
    const dashboard: EditorDashboard = { ...emptyDashboard(), widgets: [textWidget] }
    const moved = updateWidgetLayout(dashboard, 'title', { x: 4, w: 8 })
    expect(moved.ok && moved.dashboard.widgets[0]).toMatchObject({ x: 4, w: 8 })
    expect(updateWidgetLayout(dashboard, 'title', { x: 23, w: 2 }).ok).toBe(false)

    const configured = updateWidgetConfig(dashboard, 'title', { text: 'Updated', field: 'status' })
    expect(configured.ok && configured.dashboard.widgets[0].config).toEqual({ text: 'Updated', field: 'status' })
    expect(updateWidgetConfig(dashboard, 'title', { text: 'javascript:alert(1)' }).ok).toBe(false)

    const removed = removeWidget(dashboard, 'title')
    expect(removed.ok && removed.dashboard.widgets).toEqual([])
    expect(removeWidget(dashboard, 'missing').ok).toBe(false)
  })

  it('updates dashboard timewindow, responsive mode, and widget-level timewindow', () => {
    const dashboard: EditorDashboard = { ...emptyDashboard(), widgets: [textWidget] }
    const withTw = updateDashboardTimewindow(dashboard, {
      type: 'realtime',
      realtime: { interval: '1h' },
      aggregation: { func: 'avg', interval: 60000 }
    })
    expect(withTw.ok).toBe(true)
    if (withTw.ok) {
      expect(withTw.dashboard.timewindow?.type).toBe('realtime')
      expect(withTw.dashboard.timewindow?.realtime?.interval).toBe('1h')
    }

    const withResponsive = updateDashboardResponsive(dashboard, true)
    expect(withResponsive.ok).toBe(true)
    if (withResponsive.ok) {
      expect(withResponsive.dashboard.responsive).toBe(true)
    }

    const withWidgetTw = updateWidgetTimewindow(dashboard, 'title', {
      type: 'realtime',
      realtime: { interval: '15m' }
    })
    expect(withWidgetTw.ok).toBe(true)
    if (withWidgetTw.ok) {
      expect(withWidgetTw.dashboard.widgets[0].timewindow?.realtime?.interval).toBe('15m')
    }
  })

  it('preserves entityRelation config through load, update and serialization roundtrips', () => {
    const input = {
      version: 1,
      columns: 24,
      rowHeight: 60,
      widgets: [
        {
          id: 'rel_metric',
          x: 0,
          y: 0,
          w: 8,
          h: 4,
          type: 'metric',
          config: {
            label: 'Gateway Child Temp',
            field: '__rel_device_gw-1_from_Contains_temperature',
            entityRelation: {
              enabled: true,
              rootType: 'device',
              rootId: 'gw-1',
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
          x: 8,
          y: 0,
          w: 16,
          h: 6,
          type: 'line-chart',
          config: {
            title: 'Dynamic Relation Chart',
            entityRelation: {
              enabled: true,
              rootType: 'device',
              rootId: 'gw-1',
              direction: 'from',
              relationType: 'Contains',
              targetType: 'device',
              targetKey: 'humidity'
            }
          }
        }
      ]
    }

    const loaded = loadEditorDashboard(JSON.stringify(input))
    expect(loaded.ok).toBe(true)
    if (!loaded.ok) return

    expect(loaded.dashboard.widgets[0].config.entityRelation?.enabled).toBe(true)
    expect(loaded.dashboard.widgets[0].config.entityRelation?.rootId).toBe('gw-1')
    expect(loaded.dashboard.widgets[1].config.entityRelation?.enabled).toBe(true)

    const saved = serializeEditorDashboard(loaded.dashboard)
    expect(saved.ok).toBe(true)
    if (!saved.ok) return

    const parsed = JSON.parse(saved.config)
    expect(parsed.widgets[0].config.entityRelation.relationType).toBe('Contains')
    expect(parsed.widgets[1].config.entityRelation.targetKey).toBe('humidity')
  })
})
