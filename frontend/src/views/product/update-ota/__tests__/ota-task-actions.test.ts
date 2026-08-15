import { describe, expect, it } from 'vitest'
import {
  canCancelOtaTaskDetail,
  canRetryOtaTaskDetail,
  getOtaTaskDetailActionDeviceLabel,
  getOtaTaskDetailActionTitleKey,
  isOtaTaskDetailProgressActive,
  OTA_TASK_DETAIL_ACTION,
  OTA_TASK_DETAIL_STATUS
} from '../ota-task-actions'

describe('ota-task-actions', () => {
  it('keeps retry and cancel action payload codes explicit', () => {
    expect(OTA_TASK_DETAIL_ACTION.retry).toBe(1)
    expect(OTA_TASK_DETAIL_ACTION.cancel).toBe(6)
    expect(getOtaTaskDetailActionTitleKey(OTA_TASK_DETAIL_ACTION.retry)).toBe('page.product.update-ota.retryTask')
    expect(getOtaTaskDetailActionTitleKey(OTA_TASK_DETAIL_ACTION.cancel)).toBe('page.product.update-ota.cancelMakeTask')
  })

  it('limits device-level OTA actions to the statuses users can act on', () => {
    expect(canCancelOtaTaskDetail({ status: OTA_TASK_DETAIL_STATUS.pushed })).toBe(true)
    expect(canCancelOtaTaskDetail({ status: OTA_TASK_DETAIL_STATUS.upgrading })).toBe(true)
    expect(canCancelOtaTaskDetail({ status: OTA_TASK_DETAIL_STATUS.success })).toBe(false)
    expect(canCancelOtaTaskDetail({ status: OTA_TASK_DETAIL_STATUS.failed })).toBe(false)
    expect(canCancelOtaTaskDetail({ status: OTA_TASK_DETAIL_STATUS.canceled })).toBe(false)

    expect(canRetryOtaTaskDetail({ status: OTA_TASK_DETAIL_STATUS.failed })).toBe(true)
    expect(canRetryOtaTaskDetail({ status: OTA_TASK_DETAIL_STATUS.canceled })).toBe(false)
    expect(canRetryOtaTaskDetail({ status: OTA_TASK_DETAIL_STATUS.upgrading })).toBe(false)
  })

  it('marks only pushed and upgrading devices as active progress', () => {
    expect(isOtaTaskDetailProgressActive({ status: OTA_TASK_DETAIL_STATUS.pushed })).toBe(true)
    expect(isOtaTaskDetailProgressActive({ status: OTA_TASK_DETAIL_STATUS.upgrading })).toBe(true)
    expect(isOtaTaskDetailProgressActive({ status: OTA_TASK_DETAIL_STATUS.failed })).toBe(false)
  })

  it('uses the most customer-readable device label in confirmation dialogs', () => {
    expect(getOtaTaskDetailActionDeviceLabel({ id: 'detail-1', name: 'Pump A' })).toBe('Pump A')
    expect(getOtaTaskDetailActionDeviceLabel({ id: 'detail-1', device_number: 'SN-001' })).toBe('SN-001')
    expect(getOtaTaskDetailActionDeviceLabel({ id: 'detail-1' })).toBe('detail-1')
  })
})
