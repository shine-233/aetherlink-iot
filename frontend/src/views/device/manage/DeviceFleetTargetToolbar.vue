<script setup lang="ts">
import type { DropdownOption } from 'naive-ui'
import { computed, ref, watch } from 'vue'
import { $t } from '@/locales'
import type { FleetTargetPreset, FleetTargetPresetKey } from './device-fleet-target-presets'
import type { FleetSelectionScope, FleetSelectionScopeMessage } from './device-fleet-select-all'

type SavedFleetFilterOption = DropdownOption & {
  key: string | number
  rawName?: string
  shared?: boolean
  owned?: boolean
}

const props = defineProps<{
  presets: FleetTargetPreset[]
  activePreset: FleetTargetPresetKey
  targetPreviewTotal: number | null
  currentPageDeviceCount: number
  savedFilterOptions: SavedFleetFilterOption[]
  savedFilterCount: number
  canSaveCurrentFleetFilter: boolean
  selectedDeviceCount: number
  selectionScope: FleetSelectionScope
  selectionScopeMessage: FleetSelectionScopeMessage
  canSelectAllMatching: boolean
}>()

const emit = defineEmits<{
  applyPreset: [presetKey: FleetTargetPresetKey]
  saveFilter: []
  refreshSavedFilters: []
  applySavedFilter: [filterID: string | number]
  openSavedFilterCommandContext: [filterID: string | number]
  deleteSavedFilter: [filterID: string | number]
  renameSavedFilter: [filterID: string | number, name: string]
  shareSavedFilter: [filterID: string | number, shared: boolean]
  exportCurrentPage: []
  showSelectedSummary: []
  addSelectedToGroup: []
  openOtaContext: []
  openAlarmContext: []
  openCommandContext: []
  openConfigContext: []
  openAuditContext: []
  selectAllMatching: []
  clearSelectAllMatching: []
  openSelectAllCommandContext: []
}>()

// 选中语义的说明文案由父级算好 key + 参数，这里只负责渲染，保证「当页」和「全部匹配」不会混淆。
const selectionScopeText = computed(() =>
  $t(props.selectionScopeMessage.key, props.selectionScopeMessage.params)
)

const isAllMatchingSelection = computed(() => props.selectionScope.mode === 'all_matching')

const editingSavedFilterId = ref<string | number | null>(null)
const editingSavedFilterName = ref('')
const submittedSavedFilterName = ref('')

const selectedSavedFilter = computed(() =>
  props.savedFilterOptions.find((option) => option.key === editingSavedFilterId.value)
)

const startRenameSavedFilter = (filterID: string | number) => {
  editingSavedFilterId.value = filterID
  const option = selectedSavedFilter.value
  editingSavedFilterName.value =
    option?.rawName || (typeof option?.label === 'string' ? option.label : '')
  submittedSavedFilterName.value = ''
}

const submitRenameSavedFilter = () => {
  if (!editingSavedFilterId.value || !editingSavedFilterName.value.trim()) return
  submittedSavedFilterName.value = editingSavedFilterName.value.trim()
  emit('renameSavedFilter', editingSavedFilterId.value, submittedSavedFilterName.value)
}

const cancelRenameSavedFilter = () => {
  editingSavedFilterId.value = null
  editingSavedFilterName.value = ''
  submittedSavedFilterName.value = ''
}

// Only owned filters may toggle sharing; a shared one flips back to private.
const shareableSavedFilterOptions = computed(() =>
  props.savedFilterOptions
    .filter((option) => option.owned !== false)
    .map((option) => ({
      ...option,
      label: option.shared
        ? `${option.rawName || ''} · ${$t('custom.devicePage.savedFleetFilterSharedBadge')}`
        : option.rawName || (typeof option.label === 'string' ? option.label : '')
    }))
)

