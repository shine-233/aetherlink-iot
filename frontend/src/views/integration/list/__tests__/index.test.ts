/**
 * 文件用途：覆盖 integration/list/index.vue 迁移 useListPage 后的列表契约。
 * 核心逻辑：mock 集成与转换器/设备 API，验证首屏拉取参数、搜索回第一页、行内启停原地改写、
 *   删除成功回刷列表。
 * 关键注意事项：本套件只覆盖前端组件行为，不证明后端集成绑定链路。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getIntegrationsList: vi.fn(),
  createIntegration: vi.fn(),
  updateIntegration: vi.fn(),
  deleteIntegration: vi.fn(),
  getDeviceListForSelect: vi.fn(),
  getDataConvertersList: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api', () => ({
  getIntegrationsList: hoisted.getIntegrationsList,
  createIntegration: hoisted.createIntegration,
  updateIntegration: hoisted.updateIntegration,
  deleteIntegration: hoisted.deleteIntegration,
  getDeviceListForSelect: hoisted.getDeviceListForSelect
}))

vi.mock('@/service/api/data-converter', () => ({
  getDataConvertersList: hoisted.getDataConvertersList
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

// 页面模板组件走 unplugin 自动导入（测试环境不生效），用全局 stubs 按标签名拦截；
// 表单弹窗为子组件，shallowMount 会自动 stub。
function tagStub(tag = 'div') {
  return defineComponent({
    name: 'TagStub',
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      return () => h(tag, attrs, slots.default?.())
    }
  })
}

import IntegrationList from '../index.vue'

const integrationFixture = {
  id: 'int-1',
  name: 'OPC UA north',
  connector_type: 'opcua',
  converter_uplink_id: null,
  converter_downlink_id: null,
  config: '{"device_ids":["dev-1"]}',
  enabled: true,
  created_at: '2026-08-01T00:00:00Z'
}

const mountedWrappers: Array<VueWrapper> = []

function mountComponent() {
  const wrapper = shallowMount(IntegrationList, {
    global: {
      stubs: {
        NCard: tagStub(),
        NDataTable: tagStub(),
        NInput: tagStub('input'),
        NSelect: tagStub('select'),
        NModal: tagStub(),
        NButton: tagStub('button'),
        NSpace: tagStub(),
        IntegrationFormModal: tagStub()
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function getSetupState(wrapper: VueWrapper) {
  return wrapper.vm.$.setupState as unknown as Record<string, any>
}

describe('integration/list/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getIntegrationsList.mockResolvedValue({
      data: { total: 1, list: [{ ...integrationFixture }] },
      error: null
    })
    hoisted.getDataConvertersList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.getDeviceListForSelect.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    ;(window as unknown as Record<string, unknown>).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('fetches the first page through useListPage and maps rows', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.getIntegrationsList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      search: undefined,
      connector_type: undefined,
      enabled: undefined
    })
    expect(getSetupState(wrapper).integrations).toHaveLength(1)
    expect(getSetupState(wrapper).pagination.itemCount).toBe(1)
  })

  it('resets to the first page and passes the search term on search', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.filter.search = 'opc'
    await state.handleSearch()
    await flushPromises()

    expect(hoisted.getIntegrationsList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 10,
      search: 'opc',
      connector_type: undefined,
      enabled: undefined
    })
  })

  it('maps the enabled filter string to a boolean request field', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.filter.enabled = 'false'
    await state.handleSearch()
    await flushPromises()

    expect(hoisted.getIntegrationsList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 10,
      search: undefined,
      connector_type: undefined,
      enabled: false
    })
  })

  it('toggles a row in place without refetching the list', async () => {
    hoisted.updateIntegration.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    const row = state.integrations[0]
    await state.handleToggle(row, false)
    await flushPromises()

    expect(hoisted.updateIntegration).toHaveBeenCalledWith({ id: 'int-1', enabled: false })
    expect(row.enabled).toBe(false)
    expect(hoisted.getIntegrationsList).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalled()
  })

  it('reloads the list after a successful delete', async () => {
    hoisted.deleteIntegration.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    await state.handleDelete(state.integrations[0])
    await flushPromises()

    expect(hoisted.deleteIntegration).toHaveBeenCalledWith('int-1')
    expect(hoisted.getIntegrationsList).toHaveBeenCalledTimes(2)
  })
})
