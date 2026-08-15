import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ShortcutsCard from '../shortcuts-card.vue'

const hoisted = vi.hoisted(() => ({
  push: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: hoisted.push
  })
}))

describe('shortcuts-card.vue', () => {
  const defaultProps = {
    label: 'First device',
    icon: 'mdi:access-point-network',
    iconColor: '#409eff',
    route: '/first-device'
  }

  const mountComponent = (props: Partial<typeof defaultProps> = {}) => {
    return mount(ShortcutsCard, {
      props: { ...defaultProps, ...props },
      global: {
        stubs: {
          SvgIcon: {
            props: ['icon'],
            template: '<i data-testid="svg-icon" :data-icon="icon" :data-color="$attrs.style?.color"></i>'
          }
        }
      }
    })
  }

  it('renders an interactive shortcut card with label, icon, route and color', () => {
    const wrapper = mountComponent()

    expect(wrapper.classes()).toEqual(
      expect.arrayContaining(['h-120px', 'flex-col-center', 'cursor-pointer', 'rounded-4px'])
    )
    expect(wrapper.attributes()).toMatchObject({
      role: 'button',
      tabindex: '0'
    })
    expect(wrapper.text()).toContain('First device')
    expect(wrapper.get('[data-testid="svg-icon"]').attributes()).toMatchObject({
      'data-icon': 'mdi:access-point-network',
      'data-color': '#409eff'
    })
  })

  it('routes to the configured product path on click and keyboard activation', async () => {
    hoisted.push.mockClear()
    const wrapper = mountComponent({ route: '/device/manage' })

    await wrapper.trigger('click')
    await wrapper.trigger('keydown.enter')
    await wrapper.trigger('keydown.space')

    expect(hoisted.push).toHaveBeenCalledTimes(3)
    expect(hoisted.push).toHaveBeenCalledWith('/device/manage')
  })

  it('renders a non-interactive card when no route is provided', () => {
    const wrapper = mountComponent({ route: undefined })

    expect(wrapper.classes()).toContain('cursor-default')
    expect(wrapper.attributes('role')).toBeUndefined()
    expect(wrapper.attributes('tabindex')).toBeUndefined()
  })
})
