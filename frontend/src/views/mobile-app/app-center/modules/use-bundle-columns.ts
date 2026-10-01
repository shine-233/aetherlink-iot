/*
 * 文件用途：应用包列表的表格列定义（从 index.vue 拆出）。
 * 核心逻辑：纯展示列 + 动作列；动作回调由页面注入，保持列定义无副作用。
 */
import { computed } from 'vue'
import { h } from 'vue'
import { NButton, NPopconfirm, NSpace, NTag, NTooltip } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import type { MobileAppBundleItem } from '@/service/api'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { formatBytes, platformTextOf, statusTextOf, statusTypeOf } from '../constants'

export interface BundleColumnActions {
  onPublish: (row: MobileAppBundleItem) => void
  onArchive: (row: MobileAppBundleItem) => void
  onDownload: (row: MobileAppBundleItem) => void
  onEdit: (row: MobileAppBundleItem) => void
  onDelete: (row: MobileAppBundleItem) => void
}

export function useBundleColumns(actions: BundleColumnActions) {
  const columns = computed<DataTableColumns<MobileAppBundleItem>>(() => [
    {
      title: $t('page.app_bundle.platform'),
      key: 'platform',
      width: 100,
      render: (row) => h(NTag, { size: 'small', type: 'info' }, { default: () => platformTextOf(row.platform) })
    },
    { title: $t('page.app_bundle.version'), key: 'version', width: 120, ellipsis: { tooltip: true } },
    { title: $t('page.app_bundle.fileName'), key: 'file_name', minWidth: 160, ellipsis: { tooltip: true } },
    { title: $t('page.app_bundle.size'), key: 'file_size', width: 90, render: (row) => formatBytes(row.file_size) },
    {
      title: $t('common.status'),
      key: 'status',
      width: 100,
      render: (row) =>
        h(NTag, { size: 'small', type: statusTypeOf(row.status) }, { default: () => statusTextOf(row.status) })
    },
    {
      title: $t('page.app_bundle.checksum'),
      key: 'checksum',
      width: 130,
      render: (row) =>
        h(
          NTooltip,
          {},
          {
            trigger: () => h('span', { class: 'font-mono text-12px' }, `${row.checksum.slice(0, 10)}…`),
            default: () => h('span', { class: 'break-all font-mono' }, `sha256:${row.checksum}`)
          }
        )
    },
    {
      title: $t('page.app_bundle.publishedAt'),
      key: 'published_at',
      width: 170,
      render: (row) => formatDateTime(row.published_at) || '-'
    },
    {
      title: $t('common.actions'),
      key: 'actions',
      width: 260,
      render: (row) =>
        h(
          NSpace,
          { size: 'small', wrap: false },
          {
            default: () => [
              h(
                NButton,
                {
                  size: 'tiny',
                  type: 'primary',
                  ghost: true,
                  disabled: row.status !== 'draft',
                  onClick: () => actions.onPublish(row)
                },
                { default: () => $t('page.app_bundle.publish') }
              ),
              h(
                NButton,
                {
                  size: 'tiny',
                  type: 'warning',
                  ghost: true,
                  disabled: row.status !== 'published',
                  onClick: () => actions.onArchive(row)
                },
                { default: () => $t('page.app_bundle.archive') }
              ),
              h(
                NButton,
                { size: 'tiny', disabled: !row.file_path, onClick: () => actions.onDownload(row) },
                { default: () => $t('page.app_bundle.download') }
              ),
              h(
                NButton,
                {
                  size: 'tiny',
                  disabled: row.status !== 'draft',
                  onClick: () => actions.onEdit(row)
                },
                { default: () => $t('common.edit') }
              ),
              h(
                NPopconfirm,
                { onPositiveClick: () => actions.onDelete(row) },
                {
                  trigger: () =>
                    h(
                      NButton,
                      { size: 'tiny', type: 'error', ghost: true, disabled: row.status === 'published' },
                      { default: () => $t('common.delete') }
                    ),
                  default: () => $t('page.app_bundle.deleteConfirm')
                }
              )
            ]
          }
        )
    }
  ])

  return { columns }
}
