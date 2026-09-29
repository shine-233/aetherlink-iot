/**
 * 文件用途：定时报表的「立即运行 / 运行历史 / 重试」编排。
 * 核心逻辑：
 *   - 立即运行与重试都携带幂等键；遇到「不确定的传输错误」（请求可能已到达后端）时用同一个键自动重放一次，
 *     得到确定响应后下一次操作才生成新键。
 *   - 历史列表请求带序号 + 计划/页码快照，过期响应丢弃。
 *   - 选中运行的轮询委托给 useSelectedReportRunPoll（仅抽屉可见且选中运行未终态时轮询）。
 * 关键注意事项：dispose() 使在途历史响应失效；轮询在组件卸载时由 useSelectedReportRunPoll 自行清理。
 */
import { computed, ref } from 'vue'
import { $t } from '@/locales'
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

export function useReportRuns(options: { message: ReportMessenger; reloadSchedules: () => Promise<void> }) {
  const { message } = options
  const runningId = ref('')
  const selectedSchedule = ref<ReportSchedule | null>(null)
  const selectedScheduleId = computed(() => selectedSchedule.value?.id || '')
  const historyVisible = ref(false)
  const historyLoading = ref(false)
  const runs = ref<ReportRun[]>([])
  const runTotal = ref(0)
  const runPage = ref(1)
  const selectedRun = ref<ReportRun | null>(null)
  const retryingRunId = ref('')
  const pollFailed = ref(false)
  let sequence = 0

  async function loadRuns() {
    const scheduleId = selectedScheduleId.value
    if (!scheduleId) return
    const seq = ++sequence
    const snapshotPage = runPage.value
    historyLoading.value = true
    try {
      const { data, error } = await listReportRuns(scheduleId, { page: snapshotPage, page_size: REPORT_RUN_PAGE_SIZE })
      if (seq !== sequence || scheduleId !== selectedScheduleId.value || snapshotPage !== runPage.value) return
      if (error || !data) throw error || new Error('missing data')
      runs.value = data.list || []
      runTotal.value = data.total || 0
      if (selectedRun.value) {
        selectedRun.value = runs.value.find((run) => run.run_id === selectedRun.value?.run_id) || selectedRun.value
      }
    } catch {
      if (seq === sequence) message.error($t('report.message.historyFailed'))
    } finally {
      if (seq === sequence) historyLoading.value = false
    }
  }

  /** 选中新产生的运行：优先取当前页，否则按 id 单独拉取详情。 */
  const selectCreatedRun = async (scheduleId: string, runId: string) => {
    selectedRun.value = runs.value.find((item) => item.run_id === runId) || null
    if (selectedRun.value) return
    const detail = await getReportRun(scheduleId, runId)
    if (!detail.error && detail.data) selectedRun.value = detail.data
  }

  const resetSelection = (schedule: ReportSchedule) => {
    selectedSchedule.value = schedule
    selectedRun.value = null
    pollFailed.value = false
    runPage.value = 1
    historyVisible.value = true
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
    runPage.value = next
    void loadRuns()
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
      const index = runs.value.findIndex((item) => item.run_id === run.run_id)
      if (index >= 0) runs.value.splice(index, 1, run)
    },
    onFailure: ({ stopped }) => {
      if (stopped) pollFailed.value = true
    }
  })

  const dispose = () => {
    sequence += 1
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
