import { describe, expect, it } from 'vitest'
import { buildAddDeviceSuccessNextSteps } from '../add-device-success-next-steps'

describe('buildAddDeviceSuccessNextSteps', () => {
  it('keeps ordinary add-device detail links free of first-device onboarding context', () => {
    const steps = buildAddDeviceSuccessNextSteps('device-1')

    expect(steps.find((step) => step.key === 'open-access-guide')?.query).toEqual({
      d_id: 'device-1',
      tab: 'join',
      focus: 'quickstart'
    })
    expect(steps.find((step) => step.key === 'verify-online')?.query).toEqual({
      d_id: 'device-1',
      tab: 'ready-check'
    })
  })

  it('preserves first-device onboarding context for detail next steps', () => {
    const steps = buildAddDeviceSuccessNextSteps('device-1', {
      firstDeviceOnboarding: true
    })

    expect(steps.find((step) => step.key === 'open-access-guide')?.query).toEqual({
      d_id: 'device-1',
      onboarding: 'first-device',
      tab: 'join',
      focus: 'quickstart'
    })
    expect(steps.find((step) => step.key === 'verify-online')?.query).toEqual({
      d_id: 'device-1',
      onboarding: 'first-device',
      tab: 'ready-check'
    })
  })

  it('routes first-device automation into the telemetry-rule starter', () => {
    const steps = buildAddDeviceSuccessNextSteps('device-1', {
      firstDeviceOnboarding: true,
      deviceConfigId: 'config-1'
    })

    expect(steps.find((step) => step.key === 'create-automation')?.query).toEqual({
      device_id: 'device-1',
      device_config_id: 'config-1',
      backType: 'device',
      onboarding: 'first-device',
      starter: 'first-telemetry-rule'
    })
  })
})
