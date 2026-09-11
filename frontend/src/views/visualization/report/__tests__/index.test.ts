import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'

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
  createKey: vi.fn(),
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
vi.mock('../report-model', () => ({
  canRetryReportRun: (run?: { overall_status?: string }, scheduleEnabled = true) =>
    scheduleEnabled && ['failed', 'ambiguous'].includes(run?.overall_status || ''),
  createReportIdempotencyKey: hoisted.createKey,
  isUncertainReportTransportError: (error: { response?: unknown; request?: unknown; code?: string } | undefined) =>
    Boolean(error && !error.response && (error.request || error.code === 'ERR_NETWORK')),
  reportStatusTagType: () => 'default'
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

const schedule = {
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

const wrappers: Array<ReturnType<typeof shallowMount>> = []
const mountPage = () => {
  const wrapper = shallowMount(ReportPage)
  wrappers.push(wrapper)
  return wrapper
}
const state = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('scheduled report page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.pollOptions = undefined
    hoisted.createKey.mockReset()
    hoisted.createKey.mockReturnValueOnce('action-key-1').mockReturnValueOnce('action-key-2')
    hoisted.listReportSchedules.mockResolvedValue({
      data: { list: [schedule], total: 1, page: 1, page_size: 10 },
      error: null
    })
    hoisted.listReportRuns.mockResolvedValue({ data: { list: [run], total: 1, page: 1, page_size: 10 }, error: null })
    hoisted.getReportRun.mockResolvedValue({ data: run, error: null })
    hoisted.getReportSchedule.mockResolvedValue({ data: schedule, error: null })
    hoisted.runReportSchedule.mockResolvedValue({
      data: {
        run_id: 'run-1',
        schedule_id: 'schedule-1',
        status: 'pending',
        status_url: '/status/run-1',
        idempotent_replay: false,
        duplicate_delivery_risk: false
      },
      error: null
    })
    hoisted.retryReportRun.mockResolvedValue({
      data: {
        run_id: 'run-2',
        schedule_id: 'schedule-1',
        status: 'pending',
        status_url: '/status/run-2',
        idempotent_replay: false,
        duplicate_delivery_risk: true
      },
      error: null
    })
    hoisted.updateReportSchedule.mockResolvedValue({ data: schedule, error: null })
    hoisted.createReportSchedule.mockResolvedValue({ data: schedule, error: null })
  })

  afterEach(() => {
    wrappers.forEach((wrapper) => wrapper.unmount())
    wrappers.length = 0
  })

  it('loads schedule pagination and configures selected-run-only drawer polling', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(hoisted.listReportSchedules).toHaveBeenCalledWith({ page: 1, page_size: 10, search: '' })
    expect(hoisted.pollOptions).toEqual(
      expect.objectContaining({
        selectedScheduleId: expect.any(Object),
        selectedRun: expect.any(Object),
        visible: expect.any(Object),
        onFailure: expect.any(Function)
      })
    )
    expect((hoisted.pollOptions?.visible as { value: boolean }).value).toBe(false)
  })

  it('runs once with a new key, selects the returned run, and starts polling only after opening history', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await state(wrapper).runNow(schedule)

    expect(hoisted.runReportSchedule).toHaveBeenCalledWith('schedule-1', { idempotencyKey: 'action-key-1' })
    expect(hoisted.listReportRuns).toHaveBeenCalledWith('schedule-1', { page: 1, page_size: 10 })
    expect(state(wrapper).selectedRun.run_id).toBe('run-1')
    expect(state(wrapper).historyVisible).toBe(true)
  })

  it('automatically retries uncertain transport failures with the same key and rotates after a definite response', async () => {
    hoisted.createKey.mockReset()
    hoisted.createKey.mockReturnValueOnce('uncertain-key').mockReturnValueOnce('new-action-key')
    hoisted.runReportSchedule
      .mockResolvedValueOnce({ data: null, error: { code: 'ERR_NETWORK', request: {} } })
      .mockResolvedValueOnce({ data: null, error: { response: { status: 409 } } })
      .mockResolvedValueOnce({ data: null, error: { response: { status: 409 } } })
    const wrapper = mountPage()
    await flushPromises()

    await state(wrapper).runNow(schedule)
    await state(wrapper).runNow(schedule)

    expect(hoisted.runReportSchedule.mock.calls.map((call) => call[1].idempotencyKey)).toEqual([
      'uncertain-key',
      'uncertain-key',
      'new-action-key'
    ])
    expect(hoisted.createKey).toHaveBeenCalledTimes(2)
  })

  it('blocks run and retry actions while the selected schedule is disabled', async () => {
    const disabledSchedule = { ...schedule, enabled: false }
    const wrapper = mountPage()
    await flushPromises()

    await state(wrapper).runNow(disabledSchedule)
    state(wrapper).openHistory(disabledSchedule)
    await flushPromises()
    await state(wrapper).retryRun({ ...run, overall_status: 'failed', generation_status: 'failed' })

    expect(hoisted.runReportSchedule).not.toHaveBeenCalled()
    expect(hoisted.retryReportRun).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('report.message.scheduleDisabled')
  })

  it('surfaces a terminal polling failure in the drawer', async () => {
    const wrapper = mountPage()
    await flushPromises()
    state(wrapper).openHistory(schedule)
    await flushPromises()

    ;(hoisted.pollOptions?.onFailure as (failure: { stopped: boolean }) => void)({ stopped: true })
    await wrapper.vm.$nextTick()

    expect(state(wrapper).pollFailed).toBe(true)
    expect(wrapper.text()).toContain('report.message.pollFailed')
  })

  it('updates with route identity and current revision while keeping create enabled by default', async () => {
    const wrapper = mountPage()
    await flushPromises()
    state(wrapper).openCreate()
    expect(state(wrapper).form.enabled).toBe(true)

    state(wrapper).openEdit(schedule)
    state(wrapper).form.device_ids = ['device-1']
    state(wrapper).form.keys = ['temperature']
    state(wrapper).deviceIdsText = 'device-1'
    state(wrapper).keysText = 'temperature'
    state(wrapper).formRef = { validate: () => Promise.resolve() }
    await state(wrapper).saveSchedule()
    expect(hoisted.updateReportSchedule).toHaveBeenCalledWith('schedule-1', expect.objectContaining({ revision: 7 }))
    expect(hoisted.updateReportSchedule.mock.calls[0][1]).not.toHaveProperty('id')
  })

  it('refreshes a stale edit by structured backend code instead of matching an error message', async () => {
    const stale = { ...schedule, revision: 8, name: 'Changed elsewhere' }
    hoisted.updateReportSchedule.mockResolvedValueOnce({
      data: null,
      error: { response: { data: { code: 201002, message: 'translated elsewhere' } } }
    })
    hoisted.getReportSchedule.mockResolvedValueOnce({ data: stale, error: null })
    const wrapper = mountPage()
    await flushPromises()
    state(wrapper).openEdit(schedule)
    state(wrapper).deviceIdsText = 'device-1'
    state(wrapper).keysText = 'temperature'
    state(wrapper).formRef = { validate: () => Promise.resolve() }

    await state(wrapper).saveSchedule()

    expect(hoisted.getReportSchedule).toHaveBeenCalledWith('schedule-1')
    expect(hoisted.listReportSchedules).toHaveBeenCalledTimes(1)
    expect(state(wrapper).editing.revision).toBe(8)
    expect(state(wrapper).form.revision).toBe(8)
    expect(hoisted.messageWarning).toHaveBeenCalledWith('report.message.staleRevision')
    expect(hoisted.messageError).not.toHaveBeenCalled()
  })

  it('deletes with the current revision and refreshes after a structured stale-revision error', async () => {
    const stale = { ...schedule, revision: 8 }
    hoisted.deleteReportSchedule.mockResolvedValueOnce({
      data: null,
      error: { code: 201002, message: 'not a stable contract' }
    })
    hoisted.getReportSchedule.mockResolvedValueOnce({ data: stale, error: null })
    const wrapper = mountPage()
    await flushPromises()

    await state(wrapper).removeSchedule(schedule)

    expect(hoisted.deleteReportSchedule).toHaveBeenCalledWith('schedule-1', 7)
    expect(hoisted.getReportSchedule).toHaveBeenCalledWith('schedule-1')
    expect(hoisted.listReportSchedules).toHaveBeenCalledTimes(1)
    expect(state(wrapper).schedules[0].revision).toBe(8)
    expect(hoisted.messageWarning).toHaveBeenCalledWith('report.message.deleteStaleRevision')
    expect(hoisted.messageError).not.toHaveBeenCalled()
  })

  it('uses a distinct key for an explicit retry and selects the immutable child run', async () => {
    const wrapper = mountPage()
    await flushPromises()
    state(wrapper).openHistory(schedule)
    await flushPromises()
    await state(wrapper).runNow(schedule)
    await state(wrapper).retryRun({
      ...run,
      overall_status: 'ambiguous',
      delivery_status: 'ambiguous',
      duplicate_delivery_risk: true
    })

    expect(hoisted.retryReportRun).toHaveBeenCalledWith('schedule-1', 'run-1', { idempotencyKey: 'action-key-2' })
    expect(hoisted.runReportSchedule.mock.calls[0][1].idempotencyKey).not.toBe(
      hoisted.retryReportRun.mock.calls[0][2].idempotencyKey
    )
  })

  it('does not expose unsupported cancel, artifact, download, or delivery-only actions', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).not.toMatch(/artifact|download|delivery.only|cancel.run/i)
  })
})
