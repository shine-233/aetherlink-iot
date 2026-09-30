/**
 * 文件用途：定时报表的「立即运行 / 运行历史 / 重试」编排。
 * 核心逻辑：
 *   - 立即运行与重试都携带幂等键；遇到「不确定的传输错误」（请求可能已到达后端）时用同一个键自动重放一次，
 *     得到确定响应后下一次操作才生成新键。
 *   - 运行历史列表的分页/加载态/过期请求丢弃收口在 useListPage（@/components/data-table-page/useListPage）；
 *     选中运行在新列表中的回填放在 onLoaded。
 *   - 选中运行的轮询委托给 useSelectedReportRunPoll（仅抽屉可见且选中运行未终态时轮询）。
 * 关键注意事项：rows 为 shallowRef，轮询回填/单行替换采用不可变替换触发重渲染；轮询在组件卸载时
 *   由 useSelectedReportRunPoll 自行清理，dispose() 仅作废在途历史请求。
 */
import { computed, ref } from 'vue'
import { $t } from '@/locales'
import { useListPage } from '@/components/data-table-page/useListPage'
import {
  getReportRun,
  listReportRuns,
  retryReportRun,
  runReportSchedule,
  type ReportRun,
  type ReportSchedule
} from '@/service/api/report'
import { createReportIdempotencyKey, isUncertainReportTransportError } from './report-model'
import { REPORT_RUN_PAGE_SIZE } from './report-helpers'
import { useSelectedReportRunPoll } from './useSelectedReportRunPoll'
import type { ReportMessenger } from './useReportSchedules'

export function useReportRuns(options: { message: ReportMessenger; reloadSchedules: () => Promise<unknown> }) {
  const { message } = options
  const runningId = ref('')
  const selectedSchedule = ref<ReportSchedule | null>(null)
  const selectedScheduleId = computed(() => selectedSchedule.value?.id || '')
  const historyVisible = ref(false)
  const selectedRun = ref<ReportRun | null>(null)
  const retryingRunId = ref('')
  const pollFailed = ref(false)

  // 运行历史状态机：分页/加载/过期请求丢弃交给 useListPage；计划 id 由 fetcher 现取。
  const {
    rows: runs,
    total: runTotal,
    page: runPage,
    loading: historyLoading,
    load: loadHistoryPage,
    setPage: setRunsPage,
    patchQuery: patchRunsQuery,
    cancel: cancelHistoryLoad
  } = useListPage<ReportRun>({
    initialQuery: () => ({}),
    initialPageSize: REPORT_RUN_PAGE_SIZE,
    fetcher: async (params) => {
      const scheduleId = selectedScheduleId.value
      if (!scheduleId) return null
      try {
        const { data, error } = await listReportRuns(scheduleId, { page: params.page, page_size: params.page_size })
        if (error || !data) throw error || new Error('missing data')
        return { list: data.list || [], total: data.total || 0 }
      } catch {
        message.error($t('report.message.historyFailed'))
        return null
      }
    },
    onLoaded: (result) => {
      if (selectedRun.value) {
        selectedRun.value = result.list.find((run) => run.run_id === selectedRun.value?.run_id) || selectedRun.value
      }
    }
  })

  /** 拉取当前选中计划的运行历史；未选中计划时不发请求。 */
  async function loadRuns() {
    if (!selectedScheduleId.value) return
    await loadHistoryPage()
  }

  /** 选中新产生的运行：优先取当前页，否则按 id 单独拉取详情。 */
  const selectCreatedRun = async (scheduleId: string, runId: string) => {
    selectedRun.value = runs.value.find((item) => item.run_id === runId) || null
    if (selectedRun.value) return
    const detail = await getReportRun(scheduleId, runId)
    if (!detail.error && detail.data) selectedRun.value = detail.data
  }

  /** 打开抽屉/立即运行后统一重置选中状态并回到第一页（不触发额外请求）。 */
  const resetSelection = (schedule: ReportSchedule) => {
    selectedSchedule.value = schedule
    selectedRun.value = null
    pollFailed.value = false
    historyVisible.value = true
    patchRunsQuery({}, { reload: false })
  }

  async function runNow(row: ReportSchedule, idempotencyKey = createReportIdempotencyKey()) {
    if (runningId.value || !row.enabled) return
    runningId.value = row.id
    let retryUncertain = false
    try {
      const { data, error } = await runReportSchedule(row.id, { idempotencyKey })
      if (error || !data) throw error || new Error('missing data')
      message.success($t(data.idempotent_replay ? 'report.message.runReplay' : 'report.message.runStarted'))
      resetSelection(row)
      await Promise.all([loadRuns(), options.reloadSchedules()])
      await selectCreatedRun(row.id, data.run_id)
    } catch (error) {
      message.error($t('report.message.runFailed'))
      retryUncertain = isUncertainReportTransportError(error)
    } finally {
      runningId.value = ''
    }
    if (retryUncertain) await runNow(row, idempotencyKey)
  }

  function openHistory(row: ReportSchedule) {
    resetSelection(row)
    runs.value = []
    void loadRuns()
  }
  function changeRunPage(next: number) {
    return setRunsPage(next)
  }
  function selectRun(run: ReportRun) {
    pollFailed.value = false
    selectedRun.value = run
  }

  async function retryRun(run: ReportRun, idempotencyKey = createReportIdempotencyKey()) {
    const scheduleId = selectedScheduleId.value
    if (!scheduleId || !selectedSchedule.value?.enabled || retryingRunId.value) return
    retryingRunId.value = run.run_id
    let retryUncertain = false
    try {
      const { data, error } = await retryReportRun(scheduleId, run.run_id, { idempotencyKey })
      if (error || !data) throw error || new Error('missing data')
      message.success($t(data.idempotent_replay ? 'report.message.retryReplay' : 'report.message.retryStarted'))
      selectedRun.value = null
      pollFailed.value = false
      await loadRuns()
      await selectCreatedRun(scheduleId, data.run_id)
    } catch (error) {
      message.error($t('report.message.retryFailed'))
      retryUncertain = isUncertainReportTransportError(error)
    } finally {
      retryingRunId.value = ''
    }
    if (retryUncertain) await retryRun(run, idempotencyKey)
  }

  useSelectedReportRunPoll({
    selectedScheduleId,
    selectedRun,
    visible: historyVisible,
    onUpdate: (run) => {
      pollFailed.value = false
      // rows 为 shallowRef：轮询回填用不可变替换触发重渲染。
      runs.value = runs.value.map((item) => (item.run_id === run.run_id ? run : item))
    },
    onFailure: ({ stopped }) => {
      if (stopped) pollFailed.value = true
    }
  })

  const dispose = () => {
    cancelHistoryLoad()
  }

  return {
    runningId,
    selectedSchedule,
    historyVisible,
    historyLoading,
    runs,
    runTotal,
    runPage,
    selectedRun,
    retryingRunId,
    pollFailed,
    loadRuns,
    runNow,
    openHistory,
    changeRunPage,
    selectRun,
    retryRun,
    dispose
  }
}
