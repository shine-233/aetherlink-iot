// 文件用途：集中定义设备相关列表页的数据表格列配置。
// 核心逻辑：为设备分组、设备列表等页面生成 Naive UI 表格列，并绑定查看、删除等行级操作。
// 关键注意事项：列标题依赖国际化 key，操作按钮会触发表格行点击拦截；修改字段名需同步后端返回结构和页面筛选逻辑。
// 重构建议：建议后续把不同业务表格列拆成独立 factory，并为操作列和时间格式化补充单元测试。
import { type DataTableColumns, NButton, NFlex, NPopconfirm, NTag } from 'naive-ui'
import dayjs from 'dayjs'
import { $t } from '@/locales'

/** 分组统计的行类型（后端 GET /device/group 列表项内嵌 statistics）。 */
interface GroupStatisticsRow {
  id: string
  name: string
  description: string
  created_at: string
  statistics?: {
    device_total?: number
    online_total?: number
    offline_total?: number
    alarm_total?: number
  }
  [key: string]: any
}

/**
 * 统计列。后端把统计挂成列表项的 statistics 字段（ROADMAP TP-8②）；
 * 缺失时显示 0 而不是留空 —— 留空会被读成"没这项能力"，0 才是事实。
 */
const statisticsColumns = [
  {
    title: () => $t('custom.groupPage.statDeviceTotal'),
    key: 'stat_device_total',
    width: '100px',
    render(row: GroupStatisticsRow) {
      return String(row.statistics?.device_total ?? 0)
    }
  },
  {
    title: () => $t('custom.groupPage.statOnline'),
    key: 'stat_online_total',
    width: '90px',
    render(row: GroupStatisticsRow) {
      const value = row.statistics?.online_total ?? 0
      return <NTag type={value > 0 ? 'success' : 'default'} size="small">{String(value)}</NTag>
    }
  },
  {
    title: () => $t('custom.groupPage.statOffline'),
    key: 'stat_offline_total',
    width: '90px',
    render(row: GroupStatisticsRow) {
      return String(row.statistics?.offline_total ?? 0)
    }
  },
  {
    title: () => $t('custom.groupPage.statAlarm'),
    key: 'stat_alarm_total',
    width: '90px',
    render(row: GroupStatisticsRow) {
      const value = row.statistics?.alarm_total ?? 0
      return <NTag type={value > 0 ? 'error' : 'default'} size="small">{String(value)}</NTag>
    }
  }
]

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
  // ROADMAP TP-8②：分组列表展示统计（后端已随列表项返回 statistics）。
  ...statisticsColumns,
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
