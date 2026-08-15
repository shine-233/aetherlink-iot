import { NButton, NSpace, NTag } from 'naive-ui'
import { $t } from '@/locales'

export function createDeviceManageColumns(
  goDeviceDetails: (row: any) => void,
  editDevice: (row: any) => void,
  deleteDevice: (row: any) => void,
  shareDevice: (row: any) => void
) {
  return [
    {
      key: 'name',
      minWidth: '180px',
      label: () => $t('custom.devicePage.deviceName'),
      render: (row: any) => {
        return (
          <NButton type="primary" text onClick={() => goDeviceDetails(row)}>
            {row.name}
          </NButton>
        )
      }
    },
    {
      key: 'is_online',
      minWidth: '100px',
      label: () => $t('custom.devicePage.onlineStatus'),
      render: (row: any) => {
        if (row?.is_online === 1) {
          return (
            <NSpace>
              <NTag type="success">{$t('custom.devicePage.online')}</NTag>
            </NSpace>
          )
        }
        return (
          <NSpace>
            <NTag type="default" style="color: #999;">
              {$t('custom.devicePage.offline')}
            </NTag>
          </NSpace>
        )
      }
    },
    {
      key: 'pid_number',
      minWidth: '140px',
      label: () => $t('custom.devicePage.pidNumber'),
      render: (row: any) => row?.pid_number || row?.device_number || '-'
    },
    {
      key: 'current_version',
      minWidth: '140px',
      label: () => $t('custom.devicePage.firmwareVersion'),
      render: (row: any) => row?.current_version || '-'
    },
    {
      key: 'description',
      minWidth: '180px',
      label: () => $t('custom.devicePage.description'),
      render: (row: any) => row?.description || '-'
    },
    {
      key: 'shared_status',
      minWidth: '120px',
      label: () => $t('custom.devicePage.sharedStatus'),
      render: (row: any) => {
        if (row?.shared_status === 'shared') {
          return <NTag type="info">{$t('custom.devicePage.shared')}</NTag>
        }
        return <NTag type="default">{$t('custom.devicePage.unshared')}</NTag>
      }
    },
    {
      key: 'warn_status',
      minWidth: '100px',
      label: () => $t('custom.devicePage.alarmStatus'),
      render: (row: any) => {
        if (row?.warn_status === 'Y') {
          return (
            <NSpace>
              <NTag type="warning" style="color: #ff9900;">
                {$t('custom.devicePage.alarmed')}
              </NTag>
            </NSpace>
          )
        }
        return (
          <NSpace>
            <NTag type="default" style="color: #999;">
              {$t('custom.devicePage.notAlarmed')}
            </NTag>
          </NSpace>
        )
      }
    },
    {
      key: 'device_type',
      minWidth: '100px',
      label: () => $t('generate.device-type'),
      render: (row: any) => {
        if (row?.device_type === '1') return $t('custom.devicePage.directConnectedDevices')
        if (row?.device_type === '2') return $t('custom.devicePage.gateway')
        if (row?.device_type === '3') return $t('custom.devicePage.gatewaySubEquipment')
        return '-'
      }
    },
    {
      key: 'device_config_name',
      minWidth: '100px',
      label: () => $t('custom.devicePage.deviceConfig')
    },
    {
      key: 'device_type',
      minWidth: '160px',
      label: () => $t('custom.devicePage.accessServiceProtocol'),
      render: (row: any) => {
        if (row?.access_way === '') return '-'
        return row?.access_way === 'A'
          ? `${$t('custom.devicePage.byProtocol')}(${row?.protocol_type || '-'})`
          : `${$t('custom.devicePage.byService')}(${row?.protocol_type || '-'})`
      }
    },
    {
      key: 'ts',
      minWidth: '140px',
      label: () => $t('custom.devicePage.lastPushTime')
    },
    {
      key: 'actions',
      minWidth: '220px',
      label: () => $t('common.actions'),
      render: (row: any) => (
        <NSpace>
          <NButton
            text
            type="info"
            onClick={(event: MouseEvent) => {
              event.stopPropagation()
              shareDevice(row)
            }}
          >
            {$t('rdi.device.shareTitle')}
          </NButton>
          <NButton
            text
            type="primary"
            onClick={(event: MouseEvent) => {
              event.stopPropagation()
              editDevice(row)
            }}
          >
            {$t('common.edit')}
          </NButton>
          <NButton
            text
            type="error"
            onClick={(event: MouseEvent) => {
              event.stopPropagation()
              deleteDevice(row)
            }}
          >
            {$t('common.delete')}
          </NButton>
        </NSpace>
      )
    }
  ]
}
