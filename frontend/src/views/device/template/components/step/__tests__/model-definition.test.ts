/**
 * 文件用途: 测试模型定义步骤。
 * 核心逻辑: 模拟模型项接口和用户操作，验证新增、编辑、删除和表格刷新。
 * 关键注意事项: 测试要覆盖不同模型类别，避免只验证某一种字段类型。
 * 重构建议: 抽出模型类别用例表，减少每类模型重复测试代码。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  attributesApi: vi.fn(),
  commandsApi: vi.fn(),
  delAttributes: vi.fn(),
  delCommands: vi.fn(),
  delEvents: vi.fn(),
  delTelemetry: vi.fn(),
  eventsApi: vi.fn(),
  telemetryApi: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  attributesApi: hoisted.attributesApi,
  commandsApi: hoisted.commandsApi,
  delAttributes: hoisted.delAttributes,
  delCommands: hoisted.delCommands,
  delEvents: hoisted.delEvents,
  delTelemetry: hoisted.delTelemetry,
  eventsApi: hoisted.eventsApi,
  telemetryApi: hoisted.telemetryApi
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@aetherlink/hooks', async importOriginal => {
  const actual = await importOriginal<typeof import('@aetherlink/hooks')>()

  return {
    ...actual,
    useLoading: (init = false) => {
      const loading = ref(init)
      return {
        loading,
        startLoading: vi.fn(() => {
          loading.value = true
        }),
        endLoading: vi.fn(() => {
          loading.value = false
        })
      }
    }
  }
})

vi.mock('../tableList', () => ({
  attribute: ref([]),
  command: ref([]),
  events: ref([]),
  test: ref([])
}))

import Component from '../model-definition.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { stepCurrent: 2, modalVisible: false, deviceTemplateId: 'tpl-1', ...props },
    global: {
      stubs: {
        NButton: true,
        NModal: true,
        NPopconfirm: true,
        NSpace: true,
        SvgIcon: true,
        NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } }),
        NPagination: defineComponent({ props: { page: { default: 1 } }, emits: ['update:page'], setup() { return () => h('div') } }),
        NTabs: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NTabPane: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        AddEditTest: true,
        AddEditAttributes: true,
        AddEditEvents: true,
        AddEditCommands: true,
        CustomCommands: true,
        CustomControls: true,
        WidgetPresetConfig: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/template/components/step/model-definition.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.telemetryApi.mockResolvedValue({
      data: { list: [{ id: 't1', data_name: 'Temp', read_write_flag: 'W' }], total: 1 }
    })
    hoisted.attributesApi.mockResolvedValue({
      data: { list: [{ id: 'a1', data_name: 'Mode', read_write_flag: 'RW' }], total: 1 }
    })
    hoisted.eventsApi.mockResolvedValue({
      data: { list: [{ id: 'e1', data_name: 'Fault', params: '[{"data_name":"code"}]' }], total: 1 }
    })
    hoisted.commandsApi.mockResolvedValue({
      data: { list: [{ id: 'c1', data_name: 'Reset', params: '[{"data_name":"force"}]' }], total: 1 }
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads thing-model tabs lazily with template-scoped API queries', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const expectedQuery = { page: 1, page_size: 10, device_template_id: 'tpl-1' }
    expect(hoisted.telemetryApi).toHaveBeenCalledWith(expectedQuery)
    expect(hoisted.attributesApi).not.toHaveBeenCalled()
    expect(hoisted.eventsApi).not.toHaveBeenCalled()
    expect(hoisted.commandsApi).not.toHaveBeenCalled()
    expect(state.tabsCurrent).toBe('telemetry')
    expect(state.comList.map((item: any) => item.id)).toEqual(['telemetry', 'attributes', 'events', 'command'])
    expect(state.columnsList.map((item: any) => item.name)).toEqual(['telemetry', 'attributes', 'events', 'command'])
    expect(state.columnsList[0].data).toEqual([
      expect.objectContaining({ id: 't1', data_name: 'Temp', read_write_flag: 'device_template.table_header.writeOnly' })
    ])

    state.checkedTabs('attributes')
    await flushPromises()
    expect(hoisted.attributesApi).toHaveBeenCalledWith(expectedQuery)
    expect(state.columnsList[1].data).toEqual([
      expect.objectContaining({ id: 'a1', data_name: 'Mode', read_write_flag: 'device_template.table_header.readAndWrite' })
    ])

    state.checkedTabs('events')
    await flushPromises()
    expect(hoisted.eventsApi).toHaveBeenCalledWith(expectedQuery)
    expect(state.columnsList[2].data).toEqual([
      expect.objectContaining({ id: 'e1', data_name: 'Fault', params: 'code' })
    ])

    state.checkedTabs('command')
    await flushPromises()
    expect(hoisted.commandsApi).toHaveBeenCalledWith(expectedQuery)
    expect(state.columnsList[3].data).toEqual([
      expect.objectContaining({ id: 'c1', data_name: 'Reset', params: 'force' })
    ])
  })

  it('initializes with telemetry tab', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.tabsCurrent).toBe('telemetry')
  })

  it('has comList with 4 components', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.comList).toHaveLength(4)
  })
})
