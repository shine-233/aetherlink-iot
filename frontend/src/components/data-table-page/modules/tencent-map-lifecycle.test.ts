/*
 * 文件用途：验证数据表腾讯地图在 SDK 可用时的渲染次数与卸载清理。
 * 核心逻辑：用最小 TMap 桩替身挂载组件，断言首屏只渲染一次（不再由 watchEffect 与 onMounted 重复触发），
 *           卸载时关闭信息窗、移除标记图层并销毁地图实例。
 */
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  load: vi.fn(() => Promise.resolve())
}))

vi.mock('@/constants/map-sdk', () => ({
  TENCENT_MAP_SDK_URL: 'https://example.invalid/tmap.js'
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

const createTMapStub = () => {
  const mapInstance = {
    on: vi.fn(),
    setCenter: vi.fn(),
    setZoom: vi.fn(),
    fitBounds: vi.fn(),
    destroy: vi.fn()
  }
  const markerInstance = { on: vi.fn(), setMap: vi.fn() }
  const stub = {
    Map: vi.fn(function MapCtor() {
      return mapInstance
    }),
    MultiMarker: vi.fn(function MultiMarkerCtor() {
      return markerInstance
    }),
    MarkerStyle: vi.fn(function MarkerStyleCtor() {
      return {}
    }),
    LatLng: vi.fn(function LatLngCtor(lat: number, lng: number) {
      return { lat, lng }
    }),
    LatLngBounds: vi.fn(function LatLngBoundsCtor() {
      return { extend: vi.fn(), isEmpty: () => true, contains: () => false }
    }),
    InfoWindow: vi.fn()
  }
  return { stub, mapInstance, markerInstance }
}

describe('data-table TencentMap lifecycle', () => {
  const originalTMap = (globalThis as any).TMap

  afterEach(() => {
    if (originalTMap === undefined) Reflect.deleteProperty(globalThis, 'TMap')
    else (globalThis as any).TMap = originalTMap
    hoisted.load.mockClear()
  })

  it('renders once on mount and destroys map resources on unmount', async () => {
    const { stub, mapInstance, markerInstance } = createTMapStub()
    ;(globalThis as any).TMap = stub

    const wrapper = shallowMount(TencentMap, {
      props: { devices: [{ id: 'd1', name: 'dev', is_online: 1, location: '116.3,39.9' }] },
      attachTo: document.body
    })
    await flushPromises()

    expect(hoisted.load).toHaveBeenCalledTimes(1)
    expect(stub.Map).toHaveBeenCalledTimes(1)
    expect(stub.MultiMarker).toHaveBeenCalledTimes(1)

    wrapper.unmount()

    expect(markerInstance.setMap).toHaveBeenCalledWith(null)
    expect(mapInstance.destroy).toHaveBeenCalledTimes(1)
  })
})
