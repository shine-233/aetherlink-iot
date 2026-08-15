/**
 * 文件用途：覆盖 telemetry-dead-letter-setting 在系统设置页中的关键行为与请求契约。
 * 核心逻辑：mock dead-letter API 后验证首屏拉取、处理动作、drain 调用和 processing 状态可见性。
 * 关键注意事项：本测试只覆盖前端组件数据流，不证明真实后端 replay/drain 结果。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getTelemetryDeadLetters: vi.fn(),
  updateTelemetryDeadLetterStatus: vi.fn(),
  drainTelemetryDeadLetters: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/telemetry-dead-letter', () => ({
  getTelemetryDeadLetters: hoisted.getTelemetryDeadLetters,
  updateTelemetryDeadLetterStatus: hoisted.updateTelemetryDeadLetterStatus,
  drainTelemetryDeadLetters: hoisted.drainTelemetryDeadLetters
}))

vi.mock('~/packages/hooks', () => ({
  useLoading: (initial = false) => {
    const loading = ref(initial)
    return {
      loading,
      startLoading: () => {
        loading.value = true
      },
      endLoading: () => {
        loading.value = false
      }
    }
  }
}))

import TelemetryDeadLetterSetting from '../telemetry-dead-letter-setting.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

function simpleStub(tag = 'div') {
  return defineComponent({
    setup(_, { slots }) {
      return () => h(tag, slots.default ? slots.default() : [])
    }
  })
}

const mountComponent = () => {
  const wrapper = shallowMount(TelemetryDeadLetterSetting, {
    global: {
      stubs: {
        NForm: simpleStub('form'),
        NGrid: simpleStub(),
        NFormItemGridItem: simpleStub(),
        NSelect: simpleStub(),
        NInput: simpleStub('input'),
        NSpace: simpleStub(),
        NPopconfirm: simpleStub(),
        NDataTable: simpleStub()
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) =>
  wrapper.vm.$.setupState as Record<string, any>

describe('management/setting/components/telemetry-dead-letter-setting.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getTelemetryDeadLetters.mockResolvedValue({
      data: {
        total: 1,
        list: [
          {
            id: 'dead-1',
            device_id: 'device-1',
            tenant_id: 'tenant-1',
            key: 'temperature',
            ts: 1783583400000,
            status: 'processing',
            attempts: 1,
            created_at: '2026-07-10T00:00:00Z',
            updated_at: '2026-07-10T00:00:00Z'
          }
        ]
      }
    })
    hoisted.updateTelemetryDeadLetterStatus.mockResolvedValue({ data: null })
    hoisted.drainTelemetryDeadLetters.mockResolvedValue({
      data: {
        total_ready: 1,
        attempted: 1,
        replayed: 1,
        failed: 0,
        items: []
      }
    })
    ;(window as any).$message = {
      success: hoisted.messageSuccess
    }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads dead-letter rows on mount and exposes processing in status filters', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.getTelemetryDeadLetters).toHaveBeenCalledTimes(1)
    expect(hoisted.getTelemetryDeadLetters).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      tenant_id: '',
      device_id: '',
      key: '',
      status: ''
    })
    expect(state.tableData).toHaveLength(1)
    expect(state.statusOptions.some((option: any) => option.value === 'processing')).toBe(true)
    expect(state.statusTagType('processing')).toBe('primary')
  })

  it('replays a dead-letter row and refreshes the table', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    await state.handleAction('dead-1', 'replay')
    await flushPromises()

    expect(hoisted.updateTelemetryDeadLetterStatus).toHaveBeenCalledWith('dead-1', 'replay')
    expect(hoisted.getTelemetryDeadLetters).toHaveBeenCalledTimes(2)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('Operation submitted')
  })

  it('drains ready rows using current filters and page size', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    state.queryParams.tenant_id = 'tenant-1'
    state.queryParams.device_id = 'device-1'
    state.queryParams.key = 'temperature'
    state.pagination.pageSize = 20

    await state.handleDrain()
    await flushPromises()

    expect(hoisted.drainTelemetryDeadLetters).toHaveBeenCalledWith({
      tenant_id: 'tenant-1',
      device_id: 'device-1',
      key: 'temperature',
      limit: 20
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('Replay complete: attempted 1, replayed 1, failed 0')
    expect(hoisted.getTelemetryDeadLetters).toHaveBeenCalledTimes(2)
  })
})
