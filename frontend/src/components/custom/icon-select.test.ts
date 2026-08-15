import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import IconSelect from './icon-select.vue'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

const NPopoverStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', [slots.trigger?.(), slots.header?.(), slots.default?.()])
  }
})

const NInputStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.suffix?.())
  }
})

const SvgIconStub = defineComponent({
  props: { icon: String },
  setup(props) {
    return () => h('span', { 'data-icon': props.icon })
  }
})

function mountIconSelect(value = '') {
  return mount(IconSelect, {
    props: {
      value,
      icons: ['mdi:alarm', 'mdi:cloud']
    },
    global: {
      stubs: {
        NPopover: NPopoverStub,
        NInput: NInputStub,
        NEmpty: true,
        SvgIcon: SvgIconStub
      }
    }
  })
}

describe('IconSelect keyboard accessibility', () => {
  it('renders native named buttons with the selected state exposed', () => {
    const wrapper = mountIconSelect('mdi:cloud')
    const options = wrapper.findAll('button.icon-option')

    expect(options).toHaveLength(2)
    expect(options[0].attributes()).toMatchObject({
      type: 'button',
      'aria-label': 'mdi:alarm',
      'aria-pressed': 'false'
    })
    expect(options[1].attributes('aria-pressed')).toBe('true')
    expect(options[1].get('[data-icon="mdi:cloud"]').attributes('aria-hidden')).toBe('true')
  })

  it.each(['Enter', 'Space'] as const)('selects an icon through the native button %s activation', async (key) => {
    const wrapper = mountIconSelect()
    const option = wrapper.findAll('button.icon-option')[1]

    await option.trigger('keydown', { key })
    await option.trigger('click')

    expect(wrapper.emitted('update:value')).toEqual([['mdi:cloud']])
  })
})
