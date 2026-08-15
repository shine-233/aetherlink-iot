/*
 * 文件用途：验证数据表腾讯地图在未配置外部 SDK 时安全降级。
 * 核心逻辑：挂载组件并断言不加载脚本、不访问 TMap、展示本地不可用状态。
 * 关键注意事项：该测试只覆盖离线边界，不复制地图 SDK 的实现细节。
 */
import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  load: vi.fn()
}))

vi.mock('@/constants/map-sdk', () => ({
  TENCENT_MAP_SDK_URL: ''
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@vueuse/core', () => ({
  useScriptTag: () => ({ load: hoisted.load })
}))

vi.mock('@/service/api/system-data', () => ({
  telemetryLatestApi: vi.fn()
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({ info: vi.fn() })
}))

vi.mock('@/utils/common/map-validator', () => ({
  isValidCoordinate: vi.fn(() => true)
}))

import TencentMap from './tencent-map.vue'

describe('data-table TencentMap offline boundary', () => {
  it('does not load or access Tencent Map when no key is configured', async () => {
    const originalTMap = globalThis.TMap
    Reflect.deleteProperty(globalThis, 'TMap')

    const wrapper = shallowMount(TencentMap, {
      props: { devices: [] }
    })
    await flushPromises()

    expect(hoisted.load).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('rdi.map.mapUnavailable')

    wrapper.unmount()
    if (originalTMap !== undefined) globalThis.TMap = originalTMap
  })
})
