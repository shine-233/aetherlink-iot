import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, effectScope, h, nextTick, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { useSelectedReportRunPoll } from '../useSelectedReportRunPoll'
import type { ReportRun } from '@/service/api/report'

const { getReportRun } = vi.hoisted(() => ({ getReportRun: vi.fn() }))
vi.mock('@/service/api/report', () => ({ getReportRun }))

const activeRun = (): ReportRun => ({
  run_id: 'run-1',
  schedule_id: 'schedule-1',
  overall_status: 'processing',
  generation_status: 'processing',
  delivery_status: 'pending',
  duplicate_delivery_risk: false
})

interface HarnessOptions {
  scheduleId?: ReturnType<typeof ref<string>>
  selectedRun?: ReturnType<typeof ref<ReportRun | null>>
  visible?: ReturnType<typeof ref<boolean>>
  pollMs?: number
  maxConsecutiveFailures?: number
  onFailure?: ReturnType<typeof vi.fn>
}

const mountHarness = (options: HarnessOptions = {}) => {
  const scheduleId = options.scheduleId || ref('schedule-1')
  const selectedRun = options.selectedRun || ref<ReportRun | null>(activeRun())
  const visible = options.visible || ref(true)
  const exposed: Record<string, unknown> = {}
  const component = defineComponent({
    setup() {
      Object.assign(
        exposed,
        useSelectedReportRunPoll({
          selectedScheduleId: scheduleId,
          selectedRun,
          visible,
          pollMs: options.pollMs || 100,
          maxConsecutiveFailures: options.maxConsecutiveFailures,
          onFailure: options.onFailure
        })
      )
      return () => h('div')
    }
  })
  return { wrapper: mount(component), scheduleId, selectedRun, visible, exposed }
}

describe('selected report run polling', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    getReportRun.mockReset()
  })

  it('polls only the selected active run and ignores stale responses', async () => {
    vi.useFakeTimers()
    let resolveFirst: (value: unknown) => void = () => undefined
    getReportRun.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveFirst = resolve
      })
    )
    const { wrapper, selectedRun } = mountHarness()

    await vi.advanceTimersByTimeAsync(100)
    selectedRun.value = { ...activeRun(), run_id: 'run-2' }
    resolveFirst({ data: { ...activeRun(), overall_status: 'succeeded' }, error: null })
    await Promise.resolve()

    expect(selectedRun.value?.run_id).toBe('run-2')
    wrapper.unmount()
  })

  it('does not overlap refreshes and services one queued refresh afterward', async () => {
    let resolveFirst: (value: unknown) => void = () => undefined
    getReportRun
      .mockReturnValueOnce(
        new Promise((resolve) => {
          resolveFirst = resolve
        })
      )
      .mockResolvedValue({ data: activeRun(), error: null })
    const { wrapper, exposed } = mountHarness()

    const first = (exposed.refresh as () => Promise<void>)()
    const second = (exposed.refresh as () => Promise<void>)()
    expect(getReportRun).toHaveBeenCalledTimes(1)

    resolveFirst({ data: activeRun(), error: null })
    await first
    await second
    await nextTick()
    expect(getReportRun).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('backs off after failures and stops with exposed failure state at the threshold', async () => {
    vi.useFakeTimers()
    const onFailure = vi.fn()
    getReportRun.mockResolvedValue({ data: null, error: { response: { status: 503 } } })
    const { wrapper, exposed } = mountHarness({ maxConsecutiveFailures: 2, onFailure })

    await vi.advanceTimersByTimeAsync(100)
    expect(getReportRun).toHaveBeenCalledTimes(1)
    expect(onFailure).toHaveBeenLastCalledWith(expect.objectContaining({ consecutiveFailures: 1, stopped: false }))

    await vi.advanceTimersByTimeAsync(199)
    expect(getReportRun).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(getReportRun).toHaveBeenCalledTimes(2)
    expect(onFailure).toHaveBeenLastCalledWith(expect.objectContaining({ consecutiveFailures: 2, stopped: true }))
    expect((exposed.failure as { value: { stopped: boolean } | null }).value?.stopped).toBe(true)

    await vi.advanceTimersByTimeAsync(1000)
    expect(getReportRun).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('resets a stopped failure when a different run is selected', async () => {
    vi.useFakeTimers()
    getReportRun
      .mockResolvedValueOnce({ data: null, error: { code: 'ERR_NETWORK' } })
      .mockResolvedValue({ data: activeRun(), error: null })
    const { wrapper, selectedRun, exposed } = mountHarness({ maxConsecutiveFailures: 1 })

    await vi.advanceTimersByTimeAsync(100)
    expect((exposed.failure as { value: { stopped: boolean } | null }).value?.stopped).toBe(true)

    selectedRun.value = { ...activeRun(), run_id: 'run-2' }
    await nextTick()
    expect((exposed.failure as { value: unknown }).value).toBeNull()
    await vi.advanceTimersByTimeAsync(100)
    expect(getReportRun).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('stops while the history drawer or document is hidden and refreshes when visible', async () => {
    vi.useFakeTimers()
    getReportRun.mockResolvedValue({ data: activeRun(), error: null })
    const hidden = vi.spyOn(document, 'hidden', 'get')
    hidden.mockReturnValue(false)
    const { wrapper, visible } = mountHarness({ visible: ref(false) })

    await vi.advanceTimersByTimeAsync(300)
    expect(getReportRun).not.toHaveBeenCalled()

    visible.value = true
    await nextTick()
    await vi.advanceTimersByTimeAsync(100)
    expect(getReportRun).toHaveBeenCalledTimes(1)

    hidden.mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(300)
    expect(getReportRun).toHaveBeenCalledTimes(1)

    hidden.mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await nextTick()
    expect(getReportRun).toHaveBeenCalledTimes(2)

    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(300)
    expect(getReportRun).toHaveBeenCalledTimes(2)
  })
})
