<script setup lang="ts">
import { computed, ref } from 'vue'
import { buildBuiltInCommandTemplates, type BuiltInCommandTemplate } from './commandCenterCommandTemplates'
import type { CommandCenterSavedCommandTemplate } from './useCommandCenterCommandTemplates'

interface Props {
  commandIdentify: string
  commandTemplateName: string
  hasCommandJobScope: boolean
  isDeviceFilterScope: boolean
  savedCommandTemplates: CommandCenterSavedCommandTemplate[]
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:commandTemplateName': [value: string]
  applyBuiltInCommandTemplate: [template: { identify: string; value: string; timeoutSeconds: number }]
  applySavedCommandTemplate: [template: CommandCenterSavedCommandTemplate]
  copySavedCommandTemplate: [template: CommandCenterSavedCommandTemplate]
  copySavedCommandTemplates: []
  deleteSavedCommandTemplate: [templateId: string]
  importSavedCommandTemplates: [raw: string]
  saveCommandTemplate: []
}>()

const commandTemplateImportText = ref('')
const canImportCommandTemplates = computed(() => commandTemplateImportText.value.trim().length > 0)

const commandJobTemplates = computed(() => buildBuiltInCommandTemplates())

const applyCommandJobTemplate = (template: BuiltInCommandTemplate) => {
  emit('applyBuiltInCommandTemplate', {
    identify: template.identify,
    value: template.value,
    timeoutSeconds: template.timeoutSeconds
  })
}

const importSavedCommandTemplates = () => {
  const raw = commandTemplateImportText.value.trim()
  if (!raw) return
  emit('importSavedCommandTemplates', raw)
  commandTemplateImportText.value = ''
}
</script>

<template>
  <div class="command-template-library">
    <div class="command-template-library__head">
      <div>
        <strong>{{ $t('custom.commandCenter.quickStarterTitle') }}</strong>
        <span>{{ $t('custom.commandCenter.quickStarterDesc') }}</span>
      </div>
      <NTag size="small" type="info">
        {{
          isDeviceFilterScope
            ? $t('custom.commandCenter.templateScopeSavedFilter')
            : $t('custom.commandCenter.templateScopeSelectedDevices')
        }}
      </NTag>
    </div>
    <div class="command-template-save">
      <NInput
        :value="commandTemplateName"
        clearable
        size="small"
        :placeholder="$t('custom.commandCenter.templateNamePlaceholder')"
        @update:value="emit('update:commandTemplateName', $event)"
      />
      <NButton
        size="small"
        type="primary"
        secondary
        :disabled="!commandIdentify.trim()"
        @click="emit('saveCommandTemplate')"
      >
        {{ $t('custom.commandCenter.saveCommandTemplate') }}
      </NButton>
    </div>
    <div class="command-template-import">
      <div>
        <strong>{{ $t('custom.commandCenter.importCommandTemplatesTitle') }}</strong>
        <span>{{ $t('custom.commandCenter.importCommandTemplatesDesc') }}</span>
      </div>
      <NInput
        v-model:value="commandTemplateImportText"
        type="textarea"
        size="small"
        :autosize="{ minRows: 2, maxRows: 5 }"
        :placeholder="$t('custom.commandCenter.importCommandTemplatesPlaceholder')"
      />
      <NButton size="small" secondary :disabled="!canImportCommandTemplates" @click="importSavedCommandTemplates">
        {{ $t('custom.commandCenter.importCommandTemplates') }}
      </NButton>
    </div>
    <div class="command-template-grid">
      <button
        v-for="template in commandJobTemplates"
        :key="template.key"
        type="button"
        class="command-template-card"
        :disabled="!hasCommandJobScope"
        @click="applyCommandJobTemplate(template)"
      >
        <span class="command-template-card__top">
          <strong>{{ $t(template.titleKey) }}</strong>
          <NTag size="small" :type="template.tagType">{{ template.identify }}</NTag>
        </span>
        <span>{{ $t(template.descKey) }}</span>
        <small>
          {{ $t('custom.commandCenter.templateTimeoutHint').replace('{seconds}', String(template.timeoutSeconds)) }}
        </small>
      </button>
    </div>
    <div v-if="savedCommandTemplates.length" class="command-template-saved">
      <div class="command-template-saved__head">
        <div>
          <strong>{{ $t('custom.commandCenter.savedCommandTemplatesTitle') }}</strong>
          <span>{{ $t('custom.commandCenter.savedCommandTemplatesDesc') }}</span>
        </div>
        <NButton size="small" secondary @click="emit('copySavedCommandTemplates')">
          {{ $t('custom.commandCenter.copyCommandTemplates') }}
        </NButton>
      </div>
      <div class="command-template-grid">
        <div
          v-for="template in savedCommandTemplates"
          :key="template.id"
          class="command-template-card command-template-card--saved"
        >
          <span class="command-template-card__top">
            <strong>{{ template.name }}</strong>
            <NTag size="small" type="success">{{ template.identify }}</NTag>
          </span>
          <span>{{ template.value || $t('custom.commandCenter.savedCommandTemplateNoPayload') }}</span>
          <small>
            {{ $t('custom.commandCenter.templateTimeoutHint').replace('{seconds}', String(template.timeoutSeconds)) }}
          </small>
          <NSpace :size="[8, 8]">
            <NButton
              size="small"
              secondary
              :disabled="!hasCommandJobScope"
              @click="emit('applySavedCommandTemplate', template)"
            >
              {{ $t('custom.commandCenter.applyCommandTemplate') }}
            </NButton>
            <NButton size="small" secondary @click="emit('copySavedCommandTemplate', template)">
              {{ $t('custom.commandCenter.copyCommandTemplate') }}
            </NButton>
            <NPopconfirm @positive-click="emit('deleteSavedCommandTemplate', template.id)">
              <template #trigger>
                <NButton size="small" secondary type="error">
                  {{ $t('common.delete') }}
                </NButton>
              </template>
              {{ $t('custom.commandCenter.deleteCommandTemplateConfirm') }}
            </NPopconfirm>
          </NSpace>
        </div>
      </div>
    </div>
    <NAlert v-if="!hasCommandJobScope" type="warning" :show-icon="false">
      {{ $t('custom.commandCenter.templateNeedsScope') }}
    </NAlert>
  </div>
