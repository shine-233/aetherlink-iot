import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import BetterScroll from './better-scroll.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountScroll = (options: Record<string, unknown>) => {
  const wrapper = mount(BetterScroll, {
    props: { options },
    slots: { default: '<div data-test="content">content</div>' }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

afterEach(() => {
  while (mountedWrappers.length > 0) mountedWrappers.pop()?.unmount()
})

describe('BetterScroll native compatibility boundary', () => {
  it('keeps the slot and directional overflow contract', () => {
    const wrapper = mountScroll({ scrollX: true, scrollY: false, bounce: false })
    const container = wrapper.get('[data-test="native-scroll-container"]')

    expect(wrapper.get('[data-test="content"]').text()).toBe('content')
    expect(container.attributes('style')).toContain('overflow-x: auto')
    expect(container.attributes('style')).toContain('overflow-y: hidden')
  })

  it('exposes refresh, destroy, and scrollTo compatibility methods', () => {
    const scrollTo = vi.fn()
    const wrapper = mountScroll({ scrollY: true })
    const element = wrapper.get('[data-test="native-scroll-container"]').element as HTMLElement
    element.scrollTo = scrollTo
    const instance = (wrapper.vm as unknown as { instance: { refresh: () => void; destroy: () => void; scrollTo: (x: number, y: number) => void } }).instance

    expect(() => instance.refresh()).not.toThrow()
    instance.scrollTo(12, 34)
    expect(scrollTo).toHaveBeenCalledWith({ left: 12, top: 34, behavior: 'auto' })

    instance.destroy()
    instance.scrollTo(56, 78)
    expect(scrollTo).toHaveBeenCalledTimes(1)
  })

  it('reacts when scroll direction options change', async () => {
    const wrapper = mountScroll({ scrollX: false, scrollY: true })
    await wrapper.setProps({ options: { scrollX: true, scrollY: false } })

    const style = wrapper.get('[data-test="native-scroll-container"]').attributes('style')
    expect(style).toContain('overflow-x: auto')
    expect(style).toContain('overflow-y: hidden')
  })
})
