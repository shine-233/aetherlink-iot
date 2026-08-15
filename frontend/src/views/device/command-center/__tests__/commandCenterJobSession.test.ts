import { createApp, defineComponent, nextTick, ref, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useCommandCenterJobSession } from '../useCommandCenterJobSession'

type SessionOptions = Parameters<typeof useCommandCenterJobSession>[0]
type CommandCenterJobSession = ReturnType<typeof useCommandCenterJobSession>

const mountedApps: App[] = []

// This composable owns lifecycle-bound timers, so tests mount it through a
// component instead of calling setup hooks without an active Vue instance.
function mountCommandCenterJobSession(options: SessionOptions): CommandCenterJobSession {
  let session: CommandCenterJobSession | undefined
  const app = createApp(
    defineComponent({
      setup() {
        session = useCommandCenterJobSession(options)
        return () => null
      }
    })
  )
  app.mount(document.createElement('div'))
  mountedApps.push(app)
  return session!
}

describe('useCommandCenterJobSession', () => {
  afterEach(() => {
    mountedApps
      .splice(0)
      .reverse()
      .forEach((app) => app.unmount())
    vi.useRealTimers()
  })

  it('remembers running jobs and exposes a prompt when the active page is different', async () => {
    const openCommandJobDetail = vi.fn()
    const refreshCommandJob = vi.fn()
    const activeCommandJobId = ref('')
    const jobActionLoading = ref(false)
    const submitResult = ref({ job_id: '', status: '' })

    const session = mountCommandCenterJobSession({
      activeCommandJobId,
      jobActionLoading,
      refreshCommandJob,
      openCommandJobDetail,
      submitResult,
      storageKey: 'test-command-job-session'
    })

    submitResult.value = { job_id: 'job-1', status: 'running' }
    await nextTick()

    expect(session.recentRunningCommandJobId.value).toBe('job-1')
    // The current page owns the submitted job; clear its local result to model
    // navigating to a different page before asserting the cross-page prompt.
    submitResult.value = { job_id: '', status: '' }
    await nextTick()
    expect(session.showRecentRunningCommandJob.value).toBe(true)

    session.openRecentRunningCommandJob()
    expect(openCommandJobDetail).toHaveBeenCalledWith('job-1')

    activeCommandJobId.value = 'job-1'
    await nextTick()
    expect(session.showRecentRunningCommandJob.value).toBe(false)
  })

  it('clears the remembered running job when the same job reaches a terminal state', async () => {
    const session = mountCommandCenterJobSession({
      activeCommandJobId: ref(''),
      jobActionLoading: ref(false),
      refreshCommandJob: vi.fn(),
      openCommandJobDetail: vi.fn(),
      submitResult: ref({ job_id: 'job-2', status: 'running' }),
      storageKey: 'test-command-job-terminal'
    })
    await nextTick()

    expect(session.recentRunningCommandJobId.value).toBe('job-2')

    session.clearRecentRunningCommandJob()
    expect(session.recentRunningCommandJobId.value).toBe('')
  })

  it('marks auto refresh active while the current job needs polling', async () => {
    const submitResult = ref({ job_id: 'job-3', status: 'running' })
    const session = mountCommandCenterJobSession({
      activeCommandJobId: ref(''),
      jobActionLoading: ref(false),
      refreshCommandJob: vi.fn(),
      openCommandJobDetail: vi.fn(),
      submitResult,
      storageKey: 'test-command-job-refresh'
    })

    expect(session.commandJobAutoRefreshActive.value).toBe(true)

    submitResult.value = { job_id: 'job-3', status: 'success' }
    await nextTick()

    expect(session.commandJobAutoRefreshActive.value).toBe(false)
  })

  it('remembers scheduled jobs and starts refreshing only when their planned time arrives', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-20T00:00:00.000Z'))
    const refreshCommandJob = vi.fn()
    const submitResult = ref({
      job_id: 'job-scheduled',
      status: 'scheduled',
      scheduled_at: '2026-07-20T00:01:00.000Z'
    })
    const session = mountCommandCenterJobSession({
      activeCommandJobId: ref(''),
      jobActionLoading: ref(false),
      refreshCommandJob,
      openCommandJobDetail: vi.fn(),
      submitResult,
      storageKey: 'test-command-job-scheduled'
    })
    await nextTick()

    expect(session.recentRunningCommandJobId.value).toBe('job-scheduled')
    expect(session.commandJobAutoRefreshActive.value).toBe(false)

    await vi.advanceTimersByTimeAsync(59_000)
    expect(refreshCommandJob).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1_000)
    await nextTick()
    expect(refreshCommandJob).toHaveBeenCalledTimes(1)
    expect(session.commandJobAutoRefreshActive.value).toBe(true)

    submitResult.value = { job_id: 'job-scheduled', status: 'completed', scheduled_at: submitResult.value.scheduled_at }
    await nextTick()
    expect(session.commandJobAutoRefreshActive.value).toBe(false)
    expect(session.recentRunningCommandJobId.value).toBe('')
  })

  it('cancels the scheduled wake when the job reaches a terminal state early', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-20T00:00:00.000Z'))
    const refreshCommandJob = vi.fn()
    const submitResult = ref({
      job_id: 'job-canceled-before-start',
      status: 'scheduled',
      scheduled_at: '2026-07-20T00:01:00.000Z'
    })
    const session = mountCommandCenterJobSession({
      activeCommandJobId: ref(''),
      jobActionLoading: ref(false),
      refreshCommandJob,
      openCommandJobDetail: vi.fn(),
      submitResult,
      storageKey: 'test-command-job-scheduled-cancel'
    })
    await nextTick()

    await vi.advanceTimersByTimeAsync(30_000)
    submitResult.value = { ...submitResult.value, status: 'canceled' }
    await nextTick()
    await vi.advanceTimersByTimeAsync(60_000)

    expect(refreshCommandJob).not.toHaveBeenCalled()
    expect(session.commandJobAutoRefreshActive.value).toBe(false)
    expect(session.recentRunningCommandJobId.value).toBe('')
  })

  it('replaces the old wake deadline when a scheduled job is rescheduled', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-20T00:00:00.000Z'))
    const refreshCommandJob = vi.fn()
    const submitResult = ref({
      job_id: 'job-rescheduled',
      status: 'scheduled',
      scheduled_at: '2026-07-20T00:01:00.000Z'
    })
    const session = mountCommandCenterJobSession({
      activeCommandJobId: ref(''),
      jobActionLoading: ref(false),
      refreshCommandJob,
      openCommandJobDetail: vi.fn(),
      submitResult,
      storageKey: 'test-command-job-rescheduled'
    })
    await nextTick()

    await vi.advanceTimersByTimeAsync(30_000)
    submitResult.value = { ...submitResult.value, scheduled_at: '2026-07-20T00:02:00.000Z' }
    await nextTick()
    await vi.advanceTimersByTimeAsync(89_999)

    expect(refreshCommandJob).not.toHaveBeenCalled()
    expect(session.commandJobAutoRefreshActive.value).toBe(false)

    await vi.advanceTimersByTimeAsync(1)
    await nextTick()
    expect(refreshCommandJob).toHaveBeenCalledTimes(1)
    expect(session.commandJobAutoRefreshActive.value).toBe(true)
  })

  it('defers the due refresh behind the refresh gate and resumes on the next polling tick', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-20T00:00:00.000Z'))
    const canRefreshCommandJob = ref(false)
    const refreshCommandJob = vi.fn()
    const session = mountCommandCenterJobSession({
      activeCommandJobId: ref(''),
      jobActionLoading: ref(false),
      canRefreshCommandJob,
      refreshCommandJob,
      openCommandJobDetail: vi.fn(),
      submitResult: ref({
        job_id: 'job-refresh-gated',
        status: 'scheduled',
        scheduled_at: '2026-07-20T00:00:01.000Z'
      }),
      storageKey: 'test-command-job-scheduled-gate',
      autoRefreshMs: 5_000
    })
    await nextTick()

    await vi.advanceTimersByTimeAsync(1_000)
    await nextTick()
    expect(refreshCommandJob).not.toHaveBeenCalled()
    expect(session.commandJobAutoRefreshActive.value).toBe(true)
    expect(session.commandJobAutoRefreshDeferred.value).toBe(true)

    canRefreshCommandJob.value = true
    await nextTick()
    await vi.advanceTimersByTimeAsync(4_999)
    expect(refreshCommandJob).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1)
    expect(refreshCommandJob).toHaveBeenCalledTimes(1)
  })
})
