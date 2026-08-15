import type { OtaTaskDetailRecord } from './ota-task-types'

export type OtaTaskDetailAction = 1 | 6

export const OTA_TASK_DETAIL_ACTION = {
  retry: 1,
  cancel: 6
} as const

export const OTA_TASK_DETAIL_STATUS = {
  pushed: 2,
  upgrading: 3,
  success: 4,
  failed: 5,
  canceled: 6
} as const

export function canCancelOtaTaskDetail(row: Pick<OtaTaskDetailRecord, 'status'>) {
  return (
    row.status !== OTA_TASK_DETAIL_STATUS.success &&
    row.status !== OTA_TASK_DETAIL_STATUS.failed &&
    row.status !== OTA_TASK_DETAIL_STATUS.canceled
  )
}

export function canRetryOtaTaskDetail(row: Pick<OtaTaskDetailRecord, 'status'>) {
  return row.status === OTA_TASK_DETAIL_STATUS.failed
}

export function isOtaTaskDetailProgressActive(row: Pick<OtaTaskDetailRecord, 'status'>) {
  return row.status === OTA_TASK_DETAIL_STATUS.pushed || row.status === OTA_TASK_DETAIL_STATUS.upgrading
}

export function getOtaTaskDetailActionTitleKey(action: OtaTaskDetailAction) {
  return action === OTA_TASK_DETAIL_ACTION.retry
    ? 'page.product.update-ota.retryTask'
    : 'page.product.update-ota.cancelMakeTask'
}

export function getOtaTaskDetailActionDeviceLabel(row: OtaTaskDetailRecord) {
  return row.name || row.device_number || row.id
}
