import type { FleetCommandJobPreviewRow } from '@/service/api/device'
import { getRecommendedPathLabelKey } from './commandCenterState'

type Translate = (key: any) => string

export const commandCenterImmediateChecks = [
  'custom.commandCenter.immediateCheckOnline',
  'custom.commandCenter.immediateCheckMessageId',
  'custom.commandCenter.immediateCheckResponse'
]

export const commandCenterJobRequirements = [
  'custom.commandCenter.jobRequirementScope',
  'custom.commandCenter.jobRequirementEligibility',
  'custom.commandCenter.jobRequirementProgress',
  'custom.commandCenter.jobRequirementRetry',
  'custom.commandCenter.jobRequirementAudit'
]

export const commandCenterPostSubmitChecklist = [
  'custom.commandCenter.postSubmitRefresh',
  'custom.commandCenter.postSubmitReviewFailures',
  'custom.commandCenter.postSubmitSupportBundle',
  'custom.commandCenter.postSubmitRetryOrCancel'
]

export function formatCommandCenterRecommendedPath(path: FleetCommandJobPreviewRow['recommended_path'], t: Translate) {
  return t(getRecommendedPathLabelKey(path))
}

export function formatCommandCenterTelemetryEvidence(row: FleetCommandJobPreviewRow, t: Translate) {
  const count = row.telemetry_current_count ?? 0
  if (!count) return t('custom.commandCenter.noTelemetryEvidence')

  return t('custom.commandCenter.telemetryEvidence')
    .replace('{count}', String(count))
    .replace('{latest}', row.latest_telemetry_key || '-')
}

export function buildCommandCenterContractRows(
  input: {
    currentPageCount: number | null
    filterSummaryCount: number
    requestedTotal: number | null
    routeScope: string
    scope: string
    selectedCount: number
  },
  t: Translate
) {
  return [
    {
      label: t('custom.commandCenter.scope'),
      value: input.scope
    },
    {
      label: t('custom.commandCenter.routeScope'),
      value: input.routeScope
    },
    {
      label: t('custom.commandCenter.selectedDevices'),
      value: String(input.selectedCount)
    },
    {
      label: t('custom.commandCenter.currentPageDevices'),
      value: input.currentPageCount === null ? '--' : String(input.currentPageCount)
    },
    {
      label: t('custom.commandCenter.matchingDevices'),
      value: input.requestedTotal === null ? '--' : String(input.requestedTotal)
    },
    {
      label: t('custom.commandCenter.filterFields'),
      value: String(input.filterSummaryCount)
    }
  ]
}

export function buildCommandJobLink(currentHref: string, jobId: string) {
  const url = new URL(currentHref)
  url.searchParams.set('command_job_id', jobId)
  return url.toString()
}
