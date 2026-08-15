/**
 * 文件用途: 覆盖测试在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  rdiSharedWithMeDevices: vi.fn(),
  routerPush: vi.fn(),
  routerBack: vi.fn(),
  windowMessageError: vi.fn()
}))

vi.mock('@/service/api/rdi', () => ({
  rdiSharedWithMeDevices: hoisted.rdiSharedWithMeDevices
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

const routeQuery = { device_id: '' as string | string[] }

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: routeQuery }),
  useRouter: () => ({
    push: hoisted.routerPush,
    back: hoisted.routerBack
  })
}))

import SharedWithMe from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountSharedWithMe = () => {
  const wrapper = shallowMount(SharedWithMe, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] }, loading: Boolean, pagination: { default: null } }, setup() { return () => h('div') } }),
        NDrawer: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NDrawerContent: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NDescriptions: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NDescriptionsItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const mockDeviceRecord = (overrides: Record<string, any> = {}) => ({
  device: { device_id: 'dev-1', device_name: 'Device 1', online: true, pid_number: 'PID001', firmware_version: '1.0', connection_type: 'wifi', config: { sensor_1_lower: 0, sensor_1_upper: 100, sensor_2_lower: 0, sensor_2_upper: 100, switch_1_alarm_mode: 'NO', switch_2_alarm_mode: 'NC', notification_enabled: true } },
  accepted_at: 1718900000,
  ...overrides
})

describe('shared-with-me/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.device_id = ''
    hoisted.rdiSharedWithMeDevices.mockResolvedValue({
      data: { list: [mockDeviceRecord()], total: 1 }
    })
    ;(globalThis as any).$message = { error: hoisted.windowMessageError }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  describe('formatTime', () => {
    it('returns dash for falsy value', () => {
      const wrapper = mountSharedWithMe()
      const state = getSetupState(wrapper)
      expect(state.formatTime(undefined)).toBe('-')
      expect(state.formatTime(0)).toBe('-')
      expect(state.formatTime(null)).toBe('-')
    })

    it('formats timestamp to locale string', () => {
      const wrapper = mountSharedWithMe()
      const state = getSetupState(wrapper)
      const result = state.formatTime(1718900000)
      expect(result).not.toBe('-')
      expect(typeof result).toBe('string')
    })
  })

  describe('showDeviceDetails', () => {
    it('opens detail drawer with selected record', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      const record = mockDeviceRecord()
      state.showDeviceDetails(record)

      expect(state.detailVisible).toBe(true)
      expect(state.selectedRecord).toEqual(record)
    })
  })

  describe('openDeviceDetails', () => {
    it('navigates to device details when device_id exists', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      state.openDeviceDetails(mockDeviceRecord())

      expect(hoisted.routerPush).toHaveBeenCalledWith({
        name: 'device_details',
        query: { d_id: 'dev-1', access: 'shared' }
      })
    })

    it('does not navigate when device_id is missing', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      state.openDeviceDetails(mockDeviceRecord({ device: { device_id: '' } }))

      expect(hoisted.routerPush).toHaveBeenCalledTimes(0)
    })
  })

  describe('goBack', () => {
    it('calls router.back', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      state.goBack()

      expect(hoisted.routerBack).toHaveBeenCalledTimes(1)
    })
  })

  describe('fetchSharedDevices', () => {
    it('fetches devices on mount', async () => {
      mountSharedWithMe()
      await flushPromises()

      expect(hoisted.rdiSharedWithMeDevices).toHaveBeenCalledTimes(1)
      expect(hoisted.rdiSharedWithMeDevices.mock.calls[0][0]).toMatchObject({
        page: 1,
        page_size: 10
      })
    })

    it('populates tableData and pagination on success', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      expect(state.tableData).toHaveLength(1)
      expect(state.pagination.itemCount).toBe(1)
      expect(state.loading).toBe(false)
    })

    it('auto-opens detail when device_id filter returns single result', async () => {
      routeQuery.device_id = 'dev-1'
      hoisted.rdiSharedWithMeDevices.mockResolvedValue({
        data: { list: [mockDeviceRecord()], total: 1 }
      })
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      expect(state.detailVisible).toBe(true)
      expect(state.selectedRecord).toEqual(expect.objectContaining({
        device: expect.objectContaining({ device_id: 'dev-1' })
      }))
    })

    it('does not auto-open detail when multiple results', async () => {
      routeQuery.device_id = 'dev-1'
      hoisted.rdiSharedWithMeDevices.mockResolvedValue({
        data: { list: [mockDeviceRecord(), mockDeviceRecord({ device: { device_id: 'dev-2', device_name: 'Device 2' } })], total: 2 }
      })
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      expect(state.detailVisible).toBe(false)
    })

    it('shows error and clears table on API failure', async () => {
      hoisted.rdiSharedWithMeDevices.mockRejectedValue({ error: { message: 'Not found' } })
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      expect(state.tableData).toHaveLength(0)
      expect(state.pagination.itemCount).toBe(0)
      expect(state.loading).toBe(false)
      expect(hoisted.windowMessageError).toHaveBeenCalledWith('Not found')
    })

    it('shows fallback error message when error has no message', async () => {
      hoisted.rdiSharedWithMeDevices.mockRejectedValue(new Error('network'))
      const wrapper = mountSharedWithMe()
      await flushPromises()

      expect(hoisted.windowMessageError).toHaveBeenCalledWith('network')
    })
  })

  describe('handleSearch', () => {
    it('resets page to 1 and fetches', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      state.queryParams.page = 5
      vi.clearAllMocks()
      hoisted.rdiSharedWithMeDevices.mockResolvedValue({ data: { list: [], total: 0 } })
      state.handleSearch()
      await flushPromises()

      expect(state.queryParams.page).toBe(1)
      expect(hoisted.rdiSharedWithMeDevices).toHaveBeenCalledTimes(1)
    })
  })

  describe('handleReset', () => {
    it('clears filters and searches', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      state.queryParams.device_id = 'dev-1'
      state.queryParams.device_name = 'Device'
      vi.clearAllMocks()
      hoisted.rdiSharedWithMeDevices.mockResolvedValue({ data: { list: [], total: 0 } })
      state.handleReset()
      await flushPromises()

      expect(state.queryParams.device_id).toBe('')
      expect(state.queryParams.device_name).toBe('')
      expect(state.queryParams.page).toBe(1)
      expect(hoisted.rdiSharedWithMeDevices).toHaveBeenCalledTimes(1)
    })
  })

  describe('pagination', () => {
    it('onChange updates page and fetches', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      vi.clearAllMocks()
      hoisted.rdiSharedWithMeDevices.mockResolvedValue({ data: { list: [], total: 0 } })
      state.pagination.onChange(3)
      await flushPromises()

      expect(state.queryParams.page).toBe(3)
      expect(hoisted.rdiSharedWithMeDevices).toHaveBeenCalledTimes(1)
    })

    it('onUpdatePageSize resets to page 1 and fetches', async () => {
      const wrapper = mountSharedWithMe()
      await flushPromises()

      const state = getSetupState(wrapper)
      state.queryParams.page = 4
      vi.clearAllMocks()
      hoisted.rdiSharedWithMeDevices.mockResolvedValue({ data: { list: [], total: 0 } })
      state.pagination.onUpdatePageSize(50)
      await flushPromises()

      expect(state.queryParams.page_size).toBe(50)
      expect(state.queryParams.page).toBe(1)
      expect(hoisted.rdiSharedWithMeDevices).toHaveBeenCalledTimes(1)
    })
  })

  describe('onMounted with route.query.device_id', () => {
    it('reads device_id from route query', async () => {
      routeQuery.device_id = 'route-device'
      hoisted.rdiSharedWithMeDevices.mockResolvedValue({ data: { list: [], total: 0 } })
      mountSharedWithMe()
      await flushPromises()

      expect(hoisted.rdiSharedWithMeDevices.mock.calls[0][0]).toMatchObject({
        device_id: 'route-device'
      })
    })

    it('handles array device_id from route query', async () => {
      routeQuery.device_id = ['arr-device', 'extra']
      hoisted.rdiSharedWithMeDevices.mockResolvedValue({ data: { list: [], total: 0 } })
      mountSharedWithMe()
      await flushPromises()

      expect(hoisted.rdiSharedWithMeDevices.mock.calls[0][0]).toMatchObject({
        device_id: 'arr-device'
      })
    })
  })
})
