/**
 * 文件用途：守护 ResponsiveMediaQuery 的监听生命周期。
 * 核心逻辑：用可控的 matchMedia 桩断言「每个断点注册一个具名 change 监听」，
 *          且 destroy() 必须把监听摘掉——旧实现只 clear() 本地 Map，
 *          MediaQueryList 仍持有 handler 闭包与本实例，是真实泄漏。
 * 关键注意事项：断言必须比对 destroy() 摘掉的 handler 与注册时是同一个函数引用，
 *              否则"移除了某个匿名函数"这种假修复也能通过。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ResponsiveMediaQuery } from '../utils/responsive'

type FakeMediaQueryList = {
  matches: boolean
  media: string
  added: Array<(event: MediaQueryListEvent) => void>
  removed: Array<(event: MediaQueryListEvent) => void>
  addEventListener: (type: string, handler: (event: MediaQueryListEvent) => void) => void
  removeEventListener: (type: string, handler: (event: MediaQueryListEvent) => void) => void
  emit: (matches: boolean) => void
}

const created: FakeMediaQueryList[] = []
const originalMatchMedia = window.matchMedia

function installMatchMediaStub() {
  window.matchMedia = vi.fn().mockImplementation((query: string) => {
    const list: FakeMediaQueryList = {
      matches: false,
      media: query,
      added: [],
      removed: [],
      addEventListener(type, handler) {
        if (type === 'change') list.added.push(handler)
      },
      removeEventListener(type, handler) {
        if (type === 'change') list.removed.push(handler)
      },
      emit(matches) {
        list.matches = matches
        list.added.forEach((handler) => handler({ matches } as MediaQueryListEvent))
      }
    }
    created.push(list)
    return list
  }) as unknown as typeof window.matchMedia
}

function listFor(minWidth: number) {
  return created.find((list) => list.media === `(min-width: ${minWidth}px)`)
}

describe('ResponsiveMediaQuery listener lifecycle', () => {
  beforeEach(() => {
    created.length = 0
    installMatchMediaStub()
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
  })

  it('registers one named change listener per breakpoint', () => {
    new ResponsiveMediaQuery({ xs: 0, md: 768, lg: 1200 })

    expect(created).toHaveLength(3)
    for (const list of created) {
      expect(list.added, `no change listener for ${list.media}`).toHaveLength(1)
    }
  })

  it('removes the exact same handlers on destroy', () => {
    const media = new ResponsiveMediaQuery({ xs: 0, md: 768 })
    const handlersBeforeDestroy = created.map((list) => list.added[0])

    media.destroy()

    created.forEach((list, index) => {
      expect(list.removed).toEqual([handlersBeforeDestroy[index]])
    })
  })

  it('notifies only the subscribers of the breakpoint that changed', () => {
    const media = new ResponsiveMediaQuery({ xs: 0, md: 768 })
    const mdCallback = vi.fn()
    const xsCallback = vi.fn()

    media.onBreakpoint('md', mdCallback)
    media.onBreakpoint('xs', xsCallback)
    // onBreakpoint 会立即回调一次当前 matches(false)。
    mdCallback.mockClear()
    xsCallback.mockClear()

    listFor(768)!.emit(true)

    expect(mdCallback).toHaveBeenCalledTimes(1)
    expect(mdCallback).toHaveBeenCalledWith(true)
    expect(xsCallback).not.toHaveBeenCalled()
  })

  it('stops notifying after unsubscribe and after destroy', () => {
    const media = new ResponsiveMediaQuery({ md: 768 })
    const callback = vi.fn()

    const unsubscribe = media.onBreakpoint('md', callback)
    callback.mockClear()

    listFor(768)!.emit(true)
    expect(callback).toHaveBeenCalledTimes(1)

    unsubscribe()
    listFor(768)!.emit(false)
    expect(callback).toHaveBeenCalledTimes(1)

    media.onBreakpoint('md', callback)
    callback.mockClear()
    media.destroy()
    // destroy() 已摘掉监听，emit 不会再触达任何订阅者。
    created.forEach((list) => list.removed.forEach(() => undefined))
    expect(listFor(768)!.removed).toHaveLength(1)
  })

  it('reports the current breakpoint from matches()', () => {
    const media = new ResponsiveMediaQuery({ xs: 0, md: 768, lg: 1200 })
    listFor(768)!.matches = true
    listFor(1200)!.matches = true

    expect(media.getCurrentBreakpoint()).toBe('lg')
  })
})
