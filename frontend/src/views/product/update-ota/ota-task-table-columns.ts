import { h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NProgress, NTag } from 'naive-ui'
import { $t } from '@/locales'
import {
  canCancelOtaTaskDetail,
  canRetryOtaTaskDetail,
  isOtaTaskDetailProgressActive,
  type OtaTaskDetailAction,
  OTA_TASK_DETAIL_ACTION,
  OTA_TASK_DETAIL_STATUS
} from './ota-task-actions'
import type { OtaPackageRecord, OtaTaskDetailRecord, OtaTaskRecord, RolloutSummaryTagType } from './ota-task-types'

type CreateOtaTaskColumnsOptions = {
  getSelectedPackage: () => OtaPackageRecord | null
  formatTime: (value?: string) => string
  openTaskDetail: (row: OtaTaskRecord) => void
}

type CreateOtaTaskDetailColumnsOptions = {
  getSelectedPackage: () => OtaPackageRecord | null
  formatTime: (value?: string) => string
  statusLabel: (status?: number) => string
  statusTagType: (status?: number) => RolloutSummaryTagType
  openFailedDeviceDiagnostics?: (row: OtaTaskDetailRecord) => void
  updateTaskDetailStatus: (row: OtaTaskDetailRecord, action: OtaTaskDetailAction) => void
}

function otaProgressPercentage(row: OtaTaskDetailRecord) {
  const value = Number(row.steps || 0)
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, value))
}

function otaTaskCount(value: unknown, fallback = 0) {
  const count = Number(value ?? fallback)
  return Number.isFinite(count) ? count : fallback
}

function renderOtaTargetScope(row: OtaTaskRecord) {
  const isFilter = row.target_mode === 'filter'
  const selected = otaTaskCount(row.selected_count, otaTaskCount(row.device_count))
  const preview = otaTaskCount(row.preview_total, selected)
  const countLabel = $t('page.product.update-ota.targetScopeCount')
    .replace('{selected}', String(selected))
    .replace('{preview}', String(preview))

  return h('div', { style: 'display:flex;align-items:center;gap:8px;flex-wrap:wrap;' }, [
    h(
      NTag,
      { size: 'small', type: isFilter ? 'info' : 'default' },
      {
        default: () =>
          isFilter
            ? $t('page.product.update-ota.targetScopeFilter')
            : $t('page.product.update-ota.targetScopeExplicit')
      }
    ),
    h('span', countLabel)
  ])
}

export function createOtaTaskColumns(options: CreateOtaTaskColumnsOptions): DataTableColumns<OtaTaskRecord> {
  return [
    {
      key: 'name',
      title: () => $t('page.product.update-ota.taskName'),
      minWidth: 200,
      ellipsis: { tooltip: true },
      render: (row) => row.name || '-'
    },
    {
      key: 'package',
      title: () => $t('page.product.update-ota.packageInfo'),
      minWidth: 180,
      render: () => options.getSelectedPackage()?.name || options.getSelectedPackage()?.version || '-'
    },
    {
      key: 'device_count',
      title: () => $t('page.product.update-ota.deviceNum'),
      width: 120,
      render: (row) => Number(row.device_count || 0)
    },
    {
      key: 'target_scope',
      title: () => $t('page.product.update-ota.targetScope'),
      minWidth: 180,
      render: renderOtaTargetScope
    },
    {
      key: 'description',
      title: () => $t('page.product.update-ota.desc'),
      minWidth: 220,
      ellipsis: { tooltip: true },
      render: (row) => row.description || '-'
    },
    {
      key: 'created_at',
      title: () => $t('page.product.update-ota.createTime'),
      width: 180,
      render: (row) => options.formatTime(row.created_at)
    },
    {
      key: 'actions',
      title: () => $t('common.actions'),
      width: 130,
      fixed: 'right',
      render: (row) =>
        h(
          NButton,
          { size: 'small', type: 'primary', onClick: () => options.openTaskDetail(row) },
          { default: () => $t('page.product.update-ota.lookTask') }
        )
    }
  ]
}

export function createOtaTaskDetailColumns(
  options: CreateOtaTaskDetailColumnsOptions
): DataTableColumns<OtaTaskDetailRecord> {
  return [
    {
      key: 'name',
      title: () => $t('page.product.update-ota.deviceName'),
      minWidth: 160,
      render: (row) => row.name || row.device_number || '-'
    },
    {
      key: 'current_version',
      title: () => $t('page.product.update-ota.currentVersion'),
      width: 150,
      render: (row) => row.current_version || '-'
    },
    {
      key: 'version',
      title: () => $t('page.product.update-ota.targetVersion'),
      width: 150,
      render: (row) => row.version || options.getSelectedPackage()?.version || '-'
    },
    {
      key: 'steps',
      title: () => $t('page.product.update-ota.progress'),
      width: 180,
      render: (row) =>
        h(NProgress, {
          type: 'line',
          percentage: otaProgressPercentage(row),
          indicatorPlacement: 'inside',
          processing: isOtaTaskDetailProgressActive(row),
          status:
            row.status === OTA_TASK_DETAIL_STATUS.failed
              ? 'error'
              : row.status === OTA_TASK_DETAIL_STATUS.success
                ? 'success'
                : 'default'
        })
    },
    {
      key: 'status',
      title: () => $t('page.product.update-ota.statusTask'),
      width: 130,
      render: (row) =>
        h(NTag, { type: options.statusTagType(row.status) }, { default: () => options.statusLabel(row.status) })
    },
    {
      key: 'status_description',
      title: () => $t('page.product.update-ota.statusDetail'),
      minWidth: 180,
      ellipsis: { tooltip: true },
      render: (row) => row.status_description || '-'
    },
    {
      key: 'updated_at',
      title: () => $t('page.product.update-ota.updateTime'),
      width: 180,
      render: (row) => options.formatTime(row.updated_at)
    },
    {
      key: 'actions',
      title: () => $t('common.actions'),
      width: 280,
      fixed: 'right',
      render: (row) =>
        h('div', { class: 'action-row' }, [
          h(
            NButton,
            {
              size: 'small',
              disabled: !canCancelOtaTaskDetail(row),
              onClick: () => options.updateTaskDetailStatus(row, OTA_TASK_DETAIL_ACTION.cancel)
            },
            { default: () => $t('page.product.update-ota.cancelMakeTask') }
          ),
          h(
            NButton,
            {
              size: 'small',
              type: 'warning',
              disabled: !canRetryOtaTaskDetail(row),
              onClick: () => options.updateTaskDetailStatus(row, OTA_TASK_DETAIL_ACTION.retry)
            },
            { default: () => $t('page.product.update-ota.retryTask') }
          ),
          row.status === OTA_TASK_DETAIL_STATUS.failed
            ? h(
                NButton,
                {
                  size: 'small',
                  secondary: true,
                  type: 'info',
                  disabled: !row.device_id || !options.openFailedDeviceDiagnostics,
                  onClick: () => options.openFailedDeviceDiagnostics?.(row)
                },
                { default: () => $t('page.product.update-ota.openFailureDiagnostics') }
              )
            : null
        ])
    }
  ]
}
