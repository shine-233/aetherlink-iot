import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({ loadCount: 0 }))

vi.mock('./icons', () => {
  hoisted.loadCount += 1
  return {
    icons: {
      AlarmOutline: defineComponent(() => () => h('span', { 'data-testid': 'alarm-icon' })),
      CloudOutline: defineComponent(() => () => h('span', { 'data-testid': 'cloud-icon' }))
    }
  }
})

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import IconSelector from './icon-selector.vue'

describe('IconSelector', () => {
  it('keeps the heavy registry lazy until the picker is expanded', async () => {
    const wrapper = mount(IconSelector)

    expect(wrapper.findAll('.icon-cell')).toHaveLength(0)
    expect(hoisted.loadCount).toBe(0)

    await wrapper.get('.icon-picker-btn').trigger('click')
    await flushPromises()

    expect(hoisted.loadCount).toBe(1)
    expect(wrapper.findAll('.icon-cell')).toHaveLength(2)

    await wrapper.get('.icon-picker-btn').trigger('click')
    await wrapper.get('.icon-picker-btn').trigger('click')
    await flushPromises()
    expect(hoisted.loadCount).toBe(1)
  })

  it('resolves a stored default icon without emitting a change', async () => {
    const wrapper = mount(IconSelector, {
      props: { defaultIcon: 'CloudOutline' }
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="cloud-icon"]').exists()).toBe(true)
    expect(wrapper.emitted('iconSelected')).toBeUndefined()
    expect(wrapper.emitted('update:value')).toBeUndefined()
  })

  it('emits both the legacy and template-driven value contracts', async () => {
    const wrapper = mount(IconSelector)
    await wrapper.get('.icon-picker-btn').trigger('click')
    await flushPromises()

    await wrapper.findAll('.icon-cell')[0].trigger('click')

    expect(wrapper.emitted('iconSelected')).toEqual([['AlarmOutline']])
    expect(wrapper.emitted('update:value')).toEqual([['AlarmOutline']])
    expect(wrapper.find('.icon-picker-dialog').exists()).toBe(false)
  })
})
