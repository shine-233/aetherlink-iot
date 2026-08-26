/*
 * 文件用途：验证数据表高德地图在可选外部 SDK 不可用时安全降级。
 * 核心逻辑：覆盖未配置 URL 和脚本加载失败，不访问真实 AMap。
 * 关键注意事项：该测试只验证 fail-closed 控制流，不代表外部地图联调通过。
 */
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  sdkUrl: '',
  load: vi.fn()
}))

vi.mock('@/constants/map-sdk', () => ({
  get AMAP_SDK_URL() {
    return hoisted.sdkUrl
  },
  ensureAmapSecurityConfig: () => {}
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@vueuse/core', () => ({
  useScriptTag: () => ({ load: hoisted.load })
}))

import GaodeMap from './gaode-map.vue'

describe('data-table GaodeMap optional SDK boundary', () => {
  beforeEach(() => {
    hoisted.sdkUrl = ''
    hoisted.load.mockReset()
    Reflect.deleteProperty(globalThis, 'AMap')
  })

  it('does not create a script loader when no key is configured', async () => {
    const wrapper = shallowMount(GaodeMap)
    await flushPromises()

    expect(hoisted.load).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('rdi.map.mapUnavailable')
  })

  it('shows the local unavailable state when SDK loading fails', async () => {
    hoisted.sdkUrl = 'https://example.invalid/amap.js'
    hoisted.load.mockRejectedValueOnce(new Error('offline'))

    const wrapper = shallowMount(GaodeMap)
    await flushPromises()

    expect(hoisted.load).toHaveBeenCalledWith(true)
    expect(wrapper.text()).toContain('rdi.map.mapUnavailable')
  })
})
