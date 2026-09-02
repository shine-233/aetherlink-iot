import type { FleetCommandJobPreviewResult } from '@/service/api/device'
import { DEFAULT_FILTER_JOB_MAX_DEVICES, DEFAULT_FILTER_JOB_SUBSET_LIMIT } from './commandCenterState'

type Translate = (key: string) => string

export function commandPreviewCoversFullFilterScope(input: {
  isDeviceFilterScope: boolean
  previewResult?: FleetCommandJobPreviewResult | null
}) {
  if (!input.isDeviceFilterScope || !input.previewResult) return true
  return input.previewResult.rows.length === input.previewResult.requested_count
}

export function buildCommandSubmitDisabledHint(
  input: {
    hasCommandJobScope: boolean
    commandIdentify: string
    previewResult?: FleetCommandJobPreviewResult | null
    previewPayloadFingerprint: string
    currentPayloadFingerprint: string
    previewCoversFullFilterScope: boolean
    maxDevices?: number | null
  },
  t: Translate
) {
  const preview = input.previewResult
  if (!input.hasCommandJobScope) return t('custom.commandCenter.noSelection')
  if (!input.commandIdentify.trim()) return t('custom.commandCenter.commandIdentifierRequired')
  if (!preview) return t('custom.commandCenter.submitBlockedPreviewMissing')
  if (input.previewPayloadFingerprint !== input.currentPayloadFingerprint)
    return t('custom.commandCenter.submitBlockedChanged')
  if (preview.eligible_count <= 0) return t('custom.commandCenter.submitBlockedNoEligible')
  if (!input.previewCoversFullFilterScope) {
    return t('custom.commandCenter.submitBlockedSubsetOnly')
      .replace('{shown}', String(preview.rows.length))
      .replace('{matched}', String(preview.requested_count))
      .replace('{max}', String(input.maxDevices || DEFAULT_FILTER_JOB_MAX_DEVICES))
  }
  return ''
}

export function buildCommandJobReadiness(
  input: {
    hasCommandJobScope: boolean
    commandIdentify: string
    previewResult?: FleetCommandJobPreviewResult | null
    previewPayloadFingerprint: string
    currentPayloadFingerprint: string
    previewCoversFullFilterScope: boolean
    maxDevices?: number | null
  },
  t: Translate
) {
  const blockingReason = buildCommandSubmitDisabledHint(input, t)
  const hasPreview = Boolean(input.previewResult)
  const canPreview = input.hasCommandJobScope && Boolean(input.commandIdentify.trim())
  const canSubmit = hasPreview && !blockingReason
  const previewCoverageStatus = !hasPreview ? 'missing' : input.previewCoversFullFilterScope ? 'full' : 'subset_only'
  const requiredNextAction = !canPreview
    ? blockingReason || t('custom.commandCenter.commandIdentifierRequired')
    : canSubmit
      ? t('custom.commandCenter.submitEligibleDevices')
      : blockingReason || t('custom.commandCenter.previewBeforeSubmit')

  return {
    canPreview,
    canSubmit,
    blockingReason,
    previewCoverageStatus,
    requiredNextAction,
    customerRiskLevel: canSubmit ? 'ready' : previewCoverageStatus === 'subset_only' ? 'warning' : 'blocked'
  }
}

export function buildFilterExecutionCapSummary(
  input: {
    requestedTotal: number | null
    maxDevices?: number | null
    subsetLimit?: number | null
  },
  t: Translate
) {
  return t('custom.commandCenter.maxDevicesHint')
    .replace('{matched}', input.requestedTotal === null ? '--' : String(input.requestedTotal))
    .replace('{max}', String(input.maxDevices || DEFAULT_FILTER_JOB_MAX_DEVICES))
    .replace('{subset}', String(input.subsetLimit || DEFAULT_FILTER_JOB_SUBSET_LIMIT))
}
