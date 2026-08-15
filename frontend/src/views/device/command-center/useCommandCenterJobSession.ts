import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

type CommandJobSessionResult = {
  job_id?: string
  status?: string
  scheduled_at?: string
}

type ReadonlyRef<T> = {
  readonly value: T
}

type CommandJobSessionOptions = {
  activeCommandJobId: ReadonlyRef<string>
  jobActionLoading: ReadonlyRef<boolean>
  canRefreshCommandJob?: ReadonlyRef<boolean>
  refreshCommandJob: () => void | Promise<unknown>
  openCommandJobDetail: (jobId: string) => void | Promise<unknown>
  submitResult: ReadonlyRef<CommandJobSessionResult | null | undefined>
  storageKey?: string
  autoRefreshMs?: number
}

const DEFAULT_AUTO_REFRESH_MS = 6000
const DEFAULT_RECENT_RUNNING_JOB_KEY = 'aetherlink-command-center:recent-running-job'
const MAX_SCHEDULE_WAKE_DELAY_MS = 2_147_000_000

export function useCommandCenterJobSession(options: CommandJobSessionOptions) {
  const recentRunningCommandJobId = ref('')
  const scheduleClock = ref(Date.now())
  let autoRefreshTimer: number | undefined
  let scheduleWakeTimer: number | undefined

  const commandJobAutoRefreshActive = computed(() => {
    const result = options.submitResult.value
    if (result?.status === 'running') return true
    if (result?.status !== 'scheduled' || !result.scheduled_at) return false
    const scheduledAt = Date.parse(result.scheduled_at)
    return Number.isFinite(scheduledAt) && scheduledAt <= scheduleClock.value
  })
  const commandJobAutoRefreshDeferred = computed(
    () => commandJobAutoRefreshActive.value && !(options.canRefreshCommandJob?.value ?? !options.jobActionLoading.value)
  )
  const showRecentRunningCommandJob = computed(() => {
    const jobId = recentRunningCommandJobId.value
    if (!jobId) return false
    return jobId !== options.activeCommandJobId.value && jobId !== options.submitResult.value?.job_id
  })

  const readRecentRunningCommandJob = () => {
    if (typeof window === 'undefined') return ''
    return window.localStorage.getItem(options.storageKey || DEFAULT_RECENT_RUNNING_JOB_KEY) || ''
  }

  const rememberRecentRunningCommandJob = (jobId: string) => {
    recentRunningCommandJobId.value = jobId
    if (typeof window === 'undefined') return
    window.localStorage.setItem(options.storageKey || DEFAULT_RECENT_RUNNING_JOB_KEY, jobId)
  }

  const clearRecentRunningCommandJob = () => {
    recentRunningCommandJobId.value = ''
    if (typeof window === 'undefined') return
    window.localStorage.removeItem(options.storageKey || DEFAULT_RECENT_RUNNING_JOB_KEY)
  }

  const openRecentRunningCommandJob = () => {
    if (!recentRunningCommandJobId.value) return
    void options.openCommandJobDetail(recentRunningCommandJobId.value)
  }

  const stopCommandJobAutoRefresh = () => {
    if (typeof window === 'undefined' || autoRefreshTimer === undefined) return
    window.clearInterval(autoRefreshTimer)
    autoRefreshTimer = undefined
  }

  const stopCommandJobScheduleWake = () => {
    if (typeof window === 'undefined' || scheduleWakeTimer === undefined) return
    window.clearTimeout(scheduleWakeTimer)
    scheduleWakeTimer = undefined
  }

  const refreshCommandJobIfAvailable = () => {
    if (options.jobActionLoading.value) return
    if (options.canRefreshCommandJob && !options.canRefreshCommandJob.value) return
    if (typeof document !== 'undefined' && document.hidden) return
    void options.refreshCommandJob()
  }

  const startCommandJobAutoRefresh = () => {
    if (typeof window === 'undefined' || autoRefreshTimer !== undefined) return
    autoRefreshTimer = window.setInterval(() => {
      if (!commandJobAutoRefreshActive.value || options.jobActionLoading.value) return
      refreshCommandJobIfAvailable()
    }, options.autoRefreshMs || DEFAULT_AUTO_REFRESH_MS)
  }

  const syncCommandJobScheduleWake = () => {
    stopCommandJobScheduleWake()
    if (typeof window === 'undefined') return
    const result = options.submitResult.value
    if (result?.status !== 'scheduled' || !result.scheduled_at) return
    const scheduledAt = Date.parse(result.scheduled_at)
    if (!Number.isFinite(scheduledAt)) return
    const remainingMs = scheduledAt - Date.now()
    if (remainingMs <= 0) {
      scheduleClock.value = Date.now()
      refreshCommandJobIfAvailable()
      return
    }
    scheduleWakeTimer = window.setTimeout(
      () => {
        scheduleWakeTimer = undefined
        syncCommandJobScheduleWake()
      },
      Math.min(remainingMs, MAX_SCHEDULE_WAKE_DELAY_MS)
    )
  }

  const syncCommandJobAutoRefresh = () => {
    if (commandJobAutoRefreshActive.value) {
      startCommandJobAutoRefresh()
      return
    }
    stopCommandJobAutoRefresh()
  }

  onMounted(() => {
    recentRunningCommandJobId.value = readRecentRunningCommandJob()
  })

  watch(commandJobAutoRefreshActive, syncCommandJobAutoRefresh, { immediate: true })

  watch(
    () => [options.submitResult.value?.status || '', options.submitResult.value?.scheduled_at || ''],
    syncCommandJobScheduleWake,
    { immediate: true }
  )

  watch(
    () => [options.submitResult.value?.job_id || '', options.submitResult.value?.status || ''],
    ([jobId, status]) => {
      if (!jobId) return
      if (status === 'running' || status === 'scheduled') {
        rememberRecentRunningCommandJob(jobId)
        return
      }
      if (recentRunningCommandJobId.value === jobId) {
        clearRecentRunningCommandJob()
      }
    },
    { immediate: true }
  )

  onBeforeUnmount(() => {
    stopCommandJobAutoRefresh()
    stopCommandJobScheduleWake()
  })

  return {
    clearRecentRunningCommandJob,
    commandJobAutoRefreshActive,
    commandJobAutoRefreshDeferred,
    openRecentRunningCommandJob,
    recentRunningCommandJobId,
    showRecentRunningCommandJob
  }
}
