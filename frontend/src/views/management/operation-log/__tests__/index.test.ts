/**
 * 文件用途：覆盖 operation-log/index.vue 在 操作审计日志 场景下的筛选参数组装、行展开渲染与分页交互。
 * 核心逻辑：mock operation-log API 后验证首屏拉取、查询/重置/刷新的参数拼装、耗时本地排序和展开行载荷渲染。
 * 关键注意事项：本测试只覆盖前端组件数据流，不证明真实后端租户过滤结果。
 */
import { defineComponent, h, ref } from 'vue'
import type { VNodeChild } from 'vue'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import dayjs from 'dayjs'

const hoisted = vi.hoisted(() => ({
  fetchOperationLogs: vi.fn()
}))

vi.mock('@/service/api/operation-log', () => ({
  fetchOperationLogs: hoisted.fetchOperationLogs
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

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import OperationLogIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof mount> | ReturnType<typeof shallowMount>> = []

function simpleStub(tag = 'div') {
  return defineComponent({
    setup(_, { slots }) {
      return () => h(tag, slots.default ? slots.default() : [])
    }
  })
}

const mountComponent = () => {
  const wrapper = shallowMount(OperationLogIndex, {
    global: {
      stubs: {
        NForm: simpleStub('form'),
        NGrid: simpleStub(),
        NGi: simpleStub(),
        NFormItemGridItem: simpleStub(),
        NInput: simpleStub('input'),
        NSelect: simpleStub(),
        NDatePicker: simpleStub(),
        NSpace: simpleStub(),
        NButton: simpleStub('button'),
        NDataTable: simpleStub()
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) =>
  wrapper.vm.$.setupState as Record<string, any>

const buildRow = (overrides: Record<string, unknown> = {}) => ({
  id: 'log-1',
  ip: '10.0.0.8',
  path: '/api/v1/device/manage',
  user_id: 'user-1',
  name: 'POST',
  created_at: '2026-08-01T10:00:00Z',
  latency: 120,
  request_message: '{"name":"dev-1"}',
  response_message: 'plain-text-response',
  tenant_id: 'tenant-1',
  remark: null,
  username: 'alice',
  email: 'alice@example.com',
  ...overrides
})

describe('management/operation-log/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchOperationLogs.mockResolvedValue({
      data: {
        total: 2,
        list: [buildRow(), buildRow({ id: 'log-2', latency: 30, name: 'GET', username: 'bob' })]
      }
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('assembles default filter params on first load and merges user filters on search', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.fetchOperationLogs).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchOperationLogs).toHaveBeenNthCalledWith(1, {
      page: 1,
      page_size: 10,
      ip: '',
      username: '',
      method: '',
      path: '',
      start_time: '',
      end_time: ''
    })

    const state = getSetupState(wrapper)
    state.queryParams.ip = '10.0.0.'
    state.queryParams.username = 'ali'
    state.queryParams.method = 'POST'
    state.queryParams.path = '/device'

    const startTs = dayjs('2026-08-01T09:30:00').valueOf()
    const endTs = dayjs('2026-08-02T08:15:00').valueOf()
    state.handleRangeChange([startTs, endTs])
    await state.handleSearch()

    expect(hoisted.fetchOperationLogs).toHaveBeenCalledTimes(2)
    expect(hoisted.fetchOperationLogs).toHaveBeenNthCalledWith(2, {
      page: 1,
      page_size: 10,
      ip: '10.0.0.',
      username: 'ali',
      method: 'POST',
      path: '/device',
      // RFC3339（带时区偏移），与后端 time.Time 契约一致
      start_time: dayjs(startTs).format('YYYY-MM-DDTHH:mm:ssZ'),
      end_time: dayjs(endTs).format('YYYY-MM-DDTHH:mm:ssZ')
    })
  })

  it('normalizes empty payloads and exposes empty-state guidance keys', async () => {
    hoisted.fetchOperationLogs.mockResolvedValueOnce({ data: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(state.tableData).toEqual([])
    expect(state.pagination.itemCount).toBe(0)
    expect(state.methodOptions.some((option: { value: string }) => option.value === '')).toBe(true)
  })

  it('renders expand row payload blocks with pretty JSON and raw fallback', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    const columns = state.columns as Array<{ type?: string; key?: string; renderExpand?: (row: any) => VNodeChild }>
    const expandColumn = columns.find(column => column.type === 'expand')
    expect(expandColumn).toBeTruthy()

    const row = buildRow()
    const ExpandHost = defineComponent({
      setup() {
        return () =>
          h(
            'div',
            { class: 'expand-host' },
            (expandColumn?.renderExpand?.(row) ?? []) as VNodeChild
          )
      }
    })
    const hostWrapper = mount(ExpandHost)
    mountedWrappers.push(hostWrapper)

    const blocks = hostWrapper.findAll('.operation-log-message-block')
    expect(blocks).toHaveLength(2)
    // 请求块：JSON 被美化成多行缩进文本
    expect(blocks[0].text()).toContain('custom.management.operationLog.detail.request')
    expect(blocks[0].find('pre').text()).toContain('"name": "dev-1"')
    // 响应块：非 JSON 按原始文本兜底展示
    expect(blocks[1].text()).toContain('custom.management.operationLog.detail.response')
    expect(blocks[1].find('pre').text()).toContain('plain-text-response')
    // 短内容不出现折叠按钮
    expect(hostWrapper.findAll('.operation-log-message-block__toggle')).toHaveLength(0)

    // 方法 tag 颜色映射
    expect(state.methodTagType('GET')).toBe('info')
    expect(state.methodTagType('DELETE')).toBe('error')
    expect(state.methodTagType('unknown')).toBe('default')
  })

  it('handles pagination changes and local latency sorting within the loaded page', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    state.pagination.onChange(3)
    expect(hoisted.fetchOperationLogs).toHaveBeenLastCalledWith(expect.objectContaining({ page: 3 }))

    state.pagination.onUpdatePageSize(20)
    expect(hoisted.fetchOperationLogs).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 20 }))

    // 耗时列在当前页内本地排序：descend -> 120 在前；ascend -> 30 在前
    state.handleSorter({ columnKey: 'latency', order: 'descend' })
    expect(state.tableData.map((row: { latency: number }) => row.latency)).toEqual([120, 30])
    state.handleSorter({ columnKey: 'latency', order: 'ascend' })
    expect(state.tableData.map((row: { latency: number }) => row.latency)).toEqual([30, 120])
    // 取消排序恢复原始顺序
    state.handleSorter({ columnKey: 'latency', order: false })
    expect(state.tableData.map((row: { latency: number }) => row.latency)).toEqual([120, 30])

    // 刷新保持当前页码重新拉取
    state.pagination.page = 2
    await state.handleRefresh()
    expect(hoisted.fetchOperationLogs).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))
    // 新数据到达后排序状态被重置，表格恢复服务端顺序
    expect(state.latencyOrder).toBe(false)

    // 重置清空全部筛选并回到第一页
    state.queryParams.ip = '10.0.0.'
    await state.handleReset()
    expect(state.queryParams.ip).toBe('')
    expect(hoisted.fetchOperationLogs).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1 }))
  })
})
