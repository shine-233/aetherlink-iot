/**
 * 文件用途：覆盖 ReportHistoryDrawer（从 index.vue 拆出的运行历史抽屉）的展示契约。
 * 核心逻辑：抽屉自身不发请求——历史列表的分页/加载/过期请求丢弃收口在 useReportRuns 的
 *   useListPage；本套件验证：停用计划提示、轮询失败提示（文案 + 稳定 testid）、
 *   历史表绑定与分页阈值（runTotal > REPORT_RUN_PAGE_SIZE 才出现分页）、
 *   刷新/翻页事件回传页面、选中运行详情渲染与 retry 转发。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { DataTableColumns } from 'naive-ui'

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

vi.mock('naive-ui', () => {
  const stub = (name: string, tag = 'div') =>
    defineComponent({
      name,
      props: ['show', 'title', 'loading', 'description', 'columns', 'data', 'page', 'pageSize', 'itemCount'],
      emits: ['click', 'update:page'],
      setup(_props, { slots, emit }) {
        return () => h(tag, { onClick: () => emit('click') }, [slots.default?.(), slots.extra?.(), slots.footer?.()])
      }
    })
  return {
    NAlert: stub('NAlert'),
    NButton: stub('NButton', 'button'),
    NDataTable: stub('NDataTable'),
    NDescriptions: stub('NDescriptions'),
    NDescriptionsItem: stub('NDescriptionsItem'),
    NDrawer: stub('NDrawer'),
    NDrawerContent: stub('NDrawerContent'),
    NEmpty: stub('NEmpty'),
    NPagination: stub('NPagination'),
    NSpace: stub('NSpace'),
    NSpin: stub('NSpin'),
    NTag: stub('NTag')
  }
})

import ReportHistoryDrawer from '../ReportHistoryDrawer.vue'
import ReportRunDetail from '../ReportRunDetail.vue'
import { REPORT_RUN_PAGE_SIZE } from '../report-helpers'

const schedule = {
  id: 'schedule-1',
  name: 'Daily fleet',
  cron_expr: '0 8 * * 1-5',
  timezone: 'UTC',
  recipients: 'ops@example.com',
  device_ids: ['device-1'],
  keys: ['temperature'],
  lookback_hours: 24,
  format: 'csv',
  enabled: true,
  revision: 7,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z'
}

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

const runColumns: DataTableColumns<typeof run> = []

const mountDrawer = (overrides: Record<string, unknown> = {}) =>
  mount(ReportHistoryDrawer, {
    props: {
      show: true,
      schedule,
      historyLoading: false,
      runs: [run],
      runTotal: 1,
      runPage: 1,
      pollFailed: false,
      runColumns,
      selectedRun: null,
      retryingRunId: '',
      ...overrides
    }
  })

describe('report history drawer', () => {
  it('surfaces the disabled-schedule notice only for disabled schedules', () => {
    const disabled = mountDrawer({ schedule: { ...schedule, enabled: false } })
    expect(disabled.text()).toContain('report.message.scheduleDisabled')
    disabled.unmount()

    const enabled = mountDrawer()
    expect(enabled.text()).not.toContain('report.message.scheduleDisabled')
    enabled.unmount()
  })

  it('surfaces a terminal polling failure with the stable test id', () => {
    const wrapper = mountDrawer({ pollFailed: true })
    const alert = wrapper.find('[data-testid="report-poll-failed"]')

    expect(alert.exists()).toBe(true)
    expect(alert.text()).toContain('report.message.pollFailed')
  })

  it('binds the run table and shows pagination only beyond one page', () => {
    const wrapper = mountDrawer({ runTotal: 25, runPage: 2 })
    const table = wrapper.findComponent({ name: 'NDataTable' })
    expect(table.props('columns')).toEqual(runColumns)
    expect(table.props('data')).toEqual([run])

    const pagination = wrapper.findComponent({ name: 'NPagination' })
    expect(pagination.props('page')).toBe(2)
    expect(pagination.props('pageSize')).toBe(REPORT_RUN_PAGE_SIZE)
    expect(pagination.props('itemCount')).toBe(25)
    wrapper.unmount()

    const singlePage = mountDrawer({ runTotal: REPORT_RUN_PAGE_SIZE })
    expect(singlePage.findComponent({ name: 'NPagination' }).exists()).toBe(false)
  })

  it('forwards refresh and pagination events to the page', async () => {
    const wrapper = mountDrawer({ runTotal: 25 })

    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)

    wrapper.findComponent({ name: 'NPagination' }).vm.$emit('update:page', 3)
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('update:runPage')).toEqual([[3]])
  })

  it('renders the selected run detail and forwards retry with the run', async () => {
    const disabledSchedule = { ...schedule, enabled: false }
    const wrapper = mountDrawer({ schedule: disabledSchedule, selectedRun: run, retryingRunId: 'run-1' })
    const detail = wrapper.findComponent(ReportRunDetail)

    expect(detail.exists()).toBe(true)
    expect(detail.props('run')).toEqual(run)
    expect(detail.props('scheduleEnabled')).toBe(false)
    expect(detail.props('retrying')).toBe(true)

    detail.vm.$emit('retry', run)
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('retry')).toEqual([[run]])
  })
})
