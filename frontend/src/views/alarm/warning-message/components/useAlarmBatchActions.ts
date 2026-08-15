import { computed, ref, type Ref } from 'vue'
import { batchActionAlarmHistory } from '@/service/api/alarm'
import { $t } from '@/locales'
import {
  buildAlarmBatchActionEvidence,
  isAcknowledged,
  isReset,
  type AlarmBatchActionEvidence
} from './alarm-configuration.helpers'
import type { AlarmConfigurationRow } from './alarmConfigurationColumns'

export type BatchAlarmAction = 'acknowledge' | 'reset'

export function useAlarmBatchActions(options: {
  tableData: Ref<AlarmConfigurationRow[]>
  selectedAlarmRowKeys: Ref<Array<string | number>>
  refresh: () => Promise<void> | void
}) {
  const batchActionLoading = ref(false)
  const batchActionDialogVisible = ref(false)
  const batchActionRows = ref<AlarmConfigurationRow[]>([])
  const batchActionType = ref<BatchAlarmAction>('acknowledge')
  const batchActionNote = ref('')
  const batchActionNoteMaxLength = 500
  const lastBatchActionEvidence = ref<AlarmBatchActionEvidence | null>(null)

  const selectedAlarmRows = computed(() => {
    const selected = new Set(options.selectedAlarmRowKeys.value)
    return options.tableData.value.filter(row => selected.has(row.id))
  })
  const selectedUnacknowledgedRows = computed(() => selectedAlarmRows.value.filter(row => !isAcknowledged(row)))
  const selectedActiveRows = computed(() => selectedAlarmRows.value.filter(row => !isReset(row)))
  const batchActionDialogTitle = computed(() =>
    batchActionType.value === 'acknowledge'
      ? $t('custom.alarmPage.batchAcknowledgeTitle')
      : $t('custom.alarmPage.batchResetTitle')
  )
  const batchActionDialogHint = computed(() => {
    const key =
      batchActionType.value === 'acknowledge'
        ? 'custom.alarmPage.batchAcknowledgeHint'
        : 'custom.alarmPage.batchResetHint'
    return $t(key).replace('{count}', String(batchActionRows.value.length))
  })

  const showBatchActionResult = (response: any, expectedCount: number, action: BatchAlarmAction, note: string) => {
    const evidence = buildAlarmBatchActionEvidence({
      response,
      expectedCount,
      action,
      note,
      t: $t
    })
    lastBatchActionEvidence.value = evidence

    if (evidence.failureCount > 0) {
      window.$message?.warning(evidence.detail)
      return
    }

    window.$message?.success(evidence.detail)
  }

  const closeBatchActionDialog = () => {
    batchActionDialogVisible.value = false
    batchActionRows.value = []
    batchActionNote.value = ''
  }

  const runBatchAlarmAction = async () => {
    const rows = batchActionRows.value
    if (!rows.length) return
    batchActionLoading.value = true
    try {
      const note = batchActionNote.value.trim()
      const response = await batchActionAlarmHistory({
        ids: rows.map(row => row.id),
        action: batchActionType.value,
        ...(note ? { note } : {})
      })
      showBatchActionResult(response, rows.length, batchActionType.value, note)
      closeBatchActionDialog()
      options.selectedAlarmRowKeys.value = []
      await options.refresh()
    } catch {
      window.$message?.error($t('custom.alarmPage.batchActionRequestFailed'))
    } finally {
      batchActionLoading.value = false
    }
  }

  const openBatchAlarmAction = (rows: AlarmConfigurationRow[], action: BatchAlarmAction) => {
    if (!rows.length) return
    batchActionRows.value = [...rows]
    batchActionType.value = action
    batchActionNote.value = ''
    batchActionDialogVisible.value = true
  }

  const acknowledgeCurrentPage = () => {
    const rows = selectedUnacknowledgedRows.value
    if (!rows.length) return
    openBatchAlarmAction(rows, 'acknowledge')
  }

  const resetCurrentPage = () => {
    const rows = selectedActiveRows.value
    if (!rows.length) return
    openBatchAlarmAction(rows, 'reset')
  }

  return {
    selectedUnacknowledgedRows,
    selectedActiveRows,
    batchActionLoading,
    batchActionDialogVisible,
    batchActionNote,
    batchActionNoteMaxLength,
    batchActionDialogTitle,
    batchActionDialogHint,
    lastBatchActionEvidence,
    closeBatchActionDialog,
    runBatchAlarmAction,
    acknowledgeCurrentPage,
    resetCurrentPage
  }
}
