import { describe, expect, it } from 'vitest'
import { buildReadyCheckSourceContext } from '../ready-check-source-context'

describe('ready-check-source-context', () => {
  it('classifies OTA failed-rollout entrypoints and preserves task/detail evidence', () => {
    expect(
      buildReadyCheckSourceContext({
        source: 'ota',
        ota_task_id: 'task-1',
        ota_detail_id: 'detail-1'
      })
    ).toMatchObject({
      isOtaFailureSource: true,
      isFirstDeviceOnboardingSource: false,
      sourceKey: 'ota_failed_rollout',
      labelKey: 'custom.device_details.readyCheckSourceOta',
      detailText: 'task=task-1 / detail=detail-1',
      otaTaskId: 'task-1',
      otaDetailId: 'detail-1'
    })
  })

  it('keeps OTA source visible even when the route lacks task ids', () => {
    expect(buildReadyCheckSourceContext({ source: 'ota' })).toMatchObject({
      isOtaFailureSource: true,
      sourceKey: 'ota_failed_rollout',
      detailKey: 'custom.device_details.readyCheckSourceOtaDetailEmpty',
      detailText: undefined
    })
  })

  it('classifies first-device onboarding separately from ordinary device details', () => {
    expect(buildReadyCheckSourceContext({ onboarding: 'first-device' })).toMatchObject({
      isFirstDeviceOnboardingSource: true,
      isOtaFailureSource: false,
      sourceKey: 'home_first_device_onboarding',
      labelKey: 'custom.device_details.readyCheckSourceFirstDevice',
      detailKey: 'custom.device_details.readyCheckSourceFirstDeviceDetail'
    })

    expect(buildReadyCheckSourceContext({})).toMatchObject({
      isFirstDeviceOnboardingSource: false,
      isOtaFailureSource: false,
      sourceKey: 'device_details',
      labelKey: 'custom.device_details.readyCheckSourceDeviceDetails',
      detailKey: 'custom.device_details.readyCheckSourceDeviceDetailsDetail'
    })
  })
})
