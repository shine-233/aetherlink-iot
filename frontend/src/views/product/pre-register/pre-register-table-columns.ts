/**
 * 文件用途: 预注册设备列表的表格列工厂，保持列定义与页面解耦以便单测锁定。
 */
import type { DataTableColumns } from 'naive-ui'
import dayjs from 'dayjs'
import { $t } from '@/locales'
import type { PreRegisterRecord } from './types'

interface ColumnHandlers {
  formatTime: (value?: string | null) => string
}

export function createPreRegisterColumns({ formatTime }: ColumnHandlers): DataTableColumns<PreRegisterRecord> {
  return [
    { type: 'selection' },
    {
      title: $t('page.product.pre-register.deviceNumber'),
      key: 'device_number',
      width: 180,
      ellipsis: { tooltip: true }
    },
    {
      title: $t('page.product.pre-register.name'),
      key: 'name',
      minWidth: 160,
      ellipsis: { tooltip: true }
    },
    {
      title: $t('page.product.pre-register.batchNumber'),
      key: 'batch_number',
      width: 150,
      ellipsis: { tooltip: true }
    },
    {
      title: $t('page.product.pre-register.currentVersion'),
      key: 'current_version',
      width: 120,
      render: (row) => row.current_version || '-'
    },
    {
      title: $t('page.product.pre-register.activateFlag'),
      key: 'activate_flag',
      width: 110,
      render: (row) =>
        row.activate_flag === 'active'
          ? $t('page.product.pre-register.activated')
          : $t('page.product.pre-register.waitingActivate')
    },
    {
      title: $t('page.product.pre-register.activateAt'),
      key: 'activate_at',
      width: 170,
      render: (row) => formatTime(row.activate_at)
    },
    {
      title: $t('page.product.pre-register.createdAt'),
      key: 'created_at',
      width: 170,
      render: (row) => formatTime(row.created_at)
    }
  ]
}

export function formatPreRegisterTime(value?: string | null) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'
}
