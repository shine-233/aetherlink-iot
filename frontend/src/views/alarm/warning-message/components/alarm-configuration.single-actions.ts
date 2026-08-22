/**
 * 文件用途：告警配置页“单条确认 / 重置”动作弹窗的状态管理与提交闭环。
 * 核心逻辑：维护弹窗可见性、备注、加载态、审计摘要与最近一次单条操作证据。
 * 关键注意事项：提交成功后会同步关闭详情弹窗并触发列表刷新；证据行与边界文案由调用方注入，保证与页面上下文一致。
 */
import { computed, ref, type Ref } from 'vue'
import dayjs from 'dayjs'
import { batchActionAlarmHistory } from '@/service/api/alarm'
import { $t } from '@/locales'
import {
  alarmSeverityLabel,
  alarmSeverityValue,
  alarmTypeLabel,
  type AlarmOption
} from './alarm-configuration.helpers'

export type AlarmSingleActionType = 'acknowledge' | 'reset'

/** 单条操作弹窗的告警行数据（历史列表行，字段宽松） */
export type AlarmSingleActionRow = Record<string, unknown> & {
  id?: string
  name?: string
  description?: unknown
  content?: unknown
  create_at?: string | number
  alarm_config_name?: string
  alarm_status?: string
  alarm_level?: string
  remark?: unknown
}

export type UseAlarmSingleActionsOptions = {
  /** 告警级别选项（与筛选表单共用同一份响应式数据） */
  severityOptions: Ref<AlarmOption[]>
  /** 由页面注入的证据行构造器（依赖严重级别选项与时间格式化） */
  evidenceRowOf: (row: AlarmSingleActionRow) => unknown
  /** 证据边界文案 */
  evidenceBoundaryLabel: () => string
  /** 提交成功后联动关闭详情弹窗 */
  closeDetailDialog: () => void
  /** 提交成功后的列表刷新入口 */
  refresh: () => Promise<void> | void
}

export function useAlarmSingleActions(options: UseAlarmSingleActionsOptions) {
  const singleActionDialogVisible = ref(false)
  const singleActionLoading = ref(false)
  const singleActionNote = ref('')
  const singleActionRow = ref<AlarmSingleActionRow | null>(null)
  const singleActionType = ref<AlarmSingleActionType>('acknowledge')
  const singleActionNoteMaxLength = 500
  const lastSingleClosureEvidence = ref<Record<string, unknown> | null>(null)

  const alarmAuditSummary = (row: AlarmSingleActionRow) =>
    [
      `${$t('generate.alarmConfugName')}: ${row.name || '-'}`,
      `${$t('common.alarm_level')}: ${alarmSeverityLabel(alarmSeverityValue(row), options.severityOptions.value)}`,
      `${$t('rdi.overview.alarmType')}: ${alarmTypeLabel(row, $t)}`,
      `${$t('common.alarm_time')}: ${row.create_at ? dayjs(row.create_at as string).format('YYYY-MM-DD HH:mm:ss') : '-'}`
    ].join('\n')

  const singleActionDialogTitle = computed(() =>
    singleActionType.value === 'acknowledge'
      ? $t('rdi.overview.acknowledgeAlarm')
      : $t('rdi.overview.confirmResetAlarm')
  )

  const singleActionDialogHint = computed(() => {
    const row = singleActionRow.value
    if (!row) return ''
    const hint =
      singleActionType.value === 'acknowledge'
        ? $t('rdi.overview.alarmAuditConfirmHint')
        : $t('rdi.overview.alarmResetAuditHint')
    return `${alarmAuditSummary(row)}\n\n${hint}`
  })

  const closeSingleActionDialog = () => {
    if (singleActionLoading.value) return
    singleActionDialogVisible.value = false
    singleActionRow.value = null
    singleActionNote.value = ''
  }

  const openSingleAlarmAction = (row: AlarmSingleActionRow, action: AlarmSingleActionType) => {
    singleActionRow.value = row
    singleActionType.value = action
    singleActionNote.value = ''
    singleActionDialogVisible.value = true
  }

  const runSingleAlarmAction = async () => {
    const row = singleActionRow.value
    if (!row?.id) return
    singleActionLoading.value = true
    try {
      const note = singleActionNote.value.trim()
      const response: unknown = await batchActionAlarmHistory({
        ids: [row.id],
        action: singleActionType.value,
        ...(note ? { note } : {})
      })
      // 与拆分前保持一致的取值链：优先取包装响应的 data，否则回退到原始响应。
      const rawResponse = response as { data?: unknown } | null | undefined
      lastSingleClosureEvidence.value = {
        action: singleActionType.value,
        generatedAt: new Date().toISOString(),
        alarmId: row.id,
        note: note || '-',
        row: options.evidenceRowOf(row),
        response: rawResponse?.data || rawResponse || null,
        boundary: options.evidenceBoundaryLabel()
      }
      window.$message?.success(
        singleActionType.value === 'acknowledge'
          ? $t('rdi.overview.alarmAcknowledged')
          : $t('rdi.overview.alarmReset')
      )
      singleActionDialogVisible.value = false
      singleActionRow.value = null
      singleActionNote.value = ''
      options.closeDetailDialog()
      await options.refresh()
    } catch {
      window.$message?.error($t('custom.alarmPage.batchActionRequestFailed'))
    } finally {
      singleActionLoading.value = false
    }
  }

  return {
    singleActionDialogVisible,
    singleActionLoading,
    singleActionNote,
    singleActionNoteMaxLength,
    singleActionDialogTitle,
    singleActionDialogHint,
    lastSingleClosureEvidence,
    alarmAuditSummary,
    closeSingleActionDialog,
    openSingleAlarmAction,
    runSingleAlarmAction
  }
}
