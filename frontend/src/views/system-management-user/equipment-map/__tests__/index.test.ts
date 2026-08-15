/**
 * 文件用途: 覆盖测试在系统管理用户侧场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceList: vi.fn(),
  deviceMapTelemetry: vi.fn(),
  loadAmap: vi.fn().mockResolvedValue(undefined),
  amapSdkUrl: 'https://webapi.amap.com/maps?v=2.0&key=test'
}))

vi.mock('@/service/api/device', () => ({
  deviceList: hoisted.deviceList,
  deviceMapTelemetry: hoisted.deviceMapTelemetry,
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/constants/map-sdk', () => ({
  get AMAP_SDK_URL() {
    return hoisted.amapSdkUrl
  }
}))

vi.mock('@vueuse/core', () => ({
  useScriptTag: () => ({ load: hoisted.loadAmap })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

import EquipmentMap from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(EquipmentMap, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NEmpty: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSpin: defineComponent({ props: { show: Boolean }, setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NPagination: defineComponent({ props: ['page', 'pageCount'], emits: ['update:page'], setup() { return () => h('div') } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('EquipmentMap', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.amapSdkUrl = 'https://webapi.amap.com/maps?v=2.0&key=test'
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.deviceMapTelemetry.mockResolvedValue({ data: null, error: null })
  })

  it('uses the local fallback without loading an external script when AMap is not configured', async () => {
    hoisted.amapSdkUrl = ''

    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.loadAmap).not.toHaveBeenCalled()
    expect(getState(wrapper).mapError).toBe(true)
    expect(wrapper.text()).toContain('rdi.map.mapUnavailable')
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and fetch devices', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.deviceList).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceList).toHaveBeenCalledWith({
      page: 1,
      page_size: 12,
      search: undefined
    })
    expect(getState(wrapper).devices).toEqual([])
  })

  it('should compute onlineCount and alarmCount', async () => {
    hoisted.deviceList.mockResolvedValue({
      data: { list: [
        { id: '1', is_online: 1, warn_status: 'Y' },
        { id: '2', is_online: 0, warn_status: 'N' },
        { id: '3', is_online: 1, warn_status: 'N' }
      ], total: 3 },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.onlineCount).toBe(2)
    expect(state.alarmCount).toBe(1)
  })

  it('should parse location from JSON string', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const result = state.parseLocation('{"lat": 22.5, "lng": 114.0}')
    expect(result?.lat).toBe(22.5)
    expect(result?.lng).toBe(114.0)
  })

  it('should parse location from comma-separated string', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const result = state.parseLocation('22.5, 114.0')
    expect(result?.lat).toBe(22.5)
    expect(result?.lng).toBe(114.0)
  })

  it('should return null for empty location', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.parseLocation('')).toBeNull()
    expect(state.parseLocation(undefined)).toBeNull()
  })

  it('should format time correctly', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.formatTime(null)).toBe('-')
    expect(state.formatTime(undefined)).toBe('-')
    expect(state.formatTime('2024-01-01T00:00:00Z')).toBe(new Date('2024-01-01T00:00:00Z').toLocaleString())
  })

  it('should format telemetry label and value', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.telemetryLabel({ key: 'temp', label: 'Temperature' })).toBe('Temperature')
    expect(state.telemetryLabel({ key: 'temp' })).toBe('temp')
    expect(state.telemetryValue({ key: 'temp', value: 25, unit: '°C' })).toBe('25 °C')
    expect(state.telemetryValue({ key: 'temp', value: null })).toBe('-')
  })

  it('should search devices', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.query.search = 'test'
    state.searchDevices()
    expect(state.query.page).toBe(1)
  })

  it('should change page', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.changePage(2)
    expect(state.query.page).toBe(2)
  })

  it('should compute pageCount', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 20 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.pageCount).toBe(2)
  })
})
