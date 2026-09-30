/**
 * 文件用途：覆盖 visualization/widget-bundles/index.vue 把 useListPage 的 pagination 绑到
 *   NDataTable 后的分页交互契约（翻页带 page 参数、改页大小回第一页、itemCount 同步 total）。
 * 核心逻辑：mock 部件库列表 API，直接调用绑定给表格的 pagination.onUpdatePage/onUpdatePageSize，
 *   断言下一次请求参数与 pagination 状态；useListPage 自身的过期请求丢弃由组件层套件覆盖。
 * 关键注意事项：本套件只证明页面与 useListPage 的分页绑定正确，不证明后端分页语义。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getWidgetBundlesList: vi.fn()
}))

vi.mock('@/service/api', () => ({
  getWidgetBundlesList: hoisted.getWidgetBundlesList
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
    NInput: stub('NInput', 'input'),
    NPopconfirm: stub('NPopconfirm'),
    NSpace: stub('NSpace'),
    useMessage: () => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() })
  }
})

import WidgetBundles from '../index.vue'

const mountedWrappers: Array<VueWrapper> = []

function mountPage() {
  const wrapper = shallowMount(WidgetBundles)
  mountedWrappers.push(wrapper)
  return wrapper
}

function getSetupState(wrapper: VueWrapper) {
  return wrapper.vm.$.setupState as unknown as Record<string, any>
}

describe('visualization/widget-bundles/index.vue pagination binding', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getWidgetBundlesList.mockResolvedValue({
      data: { total: 35, list: [] },
      error: null
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('syncs itemCount from the total and requests page 2 when the table page changes', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.pagination.itemCount).toBe(35)
    expect(state.pagination.page).toBe(1)

    state.pagination.onUpdatePage(2)
    await flushPromises()

    expect(hoisted.getWidgetBundlesList).toHaveBeenLastCalledWith({
      page: 2,
      page_size: 10,
      search: undefined
    })
    expect(state.pagination.page).toBe(2)
  })

  it('resets to page 1 with the new page size when the table page size changes', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.pagination.onUpdatePage(3)
    await flushPromises()
    expect(state.pagination.page).toBe(3)

    state.pagination.onUpdatePageSize(50)
    await flushPromises()

    expect(hoisted.getWidgetBundlesList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 50,
      search: undefined
    })
    expect(state.pagination.page).toBe(1)
    expect(state.pagination.pageSize).toBe(50)
  })

  it('keeps the current filters (search term) when paginating', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.listFilter.search = 'gauge'
    state.pagination.onUpdatePage(2)
    await flushPromises()

    expect(hoisted.getWidgetBundlesList).toHaveBeenLastCalledWith({
      page: 2,
      page_size: 10,
      search: 'gauge'
    })
  })
})
