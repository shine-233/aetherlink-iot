/**
 * 文件用途: tencent-map 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  sdkUrl: 'https://map.qq.com/api/gljs?v=1.exp&key=test',
  load: vi.fn().mockResolvedValue(undefined)
}))

vi.mock('@/constants/map-sdk', () => ({
  get TENCENT_MAP_SDK_URL() {
    return hoisted.sdkUrl
  }
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/map-validator', () => ({
  isValidCoordinate: vi.fn((lat: number, lng: number) => lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180),
  getCoordinateValidationError: vi.fn(() => 'Invalid coordinate')
}))

vi.mock('@vueuse/core', () => ({
  useScriptTag: () => ({ load: hoisted.load })
}))

globalThis.TMap = {
  Map: vi.fn().mockImplementation(() => ({
    on: vi.fn(),
    setCenter: vi.fn()
  })),
  LatLng: vi.fn().mockImplementation((lat: number, lng: number) => ({ lat, lng })),
  MultiMarker: vi.fn().mockImplementation(() => ({ setMap: vi.fn() })),
  MarkerStyle: vi.fn().mockImplementation(() => ({}))
} as any

import Component from '../tencent-map.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      longitude: '116.307484',
      latitude: '39.98412',
      ...props
    },
    global: {
      stubs: {}
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/details/modules/public/tencent-map.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.sdkUrl = 'https://map.qq.com/api/gljs?v=1.exp&key=test'
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('uses a local unavailable state without loading an external script when unconfigured', async () => {
    hoisted.sdkUrl = ''

    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.load).not.toHaveBeenCalled()
    expect(globalThis.TMap.Map).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('rdi.map.mapUnavailable')
  })

  it('initializes map with provided valid coordinate and current-location marker', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const mapContainer = wrapper.element.firstElementChild
    expect(globalThis.TMap.LatLng).toHaveBeenCalledWith(39.98412, 116.307484)
    expect(globalThis.TMap.Map).toHaveBeenCalledWith(
      mapContainer,
      expect.objectContaining({
        center: { lat: 39.98412, lng: 116.307484 },
        zoom: 15,
        maxZoom: 18,
        minZoom: 6
      })
    )
    expect(globalThis.TMap.MultiMarker).toHaveBeenCalledWith(
      expect.objectContaining({
        geometries: [
          expect.objectContaining({
            id: 'current-position',
            styleId: 'current-location',
            position: { lat: 39.98412, lng: 116.307484 }
          })
        ]
      })
    )
  })

  it('keeps the map mount container full size for embedding in device forms', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const mapContainer = wrapper.element.firstElementChild
    expect(wrapper.classes()).toEqual(['relative', 'w-full', 'h-full'])
    expect(mapContainer).toBeInstanceOf(HTMLElement)
    expect(Array.from(mapContainer!.classList)).toEqual(expect.arrayContaining(['w-full', 'h-full']))
    expect(globalThis.TMap.Map).toHaveBeenCalledWith(mapContainer, expect.any(Object))
  })

  it('emits position-selected event from a valid Tencent map click', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const mapInstance = (globalThis.TMap.Map as any).mock.results[0].value
    const clickHandler = mapInstance.on.mock.calls.find(([eventName]: [string]) => eventName === 'click')?.[1]
    clickHandler({
      latLng: {
        getLat: () => 40.123456,
        getLng: () => 116.654321
      }
    })
    expect(wrapper.emitted('position-selected')).toEqual([[{ lat: 40.123456, lng: 116.654321 }]])
    expect(globalThis.TMap.MultiMarker).toHaveBeenLastCalledWith(
      expect.objectContaining({
        geometries: [
          expect.objectContaining({
            id: 'current-position',
            position: { lat: 40.123456, lng: 116.654321 }
          })
        ]
      })
    )
  })

  it('falls back to default center and wider zoom when coordinates are empty', async () => {
    mountComponent({ longitude: '', latitude: '' })
    await flushPromises()
    expect(globalThis.TMap.Map).toHaveBeenCalledWith(
      expect.any(HTMLElement),
      expect.objectContaining({
        center: { lat: 39.98412, lng: 116.307484 },
        zoom: 9
      })
    )
    expect(globalThis.TMap.MultiMarker).toHaveBeenCalledTimes(0)
  })

  it('accepts numeric coordinate props and still renders the current marker', async () => {
    mountComponent({ longitude: 116.307484, latitude: 39.98412 })
    await flushPromises()
    expect(globalThis.TMap.Map).toHaveBeenCalledWith(
      expect.any(HTMLElement),
      expect.objectContaining({
        center: { lat: 39.98412, lng: 116.307484 },
        zoom: 15
      })
    )
    expect(globalThis.TMap.MultiMarker).toHaveBeenCalledWith(
      expect.objectContaining({
        geometries: [expect.objectContaining({ position: { lat: 39.98412, lng: 116.307484 } })]
      })
    )
  })
})
