import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  back: vi.fn(),
  push: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    back: hoisted.back,
    push: hoisted.push
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import LegalPage from '../index.vue'

const SlotStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const ResultStub = defineComponent({
  name: 'NResult',
  props: ['status', 'title', 'description'],
  setup(props, { slots }) {
    return () =>
      h('article', { 'data-status': props.status }, [h('h1', props.title), h('p', props.description), slots.footer?.()])
  }
})

const ButtonStub = defineComponent({
  name: 'NButton',
  emits: ['click'],
  setup(_, { emit, slots }) {
    return () => h('button', { onClick: () => emit('click') }, slots.default?.())
  }
})

const mountedWrappers: Array<ReturnType<typeof mount>> = []

function mountLegalPage(type?: 'terms' | 'privacy') {
  const wrapper = mount(LegalPage, {
    props: type ? { type } : {},
    global: {
      stubs: {
        NCard: SlotStub,
        NSpace: SlotStub,
        NResult: ResultStub,
        NButton: ButtonStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('legal page navigation and visible policy state', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length) mountedWrappers.pop()?.unmount()
  })

  it('renders the terms entry and sends each visible action to the correct destination', async () => {
    const wrapper = mountLegalPage()

    expect(wrapper.get('[data-testid="legal-page"] article').attributes('data-status')).toBe('info')
    expect(wrapper.get('h1').text()).toBe('legal.termsTitle')
    expect(wrapper.get('p').text()).toBe('legal.termsDescription')
    expect(wrapper.get('[data-testid="legal-back"]').text()).toBe('common.back')
    expect(wrapper.get('[data-testid="legal-login"]').text()).toBe('route.login')

    await wrapper.get('[data-testid="legal-back"]').trigger('click')
    await wrapper.get('[data-testid="legal-login"]').trigger('click')

    expect(hoisted.back).toHaveBeenCalledTimes(1)
    expect(hoisted.push).toHaveBeenCalledWith('/login')
  })

  it('updates the visible title and description when the route selects privacy', async () => {
    const wrapper = mountLegalPage('terms')
    expect(wrapper.get('h1').text()).toBe('legal.termsTitle')

    await wrapper.setProps({ type: 'privacy' })

    expect(wrapper.get('h1').text()).toBe('legal.privacyTitle')
    expect(wrapper.get('p').text()).toBe('legal.privacyDescription')
    expect(hoisted.back).not.toHaveBeenCalled()
    expect(hoisted.push).not.toHaveBeenCalled()
  })
})
