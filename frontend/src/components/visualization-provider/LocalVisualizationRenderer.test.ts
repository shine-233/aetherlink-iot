import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({ viewerProps: vi.fn() }))

vi.mock('@/components/local-visualization-viewer', () => ({
  LocalVisualizationViewer: defineComponent({
    name: 'LocalVisualizationViewer',
    props: ['dashboard'],
    setup(props) {
      hoisted.viewerProps(props.dashboard)
      return () => h('div', { 'data-testid': 'local-viewer' })
    }
  })
}))

import LocalVisualizationRenderer from './LocalVisualizationRenderer.vue'

const schema = (rendererData?: unknown) => ({
  id: 'dashboard-1',
  name: 'Local dashboard',
  thumbnail: null,
  version: 1,
  canvasConfig: { mode: 'fixed', width: 1920, height: 1080, background: null },
  nodes: [],
  dataSources: [],
  variables: [],
  rendererData,
  published: false,
  publishedAt: null,
  shareToken: null,
  projectId: 'project-1',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z'
})

describe('LocalVisualizationRenderer', () => {
  it('invokes the public local viewer with provider renderer data', () => {
    const rendererData = { version: 1, columns: 24, widgets: [] }
    const wrapper = mount(LocalVisualizationRenderer, {
      props: { id: 'dashboard-1', mode: 'viewer', schema: schema(rendererData) }
    })

    expect(wrapper.find('[data-testid="local-viewer"]').exists()).toBe(true)
    expect(hoisted.viewerProps).toHaveBeenCalledWith(rendererData)
  })

  it('mounts nothing without a validated provider schema', () => {
    const wrapper = mount(LocalVisualizationRenderer, {
      props: { id: 'dashboard-1', mode: 'viewer', schema: null }
    })

    expect(wrapper.find('[data-testid="local-viewer"]').exists()).toBe(false)
  })
})
