/**
 * 文件用途：定时报表页两张表（计划列表、运行历史）的列定义。
 * 核心逻辑：列渲染只依赖传入的状态读取函数与动作回调，便于在页面外单测与复用。
 */
import { h } from 'vue'
import { NButton, NPopconfirm, NSpace, NTag, type DataTableColumns } from 'naive-ui'
import { $t } from '@/locales'
import type { ReportRun, ReportRunStatus, ReportSchedule } from '@/service/api/report'
import { canRetryReportRun, reportStatusTagType } from './report-model'
import { formatReportTime, reportRunWindow, reportStatusText } from './report-helpers'

export interface ScheduleColumnHandlers {
  runningId: () => string
  deletingId: () => string
  onRun: (row: ReportSchedule) => void
  onHistory: (row: ReportSchedule) => void
  onEdit: (row: ReportSchedule) => void
  onDelete: (row: ReportSchedule) => void
}

const statusTag = (status?: ReportRunStatus | null) =>
  h(NTag, { size: 'small', type: reportStatusTagType(status) }, { default: () => reportStatusText(status) })

const smallButton = (label: string, props: Record<string, unknown>) =>
  h(NButton, { size: 'small', ...props }, { default: () => label })

export function createScheduleColumns(handlers: ScheduleColumnHandlers): DataTableColumns<ReportSchedule> {
  return [
    {
      title: $t('report.table.name'),
      key: 'name',
      minWidth: 180,
      render: (row) =>
        h('div', [h('strong', row.name), h('div', { class: 'report-muted' }, `${row.cron_expr} · ${row.timezone}`)])
    },
    {
      title: $t('report.table.delivery'),
      key: 'delivery',
      minWidth: 180,
      render: (row) =>
        h('div', [
          h('div', row.recipients),
          h(
            'div',
            { class: 'report-muted' },
            $t('report.table.targetSummary', { devices: row.device_ids.length, keys: row.keys.length })
          )
        ])
    },
    {
      title: $t('report.table.nextRun'),
      key: 'next_run_at',
      width: 165,
      render: (row) => formatReportTime(row.next_run_at)
    },
    {
      title: $t('report.table.lastRun'),
      key: 'last_status',
      width: 150,
      render: (row) =>
        h('div', [statusTag(row.last_status), h('div', { class: 'report-muted' }, formatReportTime(row.last_run_at))])
    },
    {
      title: $t('report.table.enabled'),
      key: 'enabled',
      width: 90,
      render: (row) =>
        h(
          NTag,
          { type: row.enabled ? 'success' : 'default', size: 'small' },
          { default: () => $t(row.enabled ? 'report.common.enabled' : 'report.common.disabled') }
        )
    },
    {
      title: $t('report.table.actions'),
      key: 'actions',
      width: 310,
      fixed: 'right',
      render: (row) =>
        h(
          NSpace,
          { size: 6, wrap: true },
          {
            default: () => [
              smallButton($t('report.action.runNow'), {
                type: 'primary',
                loading: handlers.runningId() === row.id,
                disabled: Boolean(handlers.runningId()) || !row.enabled,
                onClick: () => handlers.onRun(row)
              }),
              smallButton($t('report.action.history'), { onClick: () => handlers.onHistory(row) }),
              smallButton($t('report.action.edit'), { onClick: () => handlers.onEdit(row) }),
              h(
                NPopconfirm,
                { onPositiveClick: () => handlers.onDelete(row) },
                {
                  trigger: () =>
                    smallButton($t('report.action.delete'), {
                      type: 'error',
                      loading: handlers.deletingId() === row.id
                    }),
                  default: () => $t('report.message.deleteConfirm')
                }
              )
            ]
          }
        )
    }
  ]
}

export interface RunColumnHandlers {
  scheduleEnabled: () => boolean | undefined
  retryingRunId: () => string
  onSelect: (run: ReportRun) => void
  onRetry: (run: ReportRun) => void
}

export function createRunColumns(handlers: RunColumnHandlers): DataTableColumns<ReportRun> {
  return [
    {
      title: $t('report.history.started'),
      key: 'created_at',
      width: 170,
      render: (row) => formatReportTime(row.created_at)
    },
    { title: $t('report.history.window'), key: 'window', minWidth: 220, render: reportRunWindow },
    {
      title: $t('report.history.overall'),
      key: 'overall_status',
      width: 120,
      render: (row) => statusTag(row.overall_status)
    },
    {
      title: $t('report.history.generation'),
      key: 'generation_status',
      width: 130,
      render: (row) => reportStatusText(row.generation_status)
    },
    {
      title: $t('report.history.delivery'),
      key: 'delivery_status',
      width: 130,
      render: (row) => reportStatusText(row.delivery_status)
    },
    {
      title: $t('report.history.risk'),
      key: 'duplicate_delivery_risk',
      width: 120,
      render: (row) =>
        row.duplicate_delivery_risk
          ? h(NTag, { type: 'warning', size: 'small' }, { default: () => $t('report.risk.duplicate') })
          : $t('report.risk.none')
    },
    {
      title: $t('report.table.actions'),
      key: 'actions',
      width: 170,
      render: (row) =>
        h(
          NSpace,
          { size: 6 },
          {
            default: () => [
              smallButton($t('report.action.details'), { onClick: () => handlers.onSelect(row) }),
              canRetryReportRun(row, handlers.scheduleEnabled())
                ? smallButton($t('report.action.retry'), {
                    type: 'warning',
                    loading: handlers.retryingRunId() === row.run_id,
                    onClick: () => handlers.onRetry(row)
                  })
                : null
            ]
          }
        )
    }
  ]
}
