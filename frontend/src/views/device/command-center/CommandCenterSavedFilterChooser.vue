<script setup lang="ts">
import { ref } from 'vue'
import type { SelectOption } from 'naive-ui'
import { $t } from '@/locales'
import type { SavedFleetFilter } from '../manage/device-fleet-saved-filters'

const props = defineProps<{
  activeSavedFleetFilter: SavedFleetFilter | null
  savedFleetFilterActionError: string
  savedFleetFilterLoading: boolean
  savedFleetFilterNoticeKey: string
  savedFleetFilterOptions: SelectOption[]
  selectedSavedFleetFilterId: string | null
  staleRouteSavedFilter: boolean
  applySavedFleetFilter: (filterId: string | null) => void | Promise<void>
  clearSavedFleetFilterIdentity: () => void | Promise<void>
  deleteSavedFleetFilter: (filterId: string | number) => Promise<boolean>
  refreshSavedFleetFilters: () => void | Promise<void>
  renameSavedFleetFilter: (filterId: string | number, name: string) => Promise<boolean>
}>()

const emit = defineEmits<{
  'update:selectedSavedFleetFilterId': [value: string | null]
}>()

const renameEditing = ref(false)
const renameName = ref('')

function updateSelectedSavedFleetFilterId(value: string | null) {
  emit('update:selectedSavedFleetFilterId', value)
  void props.applySavedFleetFilter(value)
}

function startRename() {
  if (!props.activeSavedFleetFilter) return
  renameName.value = props.activeSavedFleetFilter.name
  renameEditing.value = true
}

function cancelRename() {
  renameEditing.value = false
  renameName.value = ''
}

async function submitRename() {
  if (!props.activeSavedFleetFilter) return
  const nextName = renameName.value.trim()
  if (!nextName) {
    window.$message?.warning($t('custom.commandCenter.savedFilterNameRequired'))
    return
  }

  const renamed = await props.renameSavedFleetFilter(props.activeSavedFleetFilter.id, nextName)
  if (!renamed) {
    window.$message?.warning(props.savedFleetFilterActionError || $t('custom.commandCenter.savedFilterRenameFailed'))
    return
  }

  cancelRename()
  window.$message?.success($t('custom.commandCenter.savedFilterRenamed'))
}

async function deleteActiveSavedFleetFilter() {
  if (!props.activeSavedFleetFilter) return
  const deleted = await props.deleteSavedFleetFilter(props.activeSavedFleetFilter.id)
  if (!deleted) {
    window.$message?.warning(props.savedFleetFilterActionError || $t('custom.commandCenter.savedFilterDeleteFailed'))
    return
  }

  await props.clearSavedFleetFilterIdentity()
  cancelRename()
  window.$message?.success($t('custom.commandCenter.savedFilterDeleted'))
}
</script>

<template>
  <div class="command-saved-filter-picker">
    <div>
      <strong>{{ $t('custom.commandCenter.savedFilterPickerTitle') }}</strong>
      <span>{{ $t('custom.commandCenter.savedFilterPickerDesc') }}</span>
    </div>
    <NSpace>
      <NSelect
        :value="selectedSavedFleetFilterId"
        class="w-260px"
        clearable
        filterable
        :disabled="savedFleetFilterOptions.length === 0"
        :options="savedFleetFilterOptions"
        :placeholder="$t('custom.commandCenter.savedFilterPickerPlaceholder')"
        @update:value="updateSelectedSavedFleetFilterId"
      />
      <NButton size="small" secondary :loading="savedFleetFilterLoading" @click="refreshSavedFleetFilters">
        {{ $t('custom.commandCenter.refreshSavedFilters') }}
      </NButton>
    </NSpace>
    <div v-if="activeSavedFleetFilter" class="command-saved-filter-manager">
      <div class="command-saved-filter-manager__meta">
        <strong>{{ activeSavedFleetFilter.name }}</strong>
        <span>
          {{
            $t('custom.commandCenter.savedFilterManagerMeta').replace(
              '{total}',
              activeSavedFleetFilter.previewTotal === null ? '--' : String(activeSavedFleetFilter.previewTotal)
            )
          }}
        </span>
      </div>
      <div v-if="renameEditing" class="command-saved-filter-manager__rename">
        <NInput
          v-model:value="renameName"
          size="small"
          :placeholder="$t('custom.commandCenter.savedFilterNamePlaceholder')"
          @keyup.enter="submitRename"
        />
        <NButton size="small" type="primary" @click="submitRename">
          {{ $t('common.confirm') }}
        </NButton>
        <NButton size="small" secondary @click="cancelRename">
          {{ $t('common.cancel') }}
        </NButton>
      </div>
      <NSpace v-else :size="[8, 8]">
        <NButton size="small" secondary @click="startRename">
          {{ $t('custom.commandCenter.renameSavedFilter') }}
        </NButton>
        <NPopconfirm @positive-click="deleteActiveSavedFleetFilter">
          <template #trigger>
            <NButton size="small" secondary type="error">
              {{ $t('custom.commandCenter.deleteSavedFilter') }}
            </NButton>
          </template>
          {{ $t('custom.commandCenter.deleteSavedFilterConfirm') }}
        </NPopconfirm>
        <NButton size="small" secondary @click="clearSavedFleetFilterIdentity">
          {{ $t('custom.commandCenter.clearSavedFilterIdentity') }}
        </NButton>
      </NSpace>
    </div>
    <NAlert v-if="savedFleetFilterNoticeKey" type="warning" :show-icon="false">
      <div class="command-saved-filter-notice">
        <span>{{ $t(savedFleetFilterNoticeKey) }}</span>
        <NButton v-if="staleRouteSavedFilter" size="small" secondary @click="clearSavedFleetFilterIdentity">
          {{ $t('custom.commandCenter.clearSavedFilterIdentity') }}
        </NButton>
      </div>
    </NAlert>
  </div>
</template>

<style scoped>
.command-saved-filter-picker {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  background: #ffffff;
}

.command-saved-filter-picker > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-saved-filter-picker strong {
  color: #0f172a;
  font-size: 14px;
}

.command-saved-filter-picker span {
  color: #64748b;
  font-size: 12px;
}

.command-saved-filter-manager {
  display: grid;
  gap: 8px;
  width: 100%;
  padding: 10px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f8fafc;
}

.command-saved-filter-manager__meta {
  display: grid;
  gap: 2px;
}

.command-saved-filter-manager__meta strong {
  overflow-wrap: anywhere;
}

.command-saved-filter-manager__rename {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) auto auto;
  gap: 8px;
  align-items: center;
}

.command-saved-filter-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.command-saved-filter-notice span {
  min-width: 0;
  overflow-wrap: anywhere;
}

@media (max-width: 768px) {
  .command-saved-filter-picker,
  .command-saved-filter-notice {
    flex-direction: column;
  }

  .command-saved-filter-manager__rename {
    grid-template-columns: 1fr;
  }
}
</style>
