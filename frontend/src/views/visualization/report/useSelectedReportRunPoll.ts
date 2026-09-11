import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { getReportRun, type ReportRun } from '@/service/api/report'

const DEFAULT_POLL_MS = 5000
const DEFAULT_MAX_CONSECUTIVE_FAILURES = 3
const DEFAULT_MAX_BACKOFF_MS = 30000
const TERMINAL_STATUSES = new Set(['succeeded', 'failed', 'ambiguous'])

export type ReportRunPollFailure = {
  error: unknown
  consecutiveFailures: number
  stopped: boolean
}

interface SelectedReportRunPollOptions {
  selectedScheduleId: Ref<string>
  selectedRun: Ref<ReportRun | null>
  visible?: Ref<boolean>
  pollMs?: number
  maxConsecutiveFailures?: number
  maxBackoffMs?: number
  onUpdate?: (run: ReportRun) => void
  onFailure?: (failure: ReportRunPollFailure) => void
}

export const isReportRunTerminal = (run?: ReportRun | null) =>
  Boolean(run && TERMINAL_STATUSES.has(String(run.overall_status || '').toLowerCase()))

export function useSelectedReportRunPoll(options: SelectedReportRunPollOptions) {
  const pageVisible = ref(typeof document === 'undefined' ? true : !document.hidden)
  const polling = ref(false)
  const failure = ref<ReportRunPollFailure | null>(null)
  const active = computed(
    () =>
      Boolean(options.selectedScheduleId.value && options.selectedRun.value?.run_id) &&
      (options.visible?.value ?? true) &&
      !isReportRunTerminal(options.selectedRun.value) &&
      !failure.value?.stopped
  )
  let timer: number | undefined
  let requestSequence = 0
  let inFlight = false
  let refreshQueued = false
  let applyingRefresh = false
  let disposed = false

  const stop = () => {
    if (timer !== undefined && typeof window !== 'undefined') window.clearTimeout(timer)
    timer = undefined
  }

  const scheduleNext = () => {
    stop()
    if (!active.value || !pageVisible.value || typeof window === 'undefined' || disposed || inFlight) return
    const baseDelay = options.pollMs || DEFAULT_POLL_MS
    const failures = failure.value?.consecutiveFailures || 0
    const delay = Math.min(baseDelay * 2 ** failures, options.maxBackoffMs || DEFAULT_MAX_BACKOFF_MS)
    timer = window.setTimeout(() => void refresh(), delay)
  }

  const refreshImmediately = () => {
    stop()
    if (!active.value || !pageVisible.value || typeof window === 'undefined' || disposed) return
    void refresh()
  }

  const refresh = async () => {
    const scheduleId = options.selectedScheduleId.value
    const runId = options.selectedRun.value?.run_id
    if (!scheduleId || !runId || !active.value || !pageVisible.value || disposed) return
    if (inFlight) {
      refreshQueued = true
      return
    }

    const sequence = ++requestSequence
    inFlight = true
    polling.value = true
    try {
      const { data, error } = await getReportRun(scheduleId, runId)
      if (
        sequence !== requestSequence ||
        scheduleId !== options.selectedScheduleId.value ||
        runId !== options.selectedRun.value?.run_id ||
        !(options.visible?.value ?? true) ||
        !pageVisible.value ||
        disposed
      )
        return
      if (error || !data) {
        const nextFailure: ReportRunPollFailure = {
          error: error || new Error('missing report run data'),
          consecutiveFailures: (failure.value?.consecutiveFailures || 0) + 1,
          stopped: (failure.value?.consecutiveFailures || 0) + 1 >= (options.maxConsecutiveFailures || DEFAULT_MAX_CONSECUTIVE_FAILURES)
        }
        failure.value = nextFailure
        options.onFailure?.(nextFailure)
      } else {
        failure.value = null
        // Assigning the refreshed run can retrigger the status watcher. That
        // watcher must not invalidate this in-flight request or drop the
        // already-queued follow-up refresh, so mark the write as self-inflicted.
        applyingRefresh = true
        try {
          options.selectedRun.value = data
        } finally {
          applyingRefresh = false
        }
        options.onUpdate?.(data)
      }
    } catch (error) {
      if (
        sequence === requestSequence &&
        scheduleId === options.selectedScheduleId.value &&
        runId === options.selectedRun.value?.run_id &&
        !disposed
      ) {
        const consecutiveFailures = (failure.value?.consecutiveFailures || 0) + 1
        const nextFailure = {
          error,
          consecutiveFailures,
          stopped: consecutiveFailures >= (options.maxConsecutiveFailures || DEFAULT_MAX_CONSECUTIVE_FAILURES)
        }
        failure.value = nextFailure
        options.onFailure?.(nextFailure)
      }
    } finally {
      inFlight = false
      polling.value = false
      if (refreshQueued) {
        refreshQueued = false
        if (active.value && pageVisible.value && !disposed) {
          await refresh()
          return
        }
      }
      scheduleNext()
    }
  }

  const sync = () => {
    if (applyingRefresh) return
    requestSequence += 1
    refreshQueued = false
    stop()
    if (!active.value || !pageVisible.value || typeof window === 'undefined' || disposed) return
    scheduleNext()
  }

  const handleVisibilityChange = () => {
    if (typeof document === 'undefined') return
    const wasVisible = pageVisible.value
    pageVisible.value = !document.hidden
    // Resuming from a hidden state should not wait a full poll interval before
    // showing fresh state; the sync below handles the initial fetch.
    if (!wasVisible && pageVisible.value) refreshImmediately()
  }

  watch(
    () => [
      options.selectedScheduleId.value,
      options.selectedRun.value?.run_id,
      options.selectedRun.value?.overall_status,
      options.visible?.value ?? true,
      pageVisible.value
    ],
    (current, previous) => {
      if (!previous || current[0] !== previous[0] || current[1] !== previous[1]) failure.value = null
      sync()
    },
    { flush: 'sync' }
  )

  onMounted(() => {
    if (typeof document !== 'undefined') document.addEventListener('visibilitychange', handleVisibilityChange)
    sync()
  })

  onBeforeUnmount(() => {
    disposed = true
    requestSequence += 1
    refreshQueued = false
    stop()
    if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', handleVisibilityChange)
  })

  return { active, pageVisible, polling, failure, refresh, stop }
}
