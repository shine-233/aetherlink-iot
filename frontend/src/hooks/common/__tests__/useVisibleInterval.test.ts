import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'
import { createVisibleInterval } from '../useVisibleInterval'

function setHidden(hidden: boolean) {
  Object.defineProperty(document, 'hidden', { configurable: true, get: () => hidden })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('createVisibleInterval', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setHidden(false)
  })
  afterEach(() => {
    vi.useRealTimers()
    setHidden(false)
  })

  it('polls while visible and stops firing while hidden', () => {
    const task = vi.fn()
    const handle = createVisibleInterval(task, 1000)
    handle.start()
    vi.advanceTimersByTime(3000)
    expect(task).toHaveBeenCalledTimes(3)

    setHidden(true)
    vi.advanceTimersByTime(10000)
    expect(task).toHaveBeenCalledTimes(3)

    setHidden(false)
    // 恢复可见立即补拉一次
    expect(task).toHaveBeenCalledTimes(4)
    vi.advanceTimersByTime(1000)
    expect(task).toHaveBeenCalledTimes(5)
    handle.stop()
  })

  it('does not run on visible when runOnVisible=false and is idempotent', () => {
    const task = vi.fn()
    const handle = createVisibleInterval(task, 1000, { runOnVisible: false })
    handle.start()
    handle.start()
    setHidden(true)
    setHidden(false)
    expect(task).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1000)
    expect(task).toHaveBeenCalledTimes(1)
    handle.stop()
    vi.advanceTimersByTime(5000)
    expect(task).toHaveBeenCalledTimes(1)
  })

  it('does not arm when started hidden and ignores visibility after stop', () => {
    const task = vi.fn()
    setHidden(true)
    const handle = createVisibleInterval(task, 1000)
    handle.start()
    vi.advanceTimersByTime(5000)
    expect(task).not.toHaveBeenCalled()
    handle.stop()
    setHidden(false)
    vi.advanceTimersByTime(5000)
    expect(task).not.toHaveBeenCalled()
  })

  it('stops automatically when the owning scope is disposed', () => {
    const task = vi.fn()
    const scope = effectScope()
    const handle = scope.run(() => createVisibleInterval(task, 1000))!
    handle.start()
    scope.stop()
    expect(handle.isActive()).toBe(false)
    vi.advanceTimersByTime(5000)
    expect(task).not.toHaveBeenCalled()
  })
})