</template>

<style scoped>
.command-template-library {
  display: grid;
  gap: 12px;
  padding: 12px;
  border: 1px solid #bfdbfe;
  border-radius: 10px;
  background: linear-gradient(135deg, #eff6ff 0%, #f8fafc 100%);
}

.command-template-library__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-template-library__head > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-template-library__head strong {
  color: #0f172a;
  font-size: 14px;
}

.command-template-library__head span {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.command-template-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.command-template-save {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

.command-template-import {
  display: grid;
  gap: 8px;
  padding: 10px;
  border: 1px dashed #93c5fd;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.68);
}

.command-template-import > div {
  display: grid;
  gap: 4px;
}

.command-template-import strong {
  color: #0f172a;
  font-size: 13px;
}

.command-template-import span {
  color: #475569;
  font-size: 12px;
  line-height: 1.45;
}

.command-template-card {
  display: grid;
  gap: 8px;
  min-width: 0;
  padding: 12px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #fff;
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.command-template-card:not(:disabled):hover {
  border-color: #60a5fa;
  box-shadow: 0 12px 24px rgba(37, 99, 235, 0.12);
  transform: translateY(-1px);
}

.command-template-card:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.command-template-card--saved {
  cursor: default;
}

.command-template-card.command-template-card--saved:hover {
  border-color: #dbeafe;
  box-shadow: none;
  transform: none;
}

.command-template-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.command-template-card strong {
  min-width: 0;
  color: #0f172a;
  font-size: 13px;
}

.command-template-card span,
.command-template-card small {
  overflow-wrap: anywhere;
  color: #64748b;
  font-size: 12px;
  line-height: 1.45;
}

.command-template-saved {
  display: grid;
  gap: 10px;
}

.command-template-saved__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-template-saved__head > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-template-saved__head strong {
  color: #0f172a;
  font-size: 13px;
}

.command-template-saved__head span {
  color: #475569;
  font-size: 12px;
}

@media (max-width: 900px) {
  .command-template-library__head,
  .command-template-card__top,
  .command-template-saved__head {
    flex-direction: column;
  }

  .command-template-grid,
  .command-template-save {
    grid-template-columns: 1fr;
  }
}
</style>
