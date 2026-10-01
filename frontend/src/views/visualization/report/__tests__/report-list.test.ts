/**
 * 文件用途：覆盖 visualization/report 页面迁移 useListPage 后的列表契约。
 * 核心逻辑：页面两个分页列表（计划列表、运行历史）的状态机收口在
 *   useListPage（@/components/data-table-page/useListPage），本套件验证：
 *   首屏拉取参数、搜索回第一页、翻页保持页大小、失败态 fail-closed（保留当前行 + 错误提示）、
 *   删掉当前页最后一条回退一页、运行历史按选中计划分页拉取。
 * 关键注意事项：mock 报表 API 与轮询组合式（避免真实定时器）；useListPage 用真实实现。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  createReportSchedule: vi.fn(),
  deleteReportSchedule: vi.fn(),
  getReportRun: vi.fn(),
  getReportSchedule: vi.fn(),
  listReportRuns: vi.fn(),
  listReportSchedules: vi.fn(),
  retryReportRun: vi.fn(),
  runReportSchedule: vi.fn(),
  updateReportSchedule: vi.fn(),
  pollOptions: undefined as Record<string, unknown> | undefined,
  messageSuccess: vi.fn(),
  messageWarning: vi.fn(),
  messageError: vi.fn()
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))
vi.mock('@/service/api/report', () => ({
  createReportSchedule: hoisted.createReportSchedule,
  deleteReportSchedule: hoisted.deleteReportSchedule,
  getReportRun: hoisted.getReportRun,
  getReportSchedule: hoisted.getReportSchedule,
  listReportRuns: hoisted.listReportRuns,
  listReportSchedules: hoisted.listReportSchedules,
  retryReportRun: hoisted.retryReportRun,
  runReportSchedule: hoisted.runReportSchedule,
  updateReportSchedule: hoisted.updateReportSchedule
}))
vi.mock('../useSelectedReportRunPoll', () => ({
  useSelectedReportRunPoll: (options: Record<string, unknown>) => {
    hoisted.pollOptions = options
  }
}))
vi.mock('naive-ui', () => {
  const stub = (tag = 'div') =>
    defineComponent({
      props: ['model'],
      emits: ['click', 'positiveClick', 'update:page'],
      setup(props, { slots, emit, expose }) {
        expose({ validate: () => Promise.resolve() })
        return () =>
          h(tag, { onClick: () => emit('click') }, [
            slots.trigger?.(),
            slots.default?.(),
            slots.extra?.(),
            slots.footer?.()
          ])
      }
    })
  return {
    NAlert: stub(),
    NButton: stub('button'),
    NCard: stub(),
    NCheckbox: stub(),
    NDataTable: stub(),
    NDescriptions: stub(),
    NDescriptionsItem: stub(),
    NDrawer: stub(),
    NDrawerContent: stub(),
    NEmpty: stub(),
    NForm: stub(),
    NFormItem: stub(),
    NInput: stub(),
    NInputNumber: stub(),
    NModal: stub(),
    NPagination: stub(),
    NPopconfirm: stub(),
    NSelect: stub(),
    NSpace: stub(),
    NSpin: stub(),
    NSwitch: stub(),
    NTag: stub(),
    useMessage: () => ({
      success: hoisted.messageSuccess,
      warning: hoisted.messageWarning,
      error: hoisted.messageError
    })
  }
})

import ReportPage from '../index.vue'

const makeSchedule = (overrides: Record<string, unknown> = {}) => ({
  id: 'schedule-1',
  name: 'Daily fleet',
  cron_expr: '0 8 * * *',
  timezone: 'UTC',
  recipients: 'ops@example.com',
  device_ids: ['device-1'],
  keys: ['temperature'],
  lookback_hours: 24,
  format: 'csv',
  enabled: true,
  revision: 7,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
  ...overrides
})

const run = {
  run_id: 'run-1',
  schedule_id: 'schedule-1',
  trigger: 'manual',
  overall_status: 'processing',
  generation_status: 'processing',
  delivery_status: 'pending',
  window_start_at: '2026-09-08T00:00:00Z',
  window_end_at: '2026-09-09T00:00:00Z',
  generation_attempts: 1,
  delivery_attempts: 0,
  duplicate_delivery_risk: false,
  created_at: '2026-09-09T00:00:01Z'
}

const mountedWrappers: Array<VueWrapper> = []

function mountPage() {
  const wrapper = shallowMount(ReportPage)
  mountedWrappers.push(wrapper)
  return wrapper
}

function setupState(wrapper: VueWrapper) {
  return wrapper.vm.$.setupState as unknown as Record<string, any>
}

describe('report page list migration (useListPage)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.pollOptions = undefined
    hoisted.listReportSchedules.mockResolvedValue({
      data: { list: [makeSchedule()], total: 1, page: 1, page_size: 10 },
      error: null
    })
    hoisted.listReportRuns.mockResolvedValue({ data: { list: [run], total: 1, page: 1, page_size: 10 }, error: null })
    hoisted.getReportRun.mockResolvedValue({ data: run, error: null })
    hoisted.deleteReportSchedule.mockResolvedValue({ data: null, error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads the first schedule page through useListPage with flat list params', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(hoisted.listReportSchedules).toHaveBeenCalledWith({ page: 1, page_size: 10, search: '' })
    expect(setupState(wrapper).schedules).toHaveLength(1)
    expect(setupState(wrapper).total).toBe(1)
    expect(setupState(wrapper).listLoading).toBe(false)
    expect(setupState(wrapper).listFailed).toBe(false)
  })

  it('searchSchedules trims the input and resets to the first page', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const state = setupState(wrapper)
    await state.changePage(3)
    await flushPromises()
    state.searchInput = '  fleet  '
    await state.searchSchedules()
    await flushPromises()

    expect(hoisted.listReportSchedules).toHaveBeenLastCalledWith({ page: 1, page_size: 10, search: 'fleet' })
  })

  it('changePage keeps the fixed page size while fetching the requested page', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await setupState(wrapper).changePage(2)
    await flushPromises()

    expect(hoisted.listReportSchedules).toHaveBeenLastCalledWith({ page: 2, page_size: 10, search: '' })
  })

  it('keeps failed loads fail-closed: error message plus the current rows snapshot', async () => {
    const wrapper = mountPage()
    await flushPromises()

    hoisted.listReportSchedules.mockRejectedValueOnce(new Error('network down'))
    await setupState(wrapper).changePage(2)
    await flushPromises()

    const state = setupState(wrapper)
    expect(state.listFailed).toBe(true)
    expect(state.schedules).toHaveLength(1)
    expect(hoisted.messageError).toHaveBeenCalledWith('report.message.loadFailed')
  })

  it('steps back a page when deleting the last row of a page beyond the first', async () => {
    const pageOneRows = Array.from({ length: 10 }, (_, index) => makeSchedule({ id: `schedule-${index + 1}` }))
    const lastRow = makeSchedule({ id: 'schedule-last', revision: 3 })
    hoisted.listReportSchedules.mockImplementation(async (params: { page: number }) => ({
      data: { list: params.page === 1 ? pageOneRows : [lastRow], total: 11 },
      error: null
    }))
    const wrapper = mountPage()
    await flushPromises()

    const state = setupState(wrapper)
    await state.changePage(2)
    await flushPromises()
    expect(state.schedules).toHaveLength(1)

    await state.removeSchedule(lastRow)
    await flushPromises()

    expect(hoisted.deleteReportSchedule).toHaveBeenCalledWith('schedule-last', 3)
    expect(hoisted.listReportSchedules).toHaveBeenLastCalledWith({ page: 1, page_size: 10, search: '' })
    expect(state.page).toBe(1)
  })

  it('opening history loads the first run page of the selected schedule', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const state = setupState(wrapper)
    await state.openHistory(makeSchedule())
    await flushPromises()

    expect(hoisted.listReportRuns).toHaveBeenCalledWith('schedule-1', { page: 1, page_size: 10 })
    expect(state.runs).toHaveLength(1)
    expect(state.historyVisible).toBe(true)
  })

  it('changeRunPage fetches the requested run page for the selected schedule', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const state = setupState(wrapper)
    await state.openHistory(makeSchedule())
    await flushPromises()
    await state.changeRunPage(2)
    await flushPromises()

    expect(hoisted.listReportRuns).toHaveBeenLastCalledWith('schedule-1', { page: 2, page_size: 10 })
  })

  it('keeps failed history loads fail-closed: history-failed message plus current rows', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const state = setupState(wrapper)
    await state.openHistory(makeSchedule())
    await flushPromises()
    expect(state.runs).toHaveLength(1)

    hoisted.listReportRuns.mockRejectedValueOnce(new Error('network down'))
    await state.changeRunPage(2)
    await flushPromises()

    expect(state.runs).toHaveLength(1)
    expect(hoisted.messageError).toHaveBeenCalledWith('report.message.historyFailed')
  })

  it('never issues a schedule or history request without selection and keeps polling drawer-gated', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const state = setupState(wrapper)
    await state.loadRuns()
    expect(hoisted.listReportRuns).not.toHaveBeenCalled()
    expect((hoisted.pollOptions?.visible as { value: boolean } | undefined)?.value).toBe(false)
  })
})
