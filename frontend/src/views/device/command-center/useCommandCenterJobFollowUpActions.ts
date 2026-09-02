import type { Router } from 'vue-router'
import { writeClipboardText } from '@/utils/clipboard'
import type { FleetCommandJobSupportBundle, FleetCommandJobSubmitResult } from '@/service/api/device'
import { buildCommandJobEligibilityImpactSummaryText } from './commandCenterState'
import type { CommandJobEligibilityImpactPreview } from './commandCenterState'
import { buildCommandJobLink } from './commandCenterPageView'
import { buildCommandJobCloseoutPacket } from './commandCenterJobView'

type ReadonlyRef<T> = {
  readonly value: T
}

interface UseCommandCenterJobFollowUpActionsOptions {
  router: Router
  t: (key: string) => string
  submitResult: ReadonlyRef<FleetCommandJobSubmitResult | null>
  supportBundle: ReadonlyRef<FleetCommandJobSupportBundle | null>
  loadCommandJobSupportBundle: () => Promise<void>
  jobHandoffSummary: ReadonlyRef<string>
  commandJobEligibilityImpactPreview: ReadonlyRef<CommandJobEligibilityImpactPreview | null | undefined>
}

export function useCommandCenterJobFollowUpActions(options: UseCommandCenterJobFollowUpActionsOptions) {
  const copyCommandJobLink = async () => {
    if (!options.submitResult.value?.job_id) return
    const ok = await writeClipboardText(buildCommandJobLink(window.location.href, options.submitResult.value.job_id))
    if (ok) {
      window.$message?.success(options.t('custom.commandCenter.copyJobLinkSuccess'))
    } else {
      window.$message?.warning(options.t('common.copyFailed'))
    }
  }

  const copyCommandJobHandoffSummary = async () => {
    if (!options.submitResult.value?.job_id) return
    const summary = `${options.jobHandoffSummary.value}\n${buildCommandJobLink(
      window.location.href,
      options.submitResult.value.job_id
    )}`
    const ok = await writeClipboardText(summary)
    if (ok) {
      window.$message?.success(options.t('custom.commandCenter.copyHandoffSummarySuccess'))
    } else {
      window.$message?.warning(options.t('common.copyFailed'))
    }
  }

  const copyCommandJobCloseoutPacket = async () => {
    if (!options.submitResult.value?.job_id) return
    if (!options.supportBundle.value) {
      await options.loadCommandJobSupportBundle()
    }
    const packet = buildCommandJobCloseoutPacket(
      options.submitResult.value,
      buildCommandJobLink(window.location.href, options.submitResult.value.job_id),
      options.supportBundle.value
    )
    const ok = await writeClipboardText(packet)
    if (ok) {
      window.$message?.success(options.t('custom.commandCenter.copyCloseoutPacketSuccess'))
    } else {
      window.$message?.warning(options.t('common.copyFailed'))
    }
  }

  const copyCommandJobEligibilityImpactSummary = async () => {
    const summary = buildCommandJobEligibilityImpactSummaryText(
      options.commandJobEligibilityImpactPreview.value,
      options.t
    )
    if (!summary) return
    const ok = await writeClipboardText(summary)
    if (ok) {
      window.$message?.success(options.t('custom.commandCenter.copyImpactPreviewSuccess'))
    } else {
      window.$message?.warning(options.t('common.copyFailed'))
    }
  }

  const openCommandJobDeviceDiagnosis = (deviceId: string) => {
    if (!deviceId) {
      window.$message?.warning(options.t('custom.commandCenter.openDeviceDiagnosisMissing'))
      return
    }
    options.router.push({
      path: '/device/details',
      query: {
        d_id: deviceId,
        tab: 'ready-check',
        command_job_id: options.submitResult.value?.job_id || undefined,
        source: 'command_job_diagnosis'
      }
    })
  }

  return {
    copyCommandJobLink,
    copyCommandJobHandoffSummary,
    copyCommandJobCloseoutPacket,
    copyCommandJobEligibilityImpactSummary,
    openCommandJobDeviceDiagnosis
  }
}
