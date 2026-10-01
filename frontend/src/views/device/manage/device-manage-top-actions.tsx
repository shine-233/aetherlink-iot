/*
 * device-manage-top-actions: JSX factory for the fleet-target toolbar top action.
 *
 * Extracted from index.vue when the page moved onto `useListPage`. The other top actions
 * (shared-with-me, claim device, add-device dropdown with `trigger="click"`) stay inline in
 * index.vue — they are part of the page's asserted source contract and only need $t/router.
 */
import DeviceFleetTargetToolbar from './DeviceFleetTargetToolbar.vue'
import type { SavedFleetFilterOption } from './device-fleet-saved-filters'
import type { FleetTargetPreset, FleetTargetPresetKey } from './device-fleet-target-presets'
import type { FleetSelectionScope, FleetSelectionScopeMessage } from './device-fleet-select-all'
import { $t } from '@/locales'

/**
 * Refs/handlers the toolbar needs; refs are unwrapped inside `element()` at render time.
 *
 * The types here mirror DeviceFleetTargetToolbar's props/emits exactly — they used to be
 * `unknown` placeholders from the mid-flight split, which silently accepted any shape and
 * broke the JSX prop check.
 */
export interface DeviceFleetToolbarActionInput {
  fleetTargetPresets: FleetTargetPreset[]
  activeFleetTargetPreset: { value: FleetTargetPresetKey }
  targetPreviewTotal: { value: number | null }
  currentPageDeviceCount: { value: number }
  savedFleetFilterOptions: { value: SavedFleetFilterOption[] }
  savedFleetFilters: { value: unknown[] }
  canSaveCurrentFleetFilter: { value: boolean }
  selectedFleetDeviceIds: { value: unknown[] }
  fleetSelectionScope: { value: FleetSelectionScope }
  fleetSelectionScopeMessage: { value: FleetSelectionScopeMessage }
  canSelectAllMatchingDevices: { value: boolean }
  onSelectAllMatching: () => unknown
  onClearSelectAllMatching: () => unknown
  onOpenSelectAllCommandContext: () => unknown
  onApplyPreset: (presetKey: FleetTargetPresetKey) => unknown
  onSaveFilter: () => unknown
  onRefreshSavedFilters: () => unknown
  onApplySavedFilter: (filterID: string | number) => unknown
  onOpenSavedFilterCommandContext: (filterID: string | number) => unknown
  onDeleteSavedFilter: (filterID: string | number) => unknown
  onRenameSavedFilter: (filterID: string | number, name: string) => unknown
  onShareSavedFilter: (filterID: string | number, shared: boolean) => unknown
  onExportCurrentPage: () => unknown
  onAddSelectedToGroup: () => unknown
  onShowSelectedSummary: () => unknown
  onOpenOtaContext: () => unknown
  onOpenAlarmContext: () => unknown
  onOpenCommandContext: () => unknown
  onOpenConfigContext: () => unknown
  onOpenAuditContext: () => unknown
}

/** Build the `{ element }` top action that renders DeviceFleetTargetToolbar. */
export function createDeviceFleetToolbarAction(source: DeviceFleetToolbarActionInput) {
  return {
    element: () => (
      <DeviceFleetTargetToolbar
        presets={source.fleetTargetPresets}
        activePreset={source.activeFleetTargetPreset.value}
        targetPreviewTotal={source.targetPreviewTotal.value}
        currentPageDeviceCount={source.currentPageDeviceCount.value}
        savedFilterOptions={source.savedFleetFilterOptions.value}
        savedFilterCount={source.savedFleetFilters.value.length}
        canSaveCurrentFleetFilter={source.canSaveCurrentFleetFilter.value}
        selectedDeviceCount={source.selectedFleetDeviceIds.value.length}
        selectionScope={source.fleetSelectionScope.value}
        selectionScopeMessage={source.fleetSelectionScopeMessage.value}
        canSelectAllMatching={source.canSelectAllMatchingDevices.value}
        onSelectAllMatching={source.onSelectAllMatching}
        onClearSelectAllMatching={source.onClearSelectAllMatching}
        onOpenSelectAllCommandContext={source.onOpenSelectAllCommandContext}
        onApplyPreset={source.onApplyPreset}
        onSaveFilter={source.onSaveFilter}
        onRefreshSavedFilters={source.onRefreshSavedFilters}
        onApplySavedFilter={source.onApplySavedFilter}
        onOpenSavedFilterCommandContext={source.onOpenSavedFilterCommandContext}
        onDeleteSavedFilter={source.onDeleteSavedFilter}
        onRenameSavedFilter={source.onRenameSavedFilter}
        onShareSavedFilter={source.onShareSavedFilter}
        onExportCurrentPage={source.onExportCurrentPage}
        onAddSelectedToGroup={source.onAddSelectedToGroup}
        onShowSelectedSummary={source.onShowSelectedSummary}
        onOpenOtaContext={source.onOpenOtaContext}
        onOpenAlarmContext={source.onOpenAlarmContext}
        onOpenCommandContext={source.onOpenCommandContext}
        onOpenConfigContext={source.onOpenConfigContext}
        onOpenAuditContext={source.onOpenAuditContext}
      />
    )
  }
}

/** Inputs for the full page header: shared-with-me / claim / add-device entries. */
export interface DeviceManageTopActionsInput extends DeviceFleetToolbarActionInput {
  router: { push: (to: string) => unknown }
  openClaimDeviceDialog: () => unknown
  /** Add-device dropdown selection ('hands' | 'number'); handled by the add drawer. */
  onSelectAddOption: (key: string | number) => unknown
}

/**
 * Build every header top action of the device-manage page: the fleet-target toolbar,
 * the shared-with-me entry, the claim entry, and the add-device dropdown
 * (click-triggered, matching the page's asserted source contract).
 */
export function createDeviceManageTopActions(source: DeviceManageTopActionsInput) {
  const dropOption = [
    {
      label: () => $t('custom.devicePage.manualAdd'),
      key: 'hands'
    },
    {
      label: () => $t('custom.devicePage.addByNumber'),
      key: 'number',
      disabled: false
    }
  ]

  return [
    createDeviceFleetToolbarAction(source),
    {
      element: () => (
        <n-button onClick={() => source.router.push('/device/shared-with-me')}>
          {$t('route.device_shared-with-me')}
        </n-button>
      )
    },
    {
      element: () => <n-button onClick={source.openClaimDeviceDialog}>{$t('custom.devicePage.claimDevice')}</n-button>
    },
    {
      element: () => (
        <n-dropdown options={dropOption} trigger="click" onSelect={source.onSelectAddOption}>
          <n-button type="primary">+{$t('custom.devicePage.addDevice')}</n-button>
        </n-dropdown>
      )
    }
  ]
}
