import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/components/common/grid', () => ({
  GridLayoutPlus: defineComponent({
    props: {
      layout: Array,
      config: Object,
      readonly: Boolean,
      showGrid: Boolean,
      showDropZone: Boolean,
      showTitle: Boolean,
      contentPadding: Boolean,
      idKey: String
    },
    setup(props, { slots }) {
      return () => h('div', { class: 'grid-stub' }, props.layout.map((item: unknown) => slots.default?.({ item })))
    }
  })
}))

vi.mock('./LocalEChartsWidget.vue', () => ({
  default: defineComponent({ props: ['type'], template: '<div class="chart-stub">{{ type }}</div>' })
}))

import { GridLayoutPlus } from '@/components/common/grid'
import LocalVisualizationViewer from './LocalVisualizationViewer.vue'

const widgets = [
  { id: 'text', x: 0, y: 0, w: 3, h: 1, type: 'text', config: { text: 'State: {{value}}', field: 'status' } },
  { id: 'metric', x: 3, y: 0, w: 3, h: 1, type: 'metric', config: { label: 'Power', field: 'power', unit: ' W' } },
  { id: 'chart', x: 0, y: 1, w: 6, h: 2, type: 'bar-chart', config: { categories: ['A'], values: [2] } },
  { id: 'future', x: 6, y: 0, w: 2, h: 1, type: 'custom-html', config: { html: '<b>x</b>' } }
]

describe('LocalVisualizationViewer', () => {
  it('passes an immutable viewer contract to GridLayoutPlus and renders controlled widgets', () => {
    const wrapper = mount(LocalVisualizationViewer, {
      props: { dashboard: { version: 1, columns: 12, rowHeight: 48, widgets }, fields: { status: 'online', power: 42 } }
    })
    const grid = wrapper.getComponent(GridLayoutPlus)

    expect(grid.props()).toMatchObject({
      readonly: true,
      showGrid: false,
      showDropZone: false,
      showTitle: false,
      contentPadding: false,
      idKey: 'id',
      config: expect.objectContaining({ colNum: 12, rowHeight: 48, isDraggable: false, isResizable: false, staticGrid: true })
    })
    expect(wrapper.get('[data-widget-id="text"]').text()).toContain('State: online')
    expect(wrapper.get('[data-widget-id="metric"]').text()).toContain('42 W')
    expect(wrapper.get('.chart-stub').text()).toBe('bar-chart')
    expect(wrapper.get('.local-widget-unsupported').text()).toContain('custom-html')
    expect(wrapper.html()).not.toContain('<b>x</b>')
  })

  it('fails closed and does not mount GridLayoutPlus for an invalid dashboard', () => {
    const wrapper = mount(LocalVisualizationViewer, { props: { dashboard: { version: 99, widgets: [] } } })
    expect(wrapper.get('[role="alert"]').text()).toBe('Invalid local dashboard')
    expect(wrapper.findComponent(GridLayoutPlus).exists()).toBe(false)
  })

  it('fails closed when runtime fields exceed their public contract', () => {
    const wrapper = mount(LocalVisualizationViewer, {
      props: {
        dashboard: { version: 1, widgets: widgets.slice(0, 1) },
        fields: { status: Number.POSITIVE_INFINITY }
      }
    })

    expect(wrapper.get('[role="alert"]').text()).toBe('Invalid local viewer fields')
    expect(wrapper.findComponent(GridLayoutPlus).exists()).toBe(false)
  })

  it('shows an actionable empty state instead of a blank board when no widgets are configured', () => {
    const wrapper = mount(LocalVisualizationViewer, {
      props: { dashboard: { version: 1, columns: 24, rowHeight: 60, widgets: [] }, fields: {} }
    })

    expect(wrapper.get('[data-testid="local-viewer-empty"]').text()).toContain('This board has no widgets yet')
    expect(wrapper.get('[data-testid="local-viewer-empty"]').text()).toContain('Add a widget in the board editor')
    expect(wrapper.findComponent(GridLayoutPlus).exists()).toBe(false)
  })
})
