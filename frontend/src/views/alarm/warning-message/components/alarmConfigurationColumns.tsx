import { NButton, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import dayjs from 'dayjs'
import { $t } from '@/locales'
import {
  alarmActionField,
  alarmSeverityLabel,
  alarmSeverityTagType,
  alarmSeverityValue,
  alarmTypeLabel,
  buildAlarmClosureNextAction,
  isAcknowledged,
  isReset,
  isCleared,
  type AlarmOption
} from './alarm-configuration.helpers'

export interface AlarmConfigurationRow {
  id: string
  create_at?: string
  time: string
  name: string
  content?: string
  description: string
  alarm_level: string
  alarm_status: string
  lifecycle_status?: string
  alarm_config_name?: string
  notification_group_id: string
  enabled: string
  result: string
  handler: string
  remark?: string | Record<string, unknown>
  // SLA 计时字段（TB-27，126.sql）：sla_due_at 为到期时间（未启用 SLA 时为空），
  // sla_breached 为 cron 超时升级标记。
  sla_due_at?: string | null
  sla_breached?: boolean
}

export type AlarmConfigurationColumnHandlers = {
  getAlarmStatusOptions: () => AlarmOption[]
  onShowDetails: (row: AlarmConfigurationRow) => void
  onAcknowledge: (row: AlarmConfigurationRow) => void
  onReset: (row: AlarmConfigurationRow) => void
  onClear?: (row: AlarmConfigurationRow) => void
  onMaintenance: (row: AlarmConfigurationRow) => void
}

export function createAlarmConfigurationColumns(
  handlers: AlarmConfigurationColumnHandlers
): DataTableColumns<AlarmConfigurationRow> {
  return [
    {
      type: 'selection'
    },
    {
      key: 'alarm_time',
      title: $t('common.alarm_time'),
      align: 'left',
      minWidth: '170px',
      render(row: AlarmConfigurationRow) {
        return dayjs(row.create_at).format('YYYY-MM-DD HH:mm:ss')
      }
    },
    {
      key: 'name',
      title: $t('generate.alarm-name'),
      align: 'left',
      minWidth: '100px',
      ellipsis: {
        tooltip: true
      }
    },
    {
      key: 'alarm_level',
      title: $t('common.alarm_level'),
      align: 'left',
      minWidth: '120px',
      render(row: AlarmConfigurationRow) {
        const severity = alarmSeverityValue(row)
        return (
          <NTag type={alarmSeverityTagType(severity)}>
            {alarmSeverityLabel(severity, handlers.getAlarmStatusOptions())}
          </NTag>
        )
      }
    },
    {
      key: 'alarm_type',
      title: $t('rdi.overview.alarmType'),
      align: 'left',
      minWidth: '150px',
      ellipsis: {
        tooltip: true
      },
      render: row => alarmTypeLabel(row, $t)
    },
    // SLA 到期/超时列（TB-27，126.sql）：已 breach 显示红色"SLA 超时"标记；
    // 已配置但未到期显示到期时间；未启用 SLA 显示 "-"。
    {
      key: 'sla_due_at',
      title: $t('custom.alarmPage.slaDueAtColumn'),
      align: 'left',
      minWidth: '160px',
      render(row: AlarmConfigurationRow) {
        if (row.sla_breached) {
          return (
            <NTag type="error" size="small" data-testid="alarm-sla-breached">
              {$t('custom.alarmPage.slaBreachedTag')}
            </NTag>
          )
        }
        if (row.sla_due_at) {
          return (
            <span class="alarm-sla-due-at" data-testid="alarm-sla-due-at">
              {dayjs(row.sla_due_at as any).format('YYYY-MM-DD HH:mm')}
            </span>
          )
        }
        return '-'
      }
    },
    {
      key: 'content',
      title: $t('generate.alarm-content'),
      align: 'left',
      minWidth: '100px',
      ellipsis: {
        tooltip: true
      }
    },
    {
      key: 'description',
      title: $t('generate.alarm-description'),
      align: 'left',
      minWidth: '80px',
      ellipsis: {
        tooltip: true
      }
    },
    {
      key: 'acknowledged_at',
      title: $t('rdi.overview.acknowledgedAt'),
      align: 'left',
      minWidth: '170px',
      render: row => alarmActionField(row, 'acknowledged_at')
    },
    {
      key: 'reset_at',
      title: $t('rdi.overview.resetAt'),
      align: 'left',
      minWidth: '170px',
      render: row => alarmActionField(row, 'reset_at')
    },
    {
      key: 'closure_next_action',
      title: $t('custom.alarmPage.closureNextActionColumn'),
      align: 'left',
      minWidth: '260px',
      render(row: AlarmConfigurationRow) {
        const action = buildAlarmClosureNextAction(row, $t, value =>
          value ? dayjs(value as any).format('YYYY-MM-DD HH:mm:ss') : '-'
        )
        return (
          <div class="alarm-closure-next-action" data-testid="alarm-closure-next-action">
            <NTag type={action.type} size="small">
              {action.status}
            </NTag>
            <span class="alarm-closure-next-action__step">{action.nextStep}</span>
            <span class="alarm-closure-next-action__evidence">{action.evidence}</span>
          </div>
        )
      }
    },
    {
      key: 'actions',
      title: $t('common.actions'),
      width: '420px',
      align: 'left',
      render: row => {
        return (
          <div class="flex flex-wrap gap-8px">
            <NButton type="primary" size="small" data-testid="alarm-details" onClick={() => handlers.onShowDetails(row)}>
              {$t('custom.devicePage.details')}
            </NButton>
            <NButton
              type="success"
              size="small"
              data-testid="alarm-acknowledge"
              disabled={isAcknowledged(row)}
              onClick={() => handlers.onAcknowledge(row)}
            >
              {$t('common.confirm')}
            </NButton>
            <NButton type="error" size="small" data-testid="alarm-reset" disabled={isReset(row)} onClick={() => handlers.onReset(row)}>
              {$t('common.reset')}
            </NButton>
            {handlers.onClear && (
              <NButton
                type="info"
                size="small"
                data-testid="alarm-clear"
                disabled={isCleared(row)}
                onClick={() => handlers.onClear!(row)}
              >
                {$t('common.clear') || '清除'}
              </NButton>
            )}
            <NButton type="warning" size="small" data-testid="alarm-maintenance-note" onClick={() => handlers.onMaintenance(row)}>
              {$t('common.maintenance')}
            </NButton>
          </div>
        )
      }
    }
  ]
}
