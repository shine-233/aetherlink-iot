import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { VisualizationRendererRegistry } from './renderer-registry'

const hoisted = vi.hoisted(() => ({
  selectionError: null as null | { code: string; message: string },
  renderer: null as unknown,
  facade: vi.fn(),
  getRendererRegistry: vi.fn()
}))

vi.mock('@/service/visualization-provider/index', async importOriginal => {
  const actual = await importOriginal<typeof import('@/service/visualization-provider/index')>()
  return {
    ...actual,
    getDefaultVisualizationProviderFacade: hoisted.facade
  }
})

vi.mock('./composition', () => ({
  getDefaultVisualizationRendererRegistry: hoisted.getRendererRegistry
}))

import VisualizationProviderFrame from './VisualizationProviderFrame.vue'

const Renderer = defineComponent({
  name: 'TestRenderer',
  props: ['id', 'mode', 'schema'],
  emits: ['host-save-success'],
  setup(props, { emit }) {
    return () => h('button', {
      'data-testid': 'renderer',
      'data-id': props.id,
      'data-mode': props.mode,
      onClick: () => emit('host-save-success', { id: props.id, name: 'Saved' })
    })
  }
})

describe('VisualizationRendererRegistry', () => {
  it('rejects duplicates and preserves the first renderer', () => {
    const registry = new VisualizationRendererRegistry()
    const other = defineComponent(() => () => h('div'))
    expect(registry.register('provider', Renderer)).toBe(true)
    expect(registry.register('provider', other)).toBe(false)
    expect(registry.get('provider')).toBe(Renderer)
  })
})

describe('VisualizationProviderFrame', () => {
  const wrappers: Array<ReturnType<typeof mount>> = []

  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.selectionError = null
    hoisted.renderer = Renderer
    hoisted.facade.mockImplementation(() => ({ selectionError: hoisted.selectionError }))
    hoisted.getRendererRegistry.mockImplementation(() => ({
      get: vi.fn(() => hoisted.renderer)
    }))
  })

  afterEach(() => {
    while (wrappers.length) wrappers.pop()?.unmount()
  })

  const mountFrame = (props: Record<string, unknown> = {}) => {
    const wrapper = mount(VisualizationProviderFrame, {
      props: { id: 'dashboard-1', ...props }
    })
    wrappers.push(wrapper)
    return wrapper
  }

  it('selects provider and renderer, forwards runtime props and composes emitted events', async () => {
    const schema = { id: 'dashboard-1', name: 'D' }
    const wrapper = mountFrame({
      providerId: 'custom',
      mode: 'editor',
      schema,
      context: { available: true, authenticated: true, ownerId: 'owner-1' },
      expectedOwnerId: 'owner-1'
    })

    expect(hoisted.facade).toHaveBeenCalledWith({
      providerId: 'custom',
      context: { available: true, authenticated: true, ownerId: 'owner-1' },
      expectedOwnerId: 'owner-1'
    })
    const renderer = wrapper.get('[data-testid="renderer"]')
    expect(renderer.attributes('data-id')).toBe('dashboard-1')
    expect(renderer.attributes('data-mode')).toBe('editor')
    await renderer.trigger('click')
    expect(wrapper.emitted('hostSaveSuccess')).toEqual([[{ id: 'dashboard-1', name: 'Saved' }]])
  })

  it('defaults unqualified dashboards to the local native provider', () => {
    const wrapper = mountFrame()
    expect(hoisted.facade).toHaveBeenCalledWith({
      providerId: 'native-board',
      context: { available: true, authenticated: true },
      expectedOwnerId: undefined
    })
    expect(wrapper.get('[data-testid="renderer"]').attributes('data-mode')).toBe('viewer')
  })

  it('renders an explicit blocked state on provider selection errors', () => {
    hoisted.selectionError = {
      code: 'provider-unavailable',
      message: 'External visualization provider is unavailable'
    }
    const wrapper = mountFrame({ providerId: 'legacy-thingsvis' })
    expect(wrapper.find('[data-testid="renderer"]').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').attributes('data-provider-error')).toBe('provider-unavailable')
    expect(wrapper.get('[role="alert"]').attributes('data-provider-status')).toBe('blocked-external')
    expect(wrapper.get('[role="alert"]').text()).toBe('External visualization provider is unavailable')
    expect(hoisted.getRendererRegistry).not.toHaveBeenCalled()
  })

  it('fails closed when composition has no renderer', () => {
    hoisted.renderer = undefined
    const wrapper = mountFrame({ providerId: 'custom' })
    expect(wrapper.find('[data-testid="renderer"]').exists()).toBe(false)
  })

  it('does not turn an explicitly empty provider id into the legacy default', () => {
    hoisted.selectionError = { code: 'unknown-provider' }
    const wrapper = mountFrame({ providerId: '' })
    expect(hoisted.facade).toHaveBeenCalledWith(expect.objectContaining({ providerId: '' }))
    expect(wrapper.find('[data-testid="renderer"]').exists()).toBe(false)
  })
})
