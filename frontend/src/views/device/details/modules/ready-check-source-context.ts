import { normalizeRouteQueryText } from './ready-check-deep-links'

export type ReadyCheckSourceContext = {
  isFirstDeviceOnboardingSource: boolean
  isOtaFailureSource: boolean
  isCommandJobDiagnosisSource: boolean
  otaTaskId: string
  otaDetailId: string
  commandJobId: string
  sourceKey: 'ota_failed_rollout' | 'home_first_device_onboarding' | 'command_job_diagnosis' | 'device_details'
  labelKey: string
  detailKey: string
  detailText?: string
}

export const buildReadyCheckSourceContext = (routeQuery: Record<string, unknown>): ReadyCheckSourceContext => {
  const isFirstDeviceOnboardingSource = normalizeRouteQueryText(routeQuery.onboarding) === 'first-device'
  const isOtaFailureSource = normalizeRouteQueryText(routeQuery.source) === 'ota'
  const isCommandJobDiagnosisSource = normalizeRouteQueryText(routeQuery.source) === 'command_job_diagnosis'
  const otaTaskId = normalizeRouteQueryText(routeQuery.ota_task_id)
  const otaDetailId = normalizeRouteQueryText(routeQuery.ota_detail_id)
  const commandJobId = normalizeRouteQueryText(routeQuery.command_job_id)

  if (isOtaFailureSource) {
    const ids = [
      otaTaskId ? `task=${otaTaskId}` : '',
      otaDetailId ? `detail=${otaDetailId}` : ''
    ].filter(Boolean)

    return {
      isFirstDeviceOnboardingSource,
      isOtaFailureSource,
      isCommandJobDiagnosisSource,
      otaTaskId,
      otaDetailId,
      commandJobId,
      sourceKey: 'ota_failed_rollout',
      labelKey: 'custom.device_details.readyCheckSourceOta',
      detailKey: 'custom.device_details.readyCheckSourceOtaDetailEmpty',
      detailText: ids.length ? ids.join(' / ') : undefined
    }
  }

  if (isFirstDeviceOnboardingSource) {
    return {
      isFirstDeviceOnboardingSource,
      isOtaFailureSource,
      isCommandJobDiagnosisSource,
      otaTaskId,
      otaDetailId,
      commandJobId,
      sourceKey: 'home_first_device_onboarding',
      labelKey: 'custom.device_details.readyCheckSourceFirstDevice',
      detailKey: 'custom.device_details.readyCheckSourceFirstDeviceDetail'
    }
  }

  if (isCommandJobDiagnosisSource) {
    return {
      isFirstDeviceOnboardingSource,
      isOtaFailureSource,
      isCommandJobDiagnosisSource,
      otaTaskId,
      otaDetailId,
      commandJobId,
      sourceKey: 'command_job_diagnosis',
      labelKey: 'custom.device_details.readyCheckSourceCommandJob',
      detailKey: 'custom.device_details.readyCheckSourceCommandJobDetailEmpty',
      detailText: commandJobId ? `job=${commandJobId}` : undefined
    }
  }

  return {
    isFirstDeviceOnboardingSource,
    isOtaFailureSource,
    isCommandJobDiagnosisSource,
    otaTaskId,
    otaDetailId,
    commandJobId,
    sourceKey: 'device_details',
    labelKey: 'custom.device_details.readyCheckSourceDeviceDetails',
    detailKey: 'custom.device_details.readyCheckSourceDeviceDetailsDetail'
  }
}
