import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CapabilityCard from '../capability-card.vue'

const hoisted = vi.hoisted(() => ({
  push: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: hoisted.push
  })
}))

describe('capability-card.vue', () => {
  const defaultProps = {
    name: 'Device onboarding',
    description: 'Create a device and verify first telemetry.',
    actionLabel: 'Open first-device guide',
    route: '/first-device',
    icon: 'mdi:access-point-network'
  }

  const mountComponent = (props: Partial<typeof defaultProps> = {}) => {
    return mount(CapabilityCard, {
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

  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders a clickable IoT capability card with required props', () => {
    const wrapper = mountComponent()

    expect(wrapper.classes()).toEqual(expect.arrayContaining(['min-h-120px', 'cursor-pointer', 'rounded-4px']))
    expect(wrapper.find('p').classes()).toEqual(expect.arrayContaining(['min-h-56px', 'break-words']))
    expect(wrapper.find('h3').text()).toBe('Device onboarding')
    expect(wrapper.text()).toContain('Create a device and verify first telemetry.')
    expect(wrapper.text()).toContain('Open first-device guide')
    expect(wrapper.get('[data-testid="svg-icon"]').attributes('data-icon')).toBe('mdi:access-point-network')
  })

  it('routes internal capability links inside the app', async () => {
    const wrapper = mountComponent({ route: '/device/command-center' })
    await wrapper.trigger('click')
    expect(hoisted.push).toHaveBeenCalledWith('/device/command-center')
  })

  it('opens external capability links in a new browser tab', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = mountComponent({ route: 'https://example.com/docs' })
    await wrapper.trigger('click')
    expect(openSpy).toHaveBeenCalledWith('https://example.com/docs', '_blank')
  })

  it('applies iconColor to SvgIcon style when provided', () => {
    const wrapper = mountComponent({ iconColor: '#42b883' })
    const svgIcon = wrapper.get('[data-testid="svg-icon"]')
    expect(svgIcon.attributes('data-color')).toBe('#42b883')
  })
})
