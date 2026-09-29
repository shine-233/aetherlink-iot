/**
 * Covers the leak-safety contracts of usePointerDrag and useElementSize:
 * listeners/observers registered during a drag or layout watch are released on unmount.
 */
import { mount } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useElementSize } from '../useElementSize'
import { usePointerDrag } from '../usePointerDrag'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('usePointerDrag', () => {
  it('reports deltas from the start point and stops on mouseup', () => {
    const drag = usePointerDrag()
    const deltas: Array<{ dx: number; dy: number }> = []
    drag.begin({ clientX: 10, clientY: 20 }, (delta) => deltas.push(delta))
    window.dispatchEvent(new MouseEvent('mousemove', { clientX: 15, clientY: 30 }))
    window.dispatchEvent(new MouseEvent('mouseup'))
    window.dispatchEvent(new MouseEvent('mousemove', { clientX: 99, clientY: 99 }))
    expect(deltas).toEqual([{ dx: 5, dy: 10 }])
    expect(drag.isDragging()).toBe(false)
  })

  it('releases window listeners when the owner unmounts mid-drag', () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    const onDelta = vi.fn()
    const wrapper = mount(
      defineComponent({
        setup() {
          usePointerDrag().begin({ clientX: 0, clientY: 0 }, onDelta)
          return () => h('div')
        }
      })
    )
    wrapper.unmount()
    window.dispatchEvent(new MouseEvent('mousemove', { clientX: 5, clientY: 5 }))
    expect(onDelta).not.toHaveBeenCalled()
    const removed = removeSpy.mock.calls.map((call) => call[0])
    expect(removed).toContain('mousemove')
    expect(removed).toContain('mouseup')
  })
})

describe('useElementSize', () => {
  it('disconnects the ResizeObserver on unmount', () => {
    const disconnect = vi.fn()
    const observe = vi.fn()
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe = observe
        disconnect = disconnect
      }
    )
    const wrapper = mount(
      defineComponent({
        setup() {
          const el = ref<HTMLElement | null>(document.createElement('div'))
          useElementSize(el).start()
          return () => h('div')
        }
      })
    )
    expect(observe).toHaveBeenCalledTimes(1)
    wrapper.unmount()
    expect(disconnect).toHaveBeenCalled()
  })

  it('falls back to window resize without ResizeObserver and cleans it up', () => {
    vi.stubGlobal('ResizeObserver', undefined)
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    const el = ref<HTMLElement | null>(document.createElement('div'))
    const { start, stop, size } = useElementSize(el)
    start()
    expect(size.value).toEqual({ width: 0, height: 0 })
    stop()
    expect(removeSpy.mock.calls.some((call) => call[0] === 'resize')).toBe(true)
  })
})
