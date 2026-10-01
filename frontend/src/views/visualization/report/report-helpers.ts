/**
 * 文件用途：定时报表页的纯辅助函数（格式归一、表单行解析、错误码识别、展示文案）。
 * 关键注意事项：report-model.ts 在页面测试里被整体 mock，因此页面编排用到的新辅助放在本文件。
 */
import type { SelectOption } from 'naive-ui'
import { $t } from '@/locales'
import type { ReportRun, ReportSchedule, ReportScheduleFormat, ReportSchedulePayload } from '@/service/api/report'

export const REPORT_PAGE_SIZE = 10
export const REPORT_RUN_PAGE_SIZE = 10

// TB-49：报表格式选项与后端 oneof=csv html pdf 同口径。标签为通用缩写，以常量落地（不新增 locale 键）。
export const REPORT_FORMAT_OPTIONS: Array<SelectOption & { value: ReportScheduleFormat }> = [
  { label: 'CSV', value: 'csv' },
  { label: 'HTML', value: 'html' },
  { label: 'PDF', value: 'pdf' }
]

/** 编辑回填时归一化历史数据里的宽松取值：未知/缺失格式回落 CSV（与后端 fail-closed 一致）。 */
export const normalizeReportFormat = (value?: string | null): ReportScheduleFormat =>
  value === 'html' || value === 'pdf' ? value : 'csv'

export const emptyReportForm = (): ReportSchedulePayload => ({
  name: '',
  cron_expr: '',
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
  recipients: '',
  device_ids: [],
  keys: [],
  lookback_hours: 24,
  format: 'csv',
  enabled: true
})

/** 从已有计划（或空表单）生成可编辑的表单快照；编辑时携带 revision 以便乐观并发校验。 */
export const reportFormFromSchedule = (schedule?: ReportSchedule): ReportSchedulePayload => {
  const source = schedule || emptyReportForm()
  return {
    name: source.name,
    cron_expr: source.cron_expr,
    timezone: source.timezone || 'UTC',
    recipients: source.recipients,
    device_ids: [...source.device_ids],
    keys: [...source.keys],
    lookback_hours: source.lookback_hours,
    format: normalizeReportFormat(source.format),
    enabled: source.enabled,
    ...(schedule ? { revision: schedule.revision } : {})
  }
}

/** 多行/逗号分隔文本 → 去空去重的列表。 */
export const splitReportLines = (value: string) => [
  ...new Set(
    value
      .split(/[\n,]/)
      .map((item) => item.trim())
      .filter(Boolean)
  )
]

export const REPORT_REVISION_CONFLICT_CODE = 201002

type ReportRequestError = {
  code?: number
  data?: { code?: number }
  response?: { data?: { code?: number } }
}

export const reportErrorCode = (error: unknown) => {
  if (!error || typeof error !== 'object') return undefined
  const value = error as ReportRequestError
  return value.response?.data?.code ?? value.data?.code ?? value.code
}

/** 以结构化错误码识别修订冲突，不依赖（可能被翻译的）错误文案。 */
export const isReportRevisionConflict = (error: unknown) => reportErrorCode(error) === REPORT_REVISION_CONFLICT_CODE

export const formatReportTime = (value?: string | null) =>
  value ? new Date(value).toLocaleString() : $t('report.common.notAvailable')
export const reportStatusText = (value?: string | null) =>
  value ? $t(`report.status.${value}`, value) : $t('report.status.never')
export const reportRunWindow = (run: ReportRun) =>
  `${formatReportTime(run.window_start_at)} — ${formatReportTime(run.window_end_at)}`
export const isSmtpAccepted = (run: ReportRun) => run.delivery_status === 'accepted'
export const isSmtpAmbiguous = (run: ReportRun) => run.delivery_status === 'ambiguous'
