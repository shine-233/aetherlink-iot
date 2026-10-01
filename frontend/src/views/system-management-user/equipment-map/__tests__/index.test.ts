/**
 * 文件用途: 覆盖设备地图页面的列表/分页数据流契约。
 * 核心逻辑: mock 设备列表与遥测接口后挂载页面，断言查询参数、计数统计、搜索与翻页。
 * 关键注意事项: 纯函数（位置解析/时间格式化）与 AMap 生命周期已下沉到
 *   equipment-map-model / useEquipmentAmap，分别由同名测试文件守护，此处不再重复断言。
 * 重构建议: 可补充“选中设备后自动预览首条并拉取遥测”的交互用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceList: vi.fn(),
  deviceMapTelemetry: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceList: hoisted.deviceList,
  deviceMapTelemetry: hoisted.deviceMapTelemetry
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@vueuse/core', () => ({
  useScriptTag: () => ({ load: vi.fn().mockResolvedValue(undefined) })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

import EquipmentMap from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const passthrough = (name: string) =>
  defineComponent({
    name,
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  })

const mountComponent = () => {
  const wrapper = shallowMount(EquipmentMap, {
    global: {
      stubs: {
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' } },
          setup() {
            return () => h('div')
          }
        }),
        NSpace: passthrough('NSpace'),
        NPagination: passthrough('NPagination')
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
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.deviceMapTelemetry.mockResolvedValue({ data: null, error: null })
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
      data: {
        list: [
          { id: '1', is_online: 1, warn_status: 'Y' },
          { id: '2', is_online: 0, warn_status: 'N' },
          { id: '3', is_online: 1, warn_status: 'N' }
        ],
        total: 3
      },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.onlineCount).toBe(2)
    expect(state.alarmCount).toBe(1)
  })

  it('should search devices', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.query.search = 'test'
    state.searchDevices()
    expect(state.page).toBe(1)
  })

  it('should change page', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.changePage(2)
    expect(state.page).toBe(2)
  })

  it('should compute pageCount', async () => {
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 20 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    expect(getState(wrapper).pageCount).toBe(2)
  })
})
