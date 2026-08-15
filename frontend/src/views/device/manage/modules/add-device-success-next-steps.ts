export type AddDeviceSuccessNextStep = {
  key: string
  titleKey: string
  descriptionKey: string
  actionKey: string
  routeKey: 'device_details' | 'automation_linkage-edit'
  query: Record<string, string>
  type: 'primary' | 'info' | 'success' | 'warning'
}

export type BuildAddDeviceSuccessNextStepsOptions = {
  firstDeviceOnboarding?: boolean
  deviceConfigId?: string
}

export const buildAddDeviceSuccessNextSteps = (
  deviceId: string,
  options: BuildAddDeviceSuccessNextStepsOptions = {}
): AddDeviceSuccessNextStep[] => {
  const detailsQuery: Record<string, string> = options.firstDeviceOnboarding
    ? { d_id: deviceId, onboarding: 'first-device' }
    : { d_id: deviceId }
  const accessGuideQuery = { ...detailsQuery, tab: 'join', focus: 'quickstart' }
  const readyCheckQuery = { ...detailsQuery, tab: 'ready-check' }
  const twinQuery = { ...detailsQuery, tab: 'device-twin' }
  const commandDeliveryQuery = { ...detailsQuery, tab: 'command-delivery' }
  const automationDeviceContext = {
    device_id: deviceId,
    ...(options.deviceConfigId ? { device_config_id: options.deviceConfigId } : {})
  }

  return [
    {
      key: 'open-access-guide',
      titleKey: 'custom.devicePage.nextStepAccessGuide',
      descriptionKey: 'custom.devicePage.nextStepAccessGuideDesc',
      actionKey: 'custom.devicePage.openAccessGuide',
      routeKey: 'device_details',
      query: accessGuideQuery,
      type: 'primary'
    },
    {
      key: 'verify-online',
      titleKey: 'custom.devicePage.nextStepVerifyOnline',
      descriptionKey: 'custom.devicePage.nextStepVerifyOnlineDesc',
      actionKey: 'custom.devicePage.openDeviceDetail',
      routeKey: 'device_details',
      query: readyCheckQuery,
      type: 'success'
    },
    {
      key: 'inspect-twin',
      titleKey: 'custom.devicePage.nextStepTwin',
      descriptionKey: 'custom.devicePage.nextStepTwinDesc',
      actionKey: 'custom.devicePage.openDeviceTwin',
      routeKey: 'device_details',
      query: twinQuery,
      type: 'success'
    },
    {
      key: 'check-commands',
      titleKey: 'custom.devicePage.nextStepCommand',
      descriptionKey: 'custom.devicePage.nextStepCommandDesc',
      actionKey: 'custom.devicePage.openCommandLog',
      routeKey: 'device_details',
      query: commandDeliveryQuery,
      type: 'info'
    },
    {
      key: 'create-automation',
      titleKey: 'custom.devicePage.nextStepAutomation',
      descriptionKey: 'custom.devicePage.nextStepAutomationDesc',
      actionKey: 'custom.devicePage.createAutomation',
      routeKey: 'automation_linkage-edit',
      query: {
        ...automationDeviceContext,
        backType: 'device',
        ...(options.firstDeviceOnboarding
          ? {
              onboarding: 'first-device',
              starter: 'first-telemetry-rule'
            }
          : {})
      },
      type: 'warning'
    }
  ]
}
