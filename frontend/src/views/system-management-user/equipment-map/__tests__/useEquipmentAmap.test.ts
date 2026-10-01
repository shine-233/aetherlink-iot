/**
 * 文件用途: 覆盖设备地图的 AMap 生命周期组合函数。
 * 核心逻辑: 挂载一个宿主组件取得组件作用域，断言未配置 SDK 时走本地降级而不加载外部脚本。
 * 关键注意事项: 该契约原本挂在 index.vue 上；地图已下沉到 EquipmentMapSurface 后改在此守护。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  loadAmap: vi.fn().mockResolvedValue(undefined),
  amapSdkUrl: 'https://webapi.amap.com/maps?v=2.0&key=test'
}))

vi.mock('@vueuse/core', () => ({
  useScriptTag: () => ({ load: hoisted.loadAmap })
}))

vi.mock('@/constants/map-sdk', () => ({
  get AMAP_SDK_URL() {
    return hoisted.amapSdkUrl
  },
  ensureAmapSecurityConfig: vi.fn()
}))

import { useEquipmentAmap } from '../useEquipmentAmap'

function mountHost() {
  let api: ReturnType<typeof useEquipmentAmap> | null = null
  const wrapper = mount(
    defineComponent({
      setup() {
        api = useEquipmentAmap()
        // The map container must be attached before initMap runs, otherwise it bails out early.
        return () => h('div', { ref: api!.mapDomRef })
      }
    })
  )
  return { wrapper, state: api! }
}

const mounted: Array<ReturnType<typeof mount>> = []

describe('useEquipmentAmap', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.amapSdkUrl = 'https://webapi.amap.com/maps?v=2.0&key=test'
  })

  afterEach(() => {
    while (mounted.length > 0) mounted.pop()?.unmount()
  })

  it('falls back to the local grid without loading an external script when AMap is not configured', async () => {
    hoisted.amapSdkUrl = ''

    const { wrapper, state } = mountHost()
    mounted.push(wrapper)
    await state.initMap(null)
    await flushPromises()

    expect(hoisted.loadAmap).not.toHaveBeenCalled()
    // 组合式函数对外暴露的是 ref（消费方 EquipmentMapSurface 在脚本里用 .value、模板里自动解包）。
    expect(state.mapError.value).toBe(true)
    expect(state.mapReady.value).toBe(false)
  })
})
