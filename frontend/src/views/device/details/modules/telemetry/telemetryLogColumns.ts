import dayjs from 'dayjs'

import { $t } from '@/locales'

export const createTelemetryLogColumns = () => [
  {
    title: $t('custom.device_details.command'),
    minWidth: '140px',
    key: 'data'
  },
  {
    title: $t('custom.device_details.operationType'),
    key: 'operation_type',
    minWidth: '140px',
    render: (row) =>
      row.operation_type === '1' ? $t('custom.device_details.manualOperation') : $t('card.triggerAction')
  },
  {
    title: $t('custom.device_details.operationUsers'),
    minWidth: '140px',
    key: 'username',
    render: (row) => (row.operation_type === '1' ? row.username : $t('generate.system'))
  },
  {
    title: $t('custom.device_details.operationTime'),
    key: 'created_at',
    minWidth: '140px',
    render: (row) => dayjs(row.created_at).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    title: $t('custom.device_details.sendResults'),
    minWidth: '140px',
    key: 'status',
    render: (row) => {
      if (row.status === '1') return $t('custom.devicePage.success')
      if (row.status === '2') return $t('custom.devicePage.fail')
      return $t('page.expect.pending')
    }
  }
]
