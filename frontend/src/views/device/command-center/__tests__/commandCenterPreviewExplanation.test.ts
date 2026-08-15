import { buildCommandCenterPreviewExplanationRows } from '../commandCenterPreviewExplanation'

const translations: Record<string, string> = {
  'custom.commandCenter.previewExplainTargetMode': 'Target mode',
  'custom.commandCenter.previewExplainFilteredFleet': 'Filtered fleet',
  'custom.commandCenter.previewExplainSelectedDevices': 'Selected devices',
  'custom.commandCenter.previewExplainSavedFilter': 'Saved filter',
  'custom.commandCenter.previewExplainNoSavedFilter': 'No saved filter',
  'custom.commandCenter.previewExplainSelectedCount': 'Selected',
  'custom.commandCenter.previewExplainCurrentPage': 'Current page',
  'custom.commandCenter.previewExplainBackendMatched': 'Backend matched',
  'custom.commandCenter.previewExplainShownRows': 'Shown rows',
  'custom.commandCenter.previewExplainSafetyCap': 'Safety cap',
  'custom.commandCenter.previewExplainSubsetLimit': 'Subset limit',
  'custom.commandCenter.previewExplainSubmitGate': 'Submit gate',
  'custom.commandCenter.previewExplainSubmitUnlocked': 'Unlocked',
  'custom.commandCenter.previewExplainSubmitLocked': 'Locked'
}

const t = (key: string) => translations[key] ?? key

describe('commandCenterPreviewExplanation', () => {
  it('explains selected-device previews without filter-only values', () => {
    const rows = buildCommandCenterPreviewExplanationRows(
      {
        scopeType: 'selected_devices',
        selectedCount: 3,
        currentPageCount: null,
        requestedTotal: null,
        previewRequestedCount: 3,
        previewShownCount: 3,
        canSubmitCommandJob: true
      },
      t
    )

    expect(rows).toContainEqual({ label: 'Target mode', value: 'Selected devices' })
    expect(rows).toContainEqual({ label: 'Selected', value: '3' })
    expect(rows).toContainEqual({ label: 'Safety cap', value: '--' })
    expect(rows).toContainEqual({ label: 'Subset limit', value: '--' })
    expect(rows).toContainEqual({ label: 'Submit gate', value: 'Unlocked' })
  })

  it('explains saved-filter previews with backend matched, shown rows, cap, and subset limit', () => {
    const rows = buildCommandCenterPreviewExplanationRows(
      {
        scopeType: 'device_filter',
        selectedCount: 0,
        currentPageCount: 10,
        requestedTotal: 42,
        previewRequestedCount: 38,
        previewShownCount: 20,
        activeSavedFilterName: 'Fleet 2026-07-05',
        maxDevices: 100,
        subsetLimit: 20,
        canSubmitCommandJob: false,
        submitDisabledHint: 'Preview is subset-only'
      },
      t
    )

    expect(rows).toContainEqual({ label: 'Target mode', value: 'Filtered fleet' })
    expect(rows).toContainEqual({ label: 'Saved filter', value: 'Fleet 2026-07-05' })
    expect(rows).toContainEqual({ label: 'Current page', value: '10' })
    expect(rows).toContainEqual({ label: 'Backend matched', value: '38' })
    expect(rows).toContainEqual({ label: 'Shown rows', value: '20' })
    expect(rows).toContainEqual({ label: 'Safety cap', value: '100' })
    expect(rows).toContainEqual({ label: 'Subset limit', value: '20' })
    expect(rows).toContainEqual({ label: 'Submit gate', value: 'Preview is subset-only' })
  })
})
