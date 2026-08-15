import { h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NTag } from 'naive-ui'
import { $t } from '@/locales'
import type { OtaPackageRecord } from './ota-package-types'

interface CreateOtaPackageColumnsOptions {
  formatTime: (value?: string) => string
  packageTypeLabel: (value?: number) => string
  openDetailModal: (row: OtaPackageRecord) => void
  downloadPackage: (row: OtaPackageRecord) => void
  openEditModal: (row: OtaPackageRecord) => void
  deletePackage: (row: OtaPackageRecord) => void
}

export function createOtaPackageColumns(options: CreateOtaPackageColumnsOptions): DataTableColumns<OtaPackageRecord> {
  return [
    {
      key: 'name',
      title: () => $t('page.product.update-package.packageName'),
      minWidth: 180,
      ellipsis: { tooltip: true },
      render: (row) => row.name || '-'
    },
    {
      key: 'version',
      title: () => $t('page.product.update-package.versionCode'),
      width: 150,
      render: (row) => row.version || '-'
    },
    {
      key: 'target_version',
      title: () => $t('page.product.update-package.version'),
      width: 150,
      render: (row) => row.target_version || '-'
    },
    {
      key: 'device_config_name',
      title: () => $t('page.product.update-package.deviceConfig'),
      minWidth: 180,
      render: (row) => row.device_config_name || row.device_config_id || '-'
    },
    {
      key: 'package_type',
      title: () => $t('page.product.update-package.type'),
      width: 120,
      render: (row) =>
        h(
          NTag,
          { type: row.package_type === 1 ? 'warning' : 'success' },
          { default: () => options.packageTypeLabel(row.package_type) }
        )
    },
    {
      key: 'signature',
      title: () => $t('page.product.update-ota.packageSign'),
      minWidth: 170,
      ellipsis: { tooltip: true },
      render: (row) => row.signature || '-'
    },
    {
      key: 'created_at',
      title: () => $t('page.product.update-package.createTime'),
      width: 180,
      render: (row) => options.formatTime(row.created_at)
    },
    {
      key: 'actions',
      title: () => $t('common.actions'),
      width: 330,
      fixed: 'right',
      render: (row) =>
        h('div', { class: 'action-row' }, [
          h(
            NButton,
            { size: 'small', onClick: () => options.openDetailModal(row) },
            { default: () => $t('page.product.update-package.detail') }
          ),
          h(
            NButton,
            { size: 'small', onClick: () => options.downloadPackage(row), disabled: !row.package_url },
            { default: () => $t('page.product.update-ota.download') }
          ),
          h(
            NButton,
            { size: 'small', type: 'primary', onClick: () => options.openEditModal(row) },
            { default: () => $t('common.edit') }
          ),
          h(
            NButton,
            { size: 'small', type: 'error', onClick: () => options.deletePackage(row) },
            { default: () => $t('common.delete') }
          )
        ])
    }
  ]
}
