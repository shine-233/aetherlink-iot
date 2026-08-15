import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AnimatedNumber from './AnimatedNumber.vue'

describe('AnimatedNumber', () => {
  let animationCallback: FrameRequestCallback | undefined

  beforeEach(() => {
    animationCallback = undefined
    vi.spyOn(performance, 'now').mockReturnValue(100)
    vi.stubGlobal('requestAnimationFrame', vi.fn((callback: FrameRequestCallback) => {
      animationCallback = callback
      return 1
    }))
    vi.stubGlobal('cancelAnimationFrame', vi.fn())
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false })))
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('renders the final value immediately and forwards root attributes', () => {
    const wrapper = mount(AnimatedNumber, {
      props: { mNum: 25.5, quantileShow: true },
      attrs: { class: 'reading', 'data-index': '3' }
    })

    expect(wrapper.text()).toBe('25.5')
    expect(wrapper.attributes()).toMatchObject({ class: 'reading', 'data-index': '3' })
  })

  it('animates finite updates and preserves the target precision', async () => {
    const wrapper = mount(AnimatedNumber, { props: { mNum: '1.20' } })

    await wrapper.setProps({ mNum: '3.40' })
    animationCallback?.(250)
    await wrapper.vm.$nextTick()
    expect(Number(wrapper.text())).toBeGreaterThan(1.2)
    expect(wrapper.text()).toMatch(/\.\d{2}$/)

    animationCallback?.(400)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toBe('3.40')
  })

  it('renders non-numeric values directly and skips motion for reduced-motion users', async () => {
    const wrapper = mount(AnimatedNumber, { props: { mNum: 'offline' } })
    expect(wrapper.text()).toBe('offline')

    vi.mocked(matchMedia).mockReturnValue({ matches: true } as MediaQueryList)
    await wrapper.setProps({ mNum: -2.5 })
    expect(wrapper.text()).toBe('-2.5')
    expect(requestAnimationFrame).not.toHaveBeenCalled()
  })

  it('rounds to an integer when quantile display is disabled', () => {
    const wrapper = mount(AnimatedNumber, { props: { mNum: 12.6, quantileShow: false } })
    expect(wrapper.text()).toBe('13')
  })

  it('cancels an in-flight frame before scheduling a newer update', async () => {
    const wrapper = mount(AnimatedNumber, { props: { mNum: 1 } })
    await wrapper.setProps({ mNum: 2 })
    await wrapper.setProps({ mNum: 3 })
    expect(cancelAnimationFrame).toHaveBeenCalledWith(1)
  })
})
