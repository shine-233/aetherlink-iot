/**
 * 文件用途：覆盖 alarm-configuration 在 告警消息管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, reactive, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  alarmHistory: vi.fn(),
  batchActionAlarmHistory: vi.fn(),
  deviceAlarmHistoryPut: vi.fn(),
  currentInstanceProxy: {
    getPlatform: () => false
  }
}))

vi.mock('@/service/api/alarm', () => ({
  alarmHistory: hoisted.alarmHistory,
  batchActionAlarmHistory: hoisted.batchActionAlarmHistory
}))

vi.mock('@/service/api', () => ({
  deviceAlarmHistoryPut: hoisted.deviceAlarmHistoryPut
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue', async importOriginal => {
  const actual = await importOriginal<typeof import('vue')>()
  return {
    ...actual,
    getCurrentInstance: () => ({
      proxy: hoisted.currentInstanceProxy
    })
  }
})

import AlarmConfiguration from '../alarm-configuration.vue'

const ButtonStub = defineComponent({
  name: 'ButtonStub',
  props: {
    disabled: Boolean,
    type: {
      type: String,
      default: ''
    }
  },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          'data-type': props.type,
          onClick: () => emit('click')
        },
        slots.default ? slots.default() : []
      )
  }
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    value: {
      type: [String, Number, null],
      default: ''
    }
  },
  emits: ['update:value'],
  setup() {
    return () => h('div', { class: 'select-stub' })
  }
})

const InputStub = defineComponent({
  name: 'InputStub',
  props: {
    value: {
      type: String,
      default: ''
    }
  },
  emits: ['update:value'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        value: props.value,
        onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value)
      })
  }
})

const ModalStub = defineComponent({
  name: 'ModalStub',
  props: {
    show: Boolean
  },
  emits: ['update:show'],
  setup(props, { slots }) {
    return () => (props.show ? h('div', { class: 'modal-stub' }, slots.default ? slots.default() : []) : null)
  }
})

const DataTableStub = defineComponent({
  name: 'DataTableStub',
  props: {
    data: {
      type: Array,
      default: () => []
    }
  },
  setup(props) {
    return () => h('div', { class: 'data-table-stub', 'data-row-count': String((props.data as any[]).length) })
  }
})

const baseStubs = {
  NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
  'n-form': defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
  NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  'n-form-item': defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NButton: ButtonStub,
  'n-button': ButtonStub,
  NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  'n-card': defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NInput: InputStub,
  'n-input': InputStub,
  NInputNumber: InputStub,
  NSelect: SelectStub,
  'n-select': SelectStub,
  NDatePicker: defineComponent({ setup() { return () => h('div') } }),
  'n-date-picker': defineComponent({ setup() { return () => h('div') } }),
  NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } }),
  NTable: defineComponent({ setup(_, { slots }) { return () => h('table', slots.default ? slots.default() : []) } }),
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NH3: defineComponent({ setup(_, { slots }) { return () => h('h3', slots.default ? slots.default() : []) } }),
  NModal: ModalStub,
  'n-modal': ModalStub,
  'n-data-table': DataTableStub
}

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountAlarmConfiguration = () => {
  const wrapper = shallowMount(AlarmConfiguration, {
    global: {
      stubs: baseStubs
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const buildAlarmRow = (overrides: Record<string, any> = {}) => ({
  id: 'alarm-1',
  name: 'Sensor 1 high',
  content: 'too hot',
  description: 'needs attention',
  alarm_status: 'Y',
  create_at: '2026-06-20T12:00:00Z',
  remark: JSON.stringify({
    alarm_level: 'high',
    alarm_type: 'sensor',
    acknowledged_at: '2026-06-20 12:30:00',
    reset_at: ''
  }),
  alarm_device_list: [{ id: 'device-1', name: 'Cooler A' }],
  ...overrides
})

afterEach(() => {
  while (mountedWrappers.length > 0) {
    mountedWrappers.pop()?.unmount()
  }
})

describe('alarm-configuration.vue', () => {
  beforeEach(() => {
    hoisted.alarmHistory.mockReset()
    hoisted.batchActionAlarmHistory.mockReset()
    hoisted.deviceAlarmHistoryPut.mockReset()

    hoisted.alarmHistory.mockResolvedValue({
      data: {
        total: 1,
        list: [buildAlarmRow()]
      }
    })
    hoisted.batchActionAlarmHistory.mockResolvedValue({
      data: {
        success_count: 1,
        failure_count: 0,
        results: [{ id: 'alarm-1', ok: true }]
      }
    })
    hoisted.deviceAlarmHistoryPut.mockResolvedValue({})
  })

  it('loads alarm history and submits single acknowledge plus reset through the note-capable action API', async () => {
    const wrapper = mountAlarmConfiguration()
    await flushPromises()

    expect(hoisted.alarmHistory).toHaveBeenCalledTimes(1)
    const setupState = getSetupState(wrapper)
    const row = buildAlarmRow()

    await setupState.acknowledgeAlarm(row)
    setupState.singleActionNote = 'owner accepted'
    await setupState.runSingleAlarmAction()
    await flushPromises()

    expect(hoisted.batchActionAlarmHistory).toHaveBeenCalledWith({
      ids: ['alarm-1'],
      action: 'acknowledge',
      note: 'owner accepted'
    })
    expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)

    await setupState.resetAlarm(row)
    await setupState.runSingleAlarmAction()
    await flushPromises()

    expect(hoisted.batchActionAlarmHistory).toHaveBeenLastCalledWith({
      ids: ['alarm-1'],
      action: 'reset'
    })
    expect(hoisted.alarmHistory).toHaveBeenCalledTimes(3)
  })

  it('keeps the latest batch action evidence on the page after a batch acknowledge', async () => {
    hoisted.batchActionAlarmHistory.mockResolvedValueOnce({
      data: {
        success_count: 0,
        failure_count: 1,
        results: [{ id: 'alarm-1', ok: false, error: 'already closed' }]
      }
    })
    const wrapper = mountAlarmConfiguration()
    await flushPromises()
    const setupState = getSetupState(wrapper)

    setupState.selectedAlarmRowKeys = ['alarm-1']
    setupState.acknowledgeCurrentPage()
    setupState.batchActionNote = 'batch note'
    await setupState.runBatchAlarmAction()
    await flushPromises()

    expect(hoisted.batchActionAlarmHistory).toHaveBeenCalledWith({
      ids: ['alarm-1'],
      action: 'acknowledge',
      note: 'batch note'
    })
    expect(setupState.lastBatchActionEvidence).toMatchObject({
      action: 'acknowledge',
      expectedCount: 1,
      successCount: 0,
      failureCount: 1,
      note: 'batch note',
      failedItems: ['alarm-1: already closed'],
      type: 'warning'
    })
  })

  it('opens maintenance modal and blocks empty submit before saving description', async () => {
    const wrapper = mountAlarmConfiguration()
    await flushPromises()
    const setupState = getSetupState(wrapper)
    const row = buildAlarmRow()

    setupState.maintenance(row)
    await flushPromises()

    setupState.description = ''
    await setupState.submitCallback()
    expect(globalThis.$message.error).toHaveBeenCalledWith('common.enterAlarmDesc')
    expect(hoisted.deviceAlarmHistoryPut).toHaveBeenCalledTimes(0)
    expect(setupState.showModal).toBe(true)
    expect(setupState.description).toBe('')

    setupState.description = 'checked and resolved'
    await setupState.submitCallback()
    await flushPromises()

    expect(hoisted.deviceAlarmHistoryPut).toHaveBeenCalledWith({
      id: 'alarm-1',
      description: 'checked and resolved'
    })
    expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)
  })

  it('runs the rendered search action and resets the selected page before querying', async () => {
    const wrapper = mountAlarmConfiguration()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.selectedAlarmRowKeys = ['alarm-1']
    setupState.pagination.page = 3

    setupState.handleSearch()
    await flushPromises()

    expect(setupState.pagination.page).toBe(1)
    expect(setupState.selectedAlarmRowKeys).toEqual([])
    expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)
  })
})
