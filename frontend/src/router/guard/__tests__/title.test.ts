/**
 * 文件用途：验证浏览器标题守卫在每次导航后写入标题，且不会累积泄漏的响应式 watcher。
 * 核心逻辑：用最小 router 桩捕获 afterEach 回调，多次触发后断言 document.title 与 effect 数量。
 * 关键注意事项：历史实现在 afterEach 中调用 useTitle(value)，脱离组件作用域时每次导航都会创建一个永不释放的 watch。
 */
import { effectScope, getCurrentScope } from 'vue'
import type { Router } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createDocumentTitleGuard } from '../title'
import { setDocumentTitle } from '../title-helper'

const sysSettingState = vi.hoisted(() => ({ system_name: 'AetherLink IoT' }))

vi.mock('@/store/modules/sys-setting', () => ({
  useSysSettingStore: () => sysSettingState
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

type AfterEachHook = (to: { path: string; meta: Record<string, unknown> }) => void

function installGuard() {
  let hook: AfterEachHook | undefined
  createDocumentTitleGuard({
    afterEach: (fn: AfterEachHook) => {
      hook = fn
    }
  } as unknown as Router)
  if (!hook) throw new Error('afterEach hook was not registered')
  return hook
}

describe('createDocumentTitleGuard', () => {
  beforeEach(() => {
    sysSettingState.system_name = 'AetherLink IoT'
    document.title = ''
  })

  it('writes the resolved title after each navigation', () => {
    const afterEach = installGuard()

    afterEach({ path: '/management/setting', meta: { i18nKey: 'route.management_setting' } })
    expect(document.title).toBe('route.management_setting-AetherLink IoT')

    sysSettingState.system_name = ''
    afterEach({ path: '/home', meta: {} })
    expect(document.title).toBe('title')
  })

  it('does not register reactive effects in the surrounding scope per navigation', () => {
    const scope = effectScope()
    scope.run(() => {
      const afterEach = installGuard()
      const current = getCurrentScope() as unknown as { effects: unknown[] }
      const before = current.effects.length

      for (let i = 0; i < 50; i += 1) {
        afterEach({ path: `/device/${i}`, meta: { title: `device-${i}` } })
      }

      expect(current.effects.length).toBe(before)
      expect(document.title).toBe('device-49-AetherLink IoT')
    })
    scope.stop()
  })
})

describe('setDocumentTitle', () => {
  it('only touches document.title when the value changes', () => {
    document.title = 'same'
    const setter = vi.spyOn(document, 'title', 'set')

    setDocumentTitle('same')
    expect(setter).not.toHaveBeenCalled()

    setDocumentTitle('next')
    expect(setter).toHaveBeenCalledWith('next')
    setter.mockRestore()
  })
})
