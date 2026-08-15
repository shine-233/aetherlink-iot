import {
  buildOtaTaskPreflightSummary,
  buildOtaTaskRiskDevices,
  type OtaDeviceCandidate,
  type OtaTaskPreflightSummary,
  type OtaTaskRiskDevice
} from './ota-task-state'
import type { OtaPackageRecord, RolloutSummaryTagType } from './ota-task-types'

type Translate = (key: string) => string

export type OtaTaskPreflightItem = {
  key: string
  label: string
  value: number
  type: RolloutSummaryTagType
}

export type OtaTaskPreflightView = {
  summary: OtaTaskPreflightSummary
  items: OtaTaskPreflightItem[]
  riskDevices: OtaTaskRiskDevice[]
}

export function getOtaPackageTargetVersion(selectedPackage?: OtaPackageRecord | null) {
  return selectedPackage?.target_version || selectedPackage?.version
}

export function buildOtaTaskPreflightItems(summary: OtaTaskPreflightSummary, t: Translate): OtaTaskPreflightItem[] {
  return [
    {
      key: 'eligible',
      label: t('page.product.update-ota.preflightEligible'),
      value: summary.eligible,
      type: 'info'
    },
    {
      key: 'selected',
      label: t('page.product.update-ota.preflightSelected'),
      value: summary.selected,
      type: 'success'
    },
    {
      key: 'offline',
      label: t('page.product.update-ota.preflightOffline'),
      value: summary.offline,
      type: summary.offline ? 'warning' : 'default'
    },
    {
      key: 'same-version',
      label: t('page.product.update-ota.preflightSameVersion'),
      value: summary.sameVersion,
      type: summary.sameVersion ? 'warning' : 'default'
    },
    {
      key: 'missing-version',
      label: t('page.product.update-ota.preflightMissingVersion'),
      value: summary.missingVersion,
      type: summary.missingVersion ? 'warning' : 'default'
    }
  ]
}

export function buildOtaTaskPreflightView(
  rows: OtaDeviceCandidate[],
  selectedIds: string[],
  selectedPackage: OtaPackageRecord | null,
  t: Translate
): OtaTaskPreflightView {
  const targetVersion = getOtaPackageTargetVersion(selectedPackage)
  const summary = buildOtaTaskPreflightSummary(rows, selectedIds, targetVersion)

  return {
    summary,
    items: buildOtaTaskPreflightItems(summary, t),
    riskDevices: buildOtaTaskRiskDevices(rows, selectedIds, targetVersion)
  }
}
