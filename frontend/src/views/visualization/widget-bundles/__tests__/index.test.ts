/**
 * 文件用途：覆盖 visualization/widget-bundles/index.vue 迁移 useListPage 后的列表契约。
 * 核心逻辑：mock 部件库 API，验证首屏拉取参数（search 为空时传 undefined）、搜索回第一页、
 *   删除成功回刷列表、内置预览打开与种子导入成功后回刷列表。
 * 关键注意事项：本套件只覆盖前端页面行为，不证明后端部件定义校验与种子幂等链路。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getWidgetBundlesList: vi.fn(),
  createWidgetBundle: vi.fn(),
  updateWidgetBundle: vi.fn(),
  deleteWidgetBundle: vi.fn(),
  getBuiltinWidgetBundle: vi.fn(),
  seedBuiltinWidgetBundle: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api', () => ({
  getWidgetBundlesList: hoisted.getWidgetBundlesList,
  createWidgetBundle: hoisted.createWidgetBundle,
  updateWidgetBundle: hoisted.updateWidgetBundle,
  deleteWidgetBundle: hoisted.deleteWidgetBundle,
  getBuiltinWidgetBundle: hoisted.getBuiltinWidgetBundle,
  seedBuiltinWidgetBundle: hoisted.seedBuiltinWidgetBundle
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

vi.mock('naive-ui', () => {
  const stub = (name: string, tag = 'div') =>
    defineComponent({
      name,
      inheritAttrs: false,
      setup(_, { attrs, slots }) {
        return () => h(tag, attrs, slots.default?.())
      }
    })
  return {
    NButton: stub('NButton', 'button'),
    NCard: stub('NCard'),
    NDataTable: stub('NDataTable'),
    NDrawer: stub('NDrawer'),
    NDrawerContent: stub('NDrawerContent'),
    NForm: stub('NForm'),
    NFormItem: stub('NFormItem'),
    NInput: stub('NInput', 'input'),
    NModal: stub('NModal'),
    NPopconfirm: stub('NPopconfirm'),
    NSpace: stub('NSpace'),
    NTag: stub('NTag'),
    useMessage: () => ({ success: hoisted.messageSuccess, error: vi.fn(), info: vi.fn() })
  }
})

import WidgetBundles from '../index.vue'

const bundleFixture = {
  id: 'wb-1',
  name: '工业基础部件库',
  tenant_id: 'tenant-1',
  widgets: '[{"type":"gauge","version":"1"}]',
  description: '内置四部件',
  version: '1.0.0',
  type_key: 'builtin',
  created_at: '2026-08-01T00:00:00Z'
}

const builtinExportFixture = {
  kind: 'widget-bundle',
  name: '内置四部件',
  version: '1.0.0',
  widgets: '[{"type":"gauge"},{"type":"chart"},{"type":"valve"},{"type":"twin3d"}]'
}

const mountedWrappers: Array<VueWrapper> = []

function mountComponent() {
  const wrapper = shallowMount(WidgetBundles)
  mountedWrappers.push(wrapper)
  return wrapper
}

function getSetupState(wrapper: VueWrapper) {
  return wrapper.vm.$.setupState as unknown as Record<string, any>
}

describe('visualization/widget-bundles/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getWidgetBundlesList.mockResolvedValue({
      data: { total: 1, list: [{ ...bundleFixture }] },
      error: null
    })
    hoisted.getBuiltinWidgetBundle.mockResolvedValue({ data: { ...builtinExportFixture }, error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('fetches the first page through useListPage and maps rows', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.getWidgetBundlesList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      search: undefined
    })
    expect(getSetupState(wrapper).bundles).toHaveLength(1)
    expect(getSetupState(wrapper).pagination.itemCount).toBe(1)
  })

  it('resets to the first page and passes the search term on search', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.listFilter.search = 'gauge'
    await state.handleSearch()
    await flushPromises()

    expect(hoisted.getWidgetBundlesList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 10,
      search: 'gauge'
    })
  })

  it('reloads the list after a successful delete', async () => {
    hoisted.deleteWidgetBundle.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    await state.handleDelete(state.bundles[0])
    await flushPromises()

    expect(hoisted.deleteWidgetBundle).toHaveBeenCalledWith('wb-1')
    expect(hoisted.getWidgetBundlesList).toHaveBeenCalledTimes(2)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('page.widgetBundle.deleteSuccess')
  })

  it('opens the builtin preview and refreshes the list after a successful seed', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    await state.openBuiltin()
    expect(state.builtinVisible).toBe(true)
    expect(state.builtinExport).toEqual(builtinExportFixture)

    hoisted.seedBuiltinWidgetBundle.mockResolvedValue({ data: { idempotent: false }, error: null })
    hoisted.getWidgetBundlesList.mockClear()
    await state.handleSeeded()
    await flushPromises()

    expect(hoisted.getWidgetBundlesList).toHaveBeenCalledTimes(1)
  })
})