const toggleShareSavedFilter = (filterID: string | number) => {
  const option = props.savedFilterOptions.find((item) => item.key === filterID)
  if (!option) return
  emit('shareSavedFilter', filterID, !option.shared)
}

watch(
  () => props.savedFilterOptions,
  () => {
    if (!editingSavedFilterId.value) return
    const stillExists = props.savedFilterOptions.some((option) => option.key === editingSavedFilterId.value)
    if (!stillExists) {
      cancelRenameSavedFilter()
      return
    }
    const option = selectedSavedFilter.value
    if (submittedSavedFilterName.value && option?.rawName === submittedSavedFilterName.value) {
      cancelRenameSavedFilter()
    }
  }
)
</script>

<template>
  <div class="fleet-toolbar">
    <div class="fleet-toolbar__filters">
      <span class="fleet-toolbar__label">{{ $t('custom.devicePage.fleetTargetPresets') }}</span>
      <NButtonGroup size="small">
        <NButton
          v-for="preset in presets"
          :key="preset.key"
          :type="activePreset === preset.key ? 'primary' : 'default'"
          :secondary="activePreset === preset.key"
          @click="$emit('applyPreset', preset.key)"
        >
          {{ $t(preset.labelKey) }}
        </NButton>
      </NButtonGroup>
      <NTag size="small" type="info">
        {{ $t('custom.devicePage.fleetTargetPreviewCount') }}: {{ targetPreviewTotal ?? '--' }}
      </NTag>
      <NTag size="small" type="warning">
        {{ $t('custom.devicePage.fleetCurrentPageOnly') }}
      </NTag>
      <NButton size="small" secondary :disabled="!canSaveCurrentFleetFilter" @click="$emit('saveFilter')">
        {{ $t('custom.devicePage.saveFleetFilter') }}
      </NButton>
      <NButton size="small" secondary @click="$emit('refreshSavedFilters')">
        {{ $t('custom.devicePage.refreshSavedFilters') }}
      </NButton>
      <NDropdown :options="savedFilterOptions" trigger="click" @select="$emit('applySavedFilter', $event)">
        <NButton size="small" :disabled="savedFilterCount === 0">
          {{ $t('custom.devicePage.savedFleetFilters') }}
        </NButton>
      </NDropdown>
      <NDropdown :options="savedFilterOptions" trigger="click" @select="$emit('openSavedFilterCommandContext', $event)">
        <NButton size="small" secondary :disabled="savedFilterCount === 0">
          {{ $t('custom.devicePage.openSavedFilterCommand') }}
        </NButton>
      </NDropdown>
      <NDropdown :options="savedFilterOptions" trigger="click" @select="$emit('deleteSavedFilter', $event)">
        <NButton size="small" secondary type="warning" :disabled="savedFilterCount === 0">
          {{ $t('custom.devicePage.deleteSavedFilter') }}
        </NButton>
      </NDropdown>
      <NDropdown :options="savedFilterOptions" trigger="click" @select="startRenameSavedFilter">
        <NButton size="small" secondary :disabled="savedFilterCount === 0">
          {{ $t('custom.devicePage.renameSavedFilter') }}
        </NButton>
      </NDropdown>
      <NDropdown
        :options="shareableSavedFilterOptions"
        trigger="click"
        @select="toggleShareSavedFilter"
      >
        <NButton size="small" secondary :disabled="shareableSavedFilterOptions.length === 0">
          {{ $t('custom.devicePage.shareSavedFilter') }}
        </NButton>
      </NDropdown>
      <div v-if="editingSavedFilterId" class="fleet-toolbar__rename">
        <NInput
          v-model:value="editingSavedFilterName"
          size="small"
          class="fleet-toolbar__rename-input"
          :placeholder="$t('custom.devicePage.savedFleetFilterNamePlaceholder')"
        />
        <NButton
          size="small"
          type="primary"
          :disabled="!editingSavedFilterName.trim()"
          @click="submitRenameSavedFilter"
        >
          {{ $t('common.save') }}
        </NButton>
        <NButton
          size="small"
          secondary
          @click="cancelRenameSavedFilter"
        >
          {{ $t('common.cancel') }}
        </NButton>
      </div>
    </div>

    <div class="fleet-toolbar__actions">
      <div class="fleet-toolbar__launchpad">
        <div>
          <span class="fleet-toolbar__eyebrow">Fleet operations</span>
          <strong>{{ $t('custom.devicePage.fleetLaunchpadTitle') }}</strong>
          <p>
            {{ $t('custom.devicePage.fleetLaunchpadDesc') }}
          </p>
        </div>
        <div class="fleet-toolbar__summary">
          <NTag size="small" type="success">
            {{ $t('custom.devicePage.fleetCurrentPageCount') }}: {{ currentPageDeviceCount }}
          </NTag>
          <NTag size="small" type="info">
            {{ $t('custom.devicePage.fleetSelectedCount') }}: {{ selectedDeviceCount }}
          </NTag>
        </div>
      </div>

      <div
        class="fleet-toolbar__select-all"
        :class="{ 'is-all-matching': isAllMatchingSelection }"
        data-testid="fleet-select-all-banner"
      >
        <div class="fleet-toolbar__select-all-body">
          <strong data-testid="fleet-selection-scope-text">{{ selectionScopeText }}</strong>
          <small v-if="selectionScope.truncated" data-testid="fleet-selection-cap-warning">
            {{
              $t('custom.devicePage.fleetSelectAllCapWarning', {
                effective: selectionScope.effectiveCount,
                max: selectionScope.maxDevices,
                skipped: selectionScope.skippedCount
              })
            }}
          </small>
        </div>
        <div class="fleet-toolbar__select-all-actions">
          <NButton
            v-if="!isAllMatchingSelection"
            size="small"
            type="primary"
            secondary
            :disabled="!canSelectAllMatching"
            data-testid="fleet-select-all-button"
            @click="$emit('selectAllMatching')"
          >
            {{ $t('custom.devicePage.fleetSelectAllMatching', { matched: selectionScope.matchedTotal }) }}
          </NButton>
          <template v-else>
            <NButton
              size="small"
              type="primary"
              data-testid="fleet-select-all-command-button"
              @click="$emit('openSelectAllCommandContext')"
            >
              {{ $t('custom.devicePage.fleetSelectAllOpenCommand', { effective: selectionScope.effectiveCount }) }}
            </NButton>
            <NButton
              size="small"
              secondary
              data-testid="fleet-select-all-clear-button"
              @click="$emit('clearSelectAllMatching')"
            >
              {{ $t('custom.devicePage.fleetSelectAllClear') }}
            </NButton>
          </template>
        </div>
      </div>
      <div class="fleet-toolbar__action-grid">
        <div class="fleet-toolbar__action-card fleet-toolbar__action-card--primary">
          <span>Selected devices</span>
          <strong>{{ $t('custom.devicePage.fleetSelectedActionTitle') }}</strong>
          <p>{{ $t('custom.devicePage.fleetSelectedActionDesc', { count: selectedDeviceCount }) }}</p>
          <div class="fleet-toolbar__card-actions">
            <NButton size="small" type="primary" secondary :disabled="selectedDeviceCount === 0" @click="$emit('openCommandContext')">
              {{ $t('custom.devicePage.openFleetCommand') }}
            </NButton>
            <NButton size="small" secondary :disabled="selectedDeviceCount === 0" @click="$emit('openConfigContext')">
              {{ $t('custom.devicePage.openFleetConfigTag') }}
            </NButton>
            <NButton size="small" secondary :disabled="selectedDeviceCount === 0" @click="$emit('addSelectedToGroup')">
              {{ $t('custom.devicePage.addSelectedToGroup') }}
            </NButton>
          </div>
        </div>
        <div class="fleet-toolbar__action-card">
          <span>Current scope</span>
          <strong>{{ $t('custom.devicePage.fleetScopeActionTitle') }}</strong>
          <p>{{ $t('custom.devicePage.fleetScopeActionDesc', { count: currentPageDeviceCount }) }}</p>
          <div class="fleet-toolbar__card-actions">
            <NButton size="small" secondary :disabled="currentPageDeviceCount === 0" @click="$emit('openOtaContext')">
              {{ $t('custom.devicePage.openFleetOta') }}
            </NButton>
            <NButton size="small" secondary :disabled="currentPageDeviceCount === 0" @click="$emit('exportCurrentPage')">
              {{ $t('custom.devicePage.exportCurrentFleetPage') }}
            </NButton>
          </div>
        </div>
        <div class="fleet-toolbar__action-card">
          <span>Evidence</span>
          <strong>{{ $t('custom.devicePage.fleetEvidenceActionTitle') }}</strong>
          <p>{{ $t('custom.devicePage.fleetEvidenceActionDesc') }}</p>
          <div class="fleet-toolbar__card-actions">
            <NButton size="small" secondary :disabled="currentPageDeviceCount === 0" @click="$emit('openAlarmContext')">
              {{ $t('custom.devicePage.openFleetAlarms') }}
            </NButton>
            <NButton size="small" secondary :disabled="currentPageDeviceCount === 0" @click="$emit('openAuditContext')">
              {{ $t('custom.devicePage.openFleetAudit') }}
            </NButton>
            <NButton size="small" secondary :disabled="selectedDeviceCount === 0" @click="$emit('showSelectedSummary')">
              {{ $t('custom.devicePage.checkSelectedDevices') }}
            </NButton>
          </div>
        </div>
      </div>
      <div class="fleet-toolbar__hint">
        {{ $t('custom.devicePage.fleetActionHint') }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.fleet-toolbar {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 100%;
}

.fleet-toolbar__filters,
.fleet-toolbar__summary,
.fleet-toolbar__action-grid {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.fleet-toolbar__label,
.fleet-toolbar__hint {
  color: #666;
  font-size: 13px;
}

.fleet-toolbar__actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.fleet-toolbar__launchpad {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding: 14px;
  border: 1px solid #dbeafe;
  border-radius: 14px;
  background: linear-gradient(135deg, #f8fbff 0%, #ffffff 100%);
}

.fleet-toolbar__launchpad strong,
.fleet-toolbar__action-card strong {
  display: block;
  color: #111827;
  font-weight: 700;
}

.fleet-toolbar__launchpad p,
.fleet-toolbar__action-card p {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.fleet-toolbar__eyebrow,
.fleet-toolbar__action-card span {
  display: block;
  margin-bottom: 4px;
  color: #2563eb;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.fleet-toolbar__action-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: stretch;
}

.fleet-toolbar__action-card {
  display: flex;
  min-width: 0;
  flex-direction: column;
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
}

.fleet-toolbar__action-card--primary {
  border-color: #bfdbfe;
  background: #f8fbff;
}

.fleet-toolbar__card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.fleet-toolbar__select-all {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
}

.fleet-toolbar__select-all.is-all-matching {
  border-color: #bfdbfe;
  background: #eff6ff;
}

.fleet-toolbar__select-all-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.fleet-toolbar__select-all-body strong {
  color: #111827;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.45;
}

.fleet-toolbar__select-all-body small {
  color: #b45309;
  font-size: 12px;
  line-height: 1.45;
}

.fleet-toolbar__select-all-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.fleet-toolbar__rename {
  display: flex;
  align-items: center;
  gap: 6px;
}

.fleet-toolbar__rename-input {
  width: 220px;
}

@media (max-width: 960px) {
  .fleet-toolbar__launchpad {
    flex-direction: column;
  }

  .fleet-toolbar__action-grid {
    grid-template-columns: 1fr;
  }
}
</style>
