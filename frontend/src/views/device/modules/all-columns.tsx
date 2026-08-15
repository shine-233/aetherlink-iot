// 文件用途：集中定义设备相关列表页的数据表格列配置。
// 核心逻辑：为设备分组、设备列表等页面生成 Naive UI 表格列，并绑定查看、删除等行级操作。
// 关键注意事项：列标题依赖国际化 key，操作按钮会触发表格行点击拦截；修改字段名需同步后端返回结构和页面筛选逻辑。
// 重构建议：建议后续把不同业务表格列拆成独立 factory，并为操作列和时间格式化补充单元测试。
import { type DataTableColumns, NButton, NFlex, NPopconfirm } from 'naive-ui'
import dayjs from 'dayjs'
import { $t } from '@/locales'

export const group_columns = (viewDetails: (rid: string) => void, deleteItem: (rid: string) => void) => [
  {
    title: () => $t('custom.groupPage.groupName'),
    key: 'name',
    minWidth: '140px',
    ellipsis: {
      tooltip: {
        width: 320
      }
    }
  },
  {
    title: () => $t('custom.groupPage.description'),
    key: 'description',
    minWidth: '140px',
    ellipsis: {
      tooltip: {
        width: 320
      }
    }
  },
  {
    title: () => $t('custom.groupPage.createdAt'),
    key: 'created_at',
    minWidth: '180px',
    render(row: { id: string; name: string; description: string; created_at: string; [key: string]: any }) {
      return dayjs(row.created_at).format('YYYY-MM-DD HH:mm:ss')
    }
  },
  {
    title: () => $t('custom.groupPage.actions'),
    key: 'actions',
    width: '200px',
    render: (row: { id: string; name: string; description: string; created_at: string; [key: string]: any }) => {
      return (
        <div
          onClick={e => {
            e.stopPropagation()
          }}
        >
          <NFlex justify={'start'}>
            <NButton
              type="primary"
              size={'small'}
              onClick={() => {
                viewDetails(row.id)
              }}
            >
              {$t('custom.groupPage.view')}
            </NButton>
            <NPopconfirm
              onPositiveClick={e => {
                e.stopPropagation()
                deleteItem(row.id)
              }}
            >
              {{
                default: () => $t('common.confirmDelete'),
                trigger: () => (
                  <NButton type="error" size={'small'}>
                    {$t('common.delete')}
                  </NButton>
                )
              }}
            </NPopconfirm>
          </NFlex>
        </div>
      )
    }
  }
]

export const createDeviceColumns = (): DataTableColumns<DeviceManagement.DeviceData> => [
  {
    type: 'selection',
    minWidth: '140px'
  },
  {
    title: () => $t('custom.devicePage.deviceName'),
    key: 'name',
    minWidth: '140px',
    render: row => row.name || '-'
  },
  {
    title: () => $t('custom.devicePage.deviceNumber'),
    key: 'device_number',
    minWidth: '140px',
    render: row => row.device_number || '-'
  },
  {
    title: () => $t('custom.devicePage.deviceConfig'),
    minWidth: '140px',
    key: 'device_config_name'
  }
]

export const createNoSelectDeviceColumns = (
  viewDevicsseDetails: (rid: string) => void,
  deleteDeviceItem: (rid: string) => void
): DataTableColumns<DeviceManagement.DeviceData> => {
  return [
    {
      title: () => $t('custom.devicePage.deviceName'),
      key: 'name',
      minWidth: '140px',
      render: row => row.name || '-'
    },
    {
      title: () => $t('custom.devicePage.deviceNumber'),
      key: 'device_number',
      minWidth: '140px',
      render: row => row.device_number || '-'
    },
    {
      title: () => $t('custom.devicePage.deviceConfig'),
      minWidth: '140px',
      key: 'device_config_name'
    },
    {
      title: () => $t('custom.groupPage.actions'),
      key: 'actions',
      minWidth: '140px',
      render: row => {
        return (
          <NFlex justify={'start'}>
            <NButton
              type="primary"
              size={'small'}
              onClick={() => {
                viewDevicsseDetails(row.id)
              }}
            >
              {$t('custom.groupPage.view')}
            </NButton>
            <NPopconfirm
              onPositiveClick={() => {
                deleteDeviceItem(row.id)
              }}
            >
              {{
                default: () => $t('common.confirmDelete'),
                trigger: () => (
                  <NButton type="error" size={'small'}>
                    {$t('custom.groupPage.removeFromGroup')}
                  </NButton>
                )
              }}
            </NPopconfirm>
          </NFlex>
        )
      }
    }
  ]
}
