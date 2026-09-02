import { h } from 'vue'
import { NButton, NSpace, NTag, type DataTableColumns } from 'naive-ui'
import type {
  CommandJobRowsStatusFilter,
  FleetCommandJobListItem,
  FleetCommandJobPreviewRow,
  FleetCommandJobSubmitRow
} from '@/service/api/device'
import {
  buildCommandJobHistoryAttentionSummary,
  buildCommandJobHistoryProgress,
  buildCommandJobSubmitNextAction,
  formatCommandJobDateTime,
  formatCommandJobReadiness,
  formatCommandJobResponseStatus,
  formatCommandJobStatus
} from './commandCenterJobView'

type Translate = (key: string) => string

interface CommandCenterPreviewColumnOptions {
  t: Translate
  formatRecommendedPath: (path?: FleetCommandJobPreviewRow['recommended_path']) => string
  formatTelemetryEvidence: (row: FleetCommandJobPreviewRow) => string
}

interface CommandCenterJobHistoryColumnOptions {
  t: Translate
  openCommandJobDetail: (
    jobId: string,
    options?: { rowsStatusFilter?: CommandJobRowsStatusFilter; rowsSearch?: string }
  ) => void
  reuseCommandJobDraft: (job: FleetCommandJobListItem) => void
  saveCommandJobTemplate: (job: FleetCommandJobListItem) => void
}

export const createCommandJobPreviewColumns = (
  options: CommandCenterPreviewColumnOptions
): DataTableColumns<FleetCommandJobPreviewRow> => [
  {
    title: options.t('custom.commandCenter.device'),
    key: 'device_number',
    render: (row) => row.device_number || row.name || row.device_id
  },
  {
    title: options.t('custom.commandCenter.online'),
    key: 'online',
    render: (row) => (row.online ? options.t('common.yesOrNo.yes') : options.t('common.yesOrNo.no'))
  },
  {
    title: options.t('custom.commandCenter.eligible'),
    key: 'eligible',
    render: (row) => (row.eligible ? options.t('common.yesOrNo.yes') : options.t('common.yesOrNo.no'))
  },
  {
    title: options.t('custom.commandCenter.readinessEvidence'),
    key: 'readiness',
    render: (row) => formatCommandJobReadiness(row.readiness)
  },
  {
    title: options.t('custom.commandCenter.recommendedPath'),
    key: 'recommended_path',
    render: (row) => options.formatRecommendedPath(row.recommended_path)
  },
  {
    title: options.t('custom.commandCenter.telemetryEvidenceColumn'),
    key: 'telemetry_current_count',
    render: (row) => options.formatTelemetryEvidence(row)
  },
  { title: options.t('common.status'), key: 'status' },
  { title: options.t('custom.commandCenter.advice'), key: 'advice', render: (row) => row.advice || '-' },
  { title: options.t('custom.commandCenter.reason'), key: 'reason', render: (row) => row.reason || '-' }
]

export const createCommandJobSubmitColumns = (t: Translate): DataTableColumns<FleetCommandJobSubmitRow> => [
  {
    title: t('custom.commandCenter.device'),
    key: 'device_number',
    render: (row) => row.device_number || row.name || row.device_id
  },
  { title: t('common.status'), key: 'status' },
  {
    title: t('custom.commandCenter.readinessEvidence'),
    key: 'readiness',
    render: (row) => formatCommandJobReadiness(row.readiness)
  },
  { title: t('custom.commandCenter.messageId'), key: 'message_id', render: (row) => row.message_id || '-' },
  {
    title: t('custom.commandCenter.deviceResponseStatus'),
    key: 'response_status_label',
    render: (row) => formatCommandJobResponseStatus(row.response_status_label, t)
  },
  {
    title: t('custom.commandCenter.deviceResponseEvidence'),
    key: 'response_data',
    ellipsis: { tooltip: true },
    render: (row) => row.response_error || row.response_data || '-'
  },
  {
    title: t('custom.commandCenter.logRecorded'),
    key: 'log_recorded',
    render: (row) => (row.log_recorded ? t('common.yesOrNo.yes') : t('common.yesOrNo.no'))
  },
  { title: t('custom.commandCenter.reason'), key: 'reason', render: (row) => row.reason || '-' },
  {
    title: t('custom.commandCenter.nextAction'),
    key: 'next_action',
    render: (row) => buildCommandJobSubmitNextAction(row, t)
  },
  {
    title: t('custom.commandCenter.canRetry'),
    key: 'can_retry',
    render: (row) => (row.can_retry ? t('common.yesOrNo.yes') : t('common.yesOrNo.no'))
  }
]

export const createCommandJobHistoryColumns = (
  options: CommandCenterJobHistoryColumnOptions
): DataTableColumns<FleetCommandJobListItem> => [
  { title: options.t('custom.commandCenter.jobId'), key: 'job_id', ellipsis: { tooltip: true } },
  { title: options.t('custom.commandCenter.commandIdentifier'), key: 'identify', ellipsis: { tooltip: true } },
  {
    title: options.t('common.status'),
    key: 'status',
    render: (row) =>
      h(
        NTag,
        { size: 'small', type: row.status === 'completed' ? 'success' : row.status === 'failed' ? 'error' : 'info' },
        { default: () => formatCommandJobStatus(row.status, options.t) }
      )
  },
  {
    title: options.t('custom.commandCenter.jobRequested'),
    key: 'requested_count',
    render: (row) => buildCommandJobHistoryProgress(row, options.t)
  },
  {
    title: options.t('custom.commandCenter.jobAttentionColumn'),
    key: 'needs_operator_action_count',
    render: (row) =>
      h(
        NTag,
        { size: 'small', type: row.needs_operator_action ? 'warning' : 'success' },
        { default: () => buildCommandJobHistoryAttentionSummary(row, options.t) }
      )
  },
  {
    title: options.t('custom.commandCenter.scheduledAt'),
    key: 'scheduled_at',
    render: (row) => formatCommandJobDateTime(row.scheduled_at)
  },
  {
    title: options.t('custom.commandCenter.updatedAt'),
    key: 'updated_at',
    render: (row) => formatCommandJobDateTime(row.updated_at)
  },
  {
    title: options.t('common.actions'),
    key: 'actions',
    render: (row) =>
      h(
        NSpace,
        { size: 8, wrap: false },
        {
          default: () => [
            h(
              NButton,
              { size: 'small', secondary: true, onClick: () => options.reuseCommandJobDraft(row) },
              { default: () => options.t('custom.commandCenter.reuseJobDraft') }
            ),
            h(
              NButton,
              { size: 'small', secondary: true, onClick: () => options.saveCommandJobTemplate(row) },
              { default: () => options.t('custom.commandCenter.saveJobAsTemplate') }
            ),
            row.needs_operator_action
              ? h(
                  NButton,
                  {
                    size: 'small',
                    secondary: true,
                    type: 'warning',
                    onClick: () =>
                      options.openCommandJobDetail(row.job_id, { rowsStatusFilter: 'needs_attention', rowsSearch: '' })
                  },
                  { default: () => options.t('custom.commandCenter.reviewJobAttention') }
                )
              : null,
            h(
              NButton,
              { size: 'small', secondary: true, onClick: () => options.openCommandJobDetail(row.job_id) },
              { default: () => options.t('custom.commandCenter.openJobDetail') }
            )
          ]
        }
      )
  }
]
