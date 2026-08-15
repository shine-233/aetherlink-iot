import {
  DEFAULT_FILTER_JOB_MAX_DEVICES,
  DEFAULT_FILTER_JOB_SUBSET_LIMIT,
  type FleetCommandScopeType
} from './commandCenterState'

type Translate = (key: string) => string

export interface CommandCenterPreviewExplanationInput {
  scopeType: FleetCommandScopeType
  selectedCount: number
  currentPageCount: number | null
  requestedTotal: number | null
  previewRequestedCount?: number | null
  previewShownCount?: number | null
  activeSavedFilterName?: string
  maxDevices?: number | null
  subsetLimit?: number | null
  canSubmitCommandJob: boolean
  submitDisabledHint?: string
}

export interface CommandCenterPreviewExplanationRow {
  label: string
  value: string
}

const formatNullableNumber = (value?: number | null) => (typeof value === 'number' ? String(value) : '--')

export const buildCommandCenterPreviewExplanationRows = (
  input: CommandCenterPreviewExplanationInput,
  t: Translate
): CommandCenterPreviewExplanationRow[] => {
  const isDeviceFilterScope = input.scopeType === 'device_filter'
  const backendMatched = input.previewRequestedCount ?? input.requestedTotal

  return [
    {
      label: t('custom.commandCenter.previewExplainTargetMode'),
      value: isDeviceFilterScope
        ? t('custom.commandCenter.previewExplainFilteredFleet')
        : t('custom.commandCenter.previewExplainSelectedDevices')
    },
    {
      label: t('custom.commandCenter.previewExplainSavedFilter'),
      value: input.activeSavedFilterName || t('custom.commandCenter.previewExplainNoSavedFilter')
    },
    {
      label: t('custom.commandCenter.previewExplainSelectedCount'),
      value: String(input.selectedCount)
    },
    {
      label: t('custom.commandCenter.previewExplainCurrentPage'),
      value: formatNullableNumber(input.currentPageCount)
    },
    {
      label: t('custom.commandCenter.previewExplainBackendMatched'),
      value: formatNullableNumber(backendMatched)
    },
    {
      label: t('custom.commandCenter.previewExplainShownRows'),
      value: formatNullableNumber(input.previewShownCount)
    },
    {
      label: t('custom.commandCenter.previewExplainSafetyCap'),
      value: isDeviceFilterScope ? String(input.maxDevices || DEFAULT_FILTER_JOB_MAX_DEVICES) : '--'
    },
    {
      label: t('custom.commandCenter.previewExplainSubsetLimit'),
      value: isDeviceFilterScope ? String(input.subsetLimit || DEFAULT_FILTER_JOB_SUBSET_LIMIT) : '--'
    },
    {
      label: t('custom.commandCenter.previewExplainSubmitGate'),
      value: input.canSubmitCommandJob
        ? t('custom.commandCenter.previewExplainSubmitUnlocked')
        : input.submitDisabledHint || t('custom.commandCenter.previewExplainSubmitLocked')
    }
  ]
}
