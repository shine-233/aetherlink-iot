/*
 * useDeviceManageRowActions: the device row-level actions of the manage page.
 *
 * Extracted from index.vue when the page moved onto `useListPage`. Owns the
 * delete-confirmation dialog and the column contract (device-table-columns.tsx), so
 * index.vue stays a pure orchestrator. `refresh` is the page's table refresh callback.
 */
import { deleteDevice as deleteDeviceApi } from '@/service/api/device'
import { $t } from '@/locales'
import { createDeviceManageColumns } from './device-table-columns'

export interface UseDeviceManageRowActionsOptions {
  goDeviceDetails: (row: any) => void
  openEditDevice: (row: any) => void
  openShareDevice: (row: any) => void
  openIssueClaimToken: (row: any) => void
  /** Reload the current table page after a successful delete. */
  refresh: () => void
}

export function useDeviceManageRowActions(options: UseDeviceManageRowActionsOptions) {
  const confirmDeleteDevice = (row: any) => {
    const id = String(row?.id || '')
    if (!id) return
    window.$dialog?.warning({
      title: $t('common.delete'),
      content: $t('common.confirmDelete'),
      positiveText: $t('common.confirm'),
      negativeText: $t('common.cancel'),
      onPositiveClick: async () => {
        const { error } = await deleteDeviceApi({ id })
        if (!error) {
          window.$message?.success($t('common.deleteSuccess'))
          options.refresh()
        }
      }
    })
  }

  const columnsToShow = createDeviceManageColumns(
    options.goDeviceDetails,
    options.openEditDevice,
    confirmDeleteDevice,
    options.openShareDevice,
    options.openIssueClaimToken
  )

  return { columnsToShow, confirmDeleteDevice }
}
