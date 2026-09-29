/**
 * 文件用途：RDI 概览告警表的列定义。
 * 核心逻辑：主账号查看跨租户告警时在设备列附加租户标识；确认/复位按钮按告警状态禁用。
 */
import { h, type Ref } from 'vue'
import { NButton, NTag, type DataTableColumns } from 'naive-ui'
import dayjs from 'dayjs'
import { $t } from '@/locales'
import {
  alarmStatusLabel,
  alarmTagType,
  alarmTypeLabel,
  isAcknowledgedAlarm,
  rowText,
  type AlarmRecord
} from './rdiOverviewState'

export const formatRdiTime = (value?: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-')

export interface RdiAlarmColumnHandlers {
  isMasterAccount: Ref<boolean>
  onOpenDevice: (deviceId?: string) => void
  onAcknowledge: (row: AlarmRecord) => void
  onReset: (row: AlarmRecord) => void
}

export function createRdiAlarmColumns(handlers: RdiAlarmColumnHandlers): DataTableColumns<AlarmRecord> {
  return [
    { key: 'create_at', title: () => $t('common.time'), minWidth: 170, render: (row) => formatRdiTime(row.create_at) },
    {
      key: 'name',
      title: () => $t('rdi.overview.alarm'),
      minWidth: 180,
      render: (row) => row.name || row.content || '-'
    },
    {
      key: 'alarm_status',
      title: () => $t('common.alarm_level'),
      width: 120,
      render: (row) =>
        h(NTag, { type: alarmTagType(row.alarm_status) }, { default: () => alarmStatusLabel(row.alarm_status, $t) })
    },
    {
      key: 'alarm_type',
      title: () => $t('rdi.overview.alarmType'),
      minWidth: 160,
      render: (row) => alarmTypeLabel(row, $t)
    },
    {
      key: 'devices',
      title: () => $t('rdi.overview.device'),
      minWidth: 180,
      render: (row) => {
        const firstDevice = row.alarm_device_list?.[0]
        if (!firstDevice) return '-'
        const tenantId = rowText(row as unknown as Record<string, unknown>, ['tenant_id', 'TenantID'], '')
        const deviceLabel = firstDevice.name || firstDevice.id
        const label = handlers.isMasterAccount.value && tenantId ? `${deviceLabel} · ${tenantId}` : deviceLabel
        return h(
          NButton,
          { text: true, type: 'primary', onClick: () => handlers.onOpenDevice(firstDevice.id) },
          { default: () => label }
        )
      }
    },
    {
      key: 'description',
      title: () => $t('rdi.overview.description'),
      minWidth: 200,
      ellipsis: { tooltip: true },
      render: (row) => row.description || '-'
    },
    {
      key: 'actions',
      title: () => $t('common.actions'),
      width: 180,
      render: (row) =>
        h('div', { class: 'action-row' }, [
          h(
            NButton,
            {
              size: 'small',
              type: 'success',
              disabled: isAcknowledgedAlarm(row),
              onClick: () => handlers.onAcknowledge(row)
            },
            { default: () => $t('rdi.overview.acknowledgeAlarm') }
          ),
          h(
            NButton,
            { size: 'small', type: 'error', disabled: row.alarm_status === 'N', onClick: () => handlers.onReset(row) },
            { default: () => $t('rdi.overview.resetAlarm') }
          )
        ])
    }
  ]
}
