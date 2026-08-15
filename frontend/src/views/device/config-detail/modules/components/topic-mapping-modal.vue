<!--
  Topic mapping editor modal.
  The form snapshot lives in formData; isFillingFromEdit prevents edit-mode backfill watchers from clearing fields.
  Save path: validate the form, emit a copy to the parent, then close the modal.
  Topic, direction, and identifier values must stay aligned with protocol parsing rules.
-->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { NAlert, NButton, NForm, NFormItem, NInput, NModal, NPopover, NSelect, NTag, useMessage } from 'naive-ui'
import type { FormInst } from 'naive-ui'
import type { SelectMixedOption } from 'naive-ui/es/select/src/interface'
import { useI18n } from 'vue-i18n'
import { dryRunTopicMapping } from '@/service/api/device'
import type { TopicMappingDryRunResult } from '@/service/api/device'
import {
  MarkdownTip,
  applyTopicMappingDirectionChange,
  applyTopicMappingTargetTopicChange,
  buildTopicMappingProbeTopic,
  buildTopicOptions,
  createDefaultTopicMapping,
  createTopicMappingRules,
  createTopicMappingSaveSnapshot,
  createTopicMappingSnapshot,
  downlinkTopicOptionSource,
  isCommandDownlinkTargetTopic,
  renderTopicLabel,
  renderTopicTag,
  uplinkTopicOptionSource
} from './topicMappingModalHelpers'
import type { TopicMapping } from './topicMappingModalHelpers'

interface Props {
  visible: boolean
  editData?: TopicMapping | null
  deviceConfigId?: string
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  editData: null,
  deviceConfigId: ''
})

interface Emits {
  (e: 'update:visible', visible: boolean): void
  (e: 'save', data: TopicMapping): void
}

const emit = defineEmits<Emits>()

const message = useMessage()
const formRef = ref<FormInst | null>(null)
const { t } = useI18n()
const formData = ref<TopicMapping>(createDefaultTopicMapping())
const isFillingFromEdit = ref(false)
const dryRunProbeTopic = ref('')
const dryRunLoading = ref(false)
const dryRunResult = ref<TopicMappingDryRunResult | null>(null)

const dataDirectionOptions = computed(() => [
  {
    label: t('generate.topicMapping.direction.down'),
    value: 'down'
  },
  {
    label: t('generate.topicMapping.direction.up'),
    value: 'up'
  }
])
const uplinkTopicOptions = computed(() => buildTopicOptions(uplinkTopicOptionSource, t))
const downlinkTopicOptions = computed(() => buildTopicOptions(downlinkTopicOptionSource, t))
const targetTopicOptions = computed(() =>
  formData.value.direction === 'up' ? uplinkTopicOptions.value : downlinkTopicOptions.value
)
const showDataIdentifier = computed(() => isCommandDownlinkTargetTopic(formData.value.target_topic))
const rules = computed(() => createTopicMappingRules(t))
const dryRunStatusType = computed(() => {
  if (!dryRunResult.value) return 'default'
  return dryRunResult.value.matched ? 'success' : 'error'
})
const suggestedProbeTopic = computed(() => buildTopicMappingProbeTopic(formData.value.original_topic || ''))
const mqttTestCommand = computed(() => {
  const probeTopic = dryRunProbeTopic.value.trim()
  if (!probeTopic) return ''
  if (formData.value.direction === 'up') {
    return `mosquitto_pub -h <broker_host> -p 1883 -t "${probeTopic}" -m '{"temperature":25}'`
  }
  return `mosquitto_sub -h <broker_host> -p 1883 -t "${probeTopic}"`
})
const modalVisible = computed({
  get() {
    return props.visible
  },
  set(value) {
    emit('update:visible', value)
  }
})

watch(
  () => props.editData,
  (newData) => {
    isFillingFromEdit.value = true
    formData.value = createTopicMappingSnapshot(newData)
    nextTick(() => {
      isFillingFromEdit.value = false
    })
  },
  { immediate: true }
)

watch(
  () => formData.value.direction,
  () => {
    if (isFillingFromEdit.value) return
    applyTopicMappingDirectionChange(formData.value)
    dryRunResult.value = null
  }
)

watch(
  () => formData.value.target_topic,
  () => {
    applyTopicMappingTargetTopicChange(formData.value)
    dryRunResult.value = null
  }
)

watch(
  () => [formData.value.original_topic, formData.value.data_identifier, dryRunProbeTopic.value],
  () => {
    dryRunResult.value = null
  }
)

watch(
  () => props.visible,
  (visible) => {
    if (!visible) {
      formRef.value?.restoreValidation()
      if (!props.editData) {
        formData.value = createDefaultTopicMapping()
      }
      dryRunProbeTopic.value = ''
      dryRunResult.value = null
    }
  }
)

const handleDryRun = async () => {
  const sourceTopic = formData.value.original_topic?.trim()
  const targetTopic = formData.value.target_topic?.trim()
  const probeTopic = dryRunProbeTopic.value.trim()
  if (!props.deviceConfigId || !sourceTopic || !targetTopic || !probeTopic) {
    message.error(t('generate.topicMapping.dryRun.validation'))
    return
  }

  dryRunLoading.value = true
  try {
    const res = await dryRunTopicMapping({
      device_config_id: props.deviceConfigId,
      direction: formData.value.direction,
      source_topic: sourceTopic,
      target_topic: targetTopic,
      test_topic: probeTopic,
      sample_topic: probeTopic,
      data_identifier: formData.value.data_identifier?.trim()
    })
    dryRunResult.value = ((res as any).data ?? res) as TopicMappingDryRunResult
  } catch {
    message.error(t('generate.topicMapping.dryRun.failed'))
  } finally {
    dryRunLoading.value = false
  }
}

const handleUseSuggestedProbeTopic = () => {
  dryRunProbeTopic.value = suggestedProbeTopic.value
}

const handleCopyTestCommand = async () => {
  if (!mqttTestCommand.value) return
  try {
    await navigator.clipboard.writeText(mqttTestCommand.value)
    message.success(t('generate.topicMapping.dryRun.copySuccess'))
  } catch {
    message.error(t('generate.topicMapping.dryRun.copyFailed'))
  }
}

const handleSave = async () => {
  try {
    await formRef.value?.validate()
    emit('save', createTopicMappingSaveSnapshot(formData.value))
    modalVisible.value = false
  } catch {
    message.error(t('generate.topicMapping.message.validateForm'))
  }
}

const handleCancel = () => {
  modalVisible.value = false
}
</script>

<template>
  <NModal
    v-model:show="modalVisible"
    preset="dialog"
    :title="editData ? t('generate.topicMapping.modal.editTitle') : t('generate.topicMapping.modal.createTitle')"
    style="width: 600px"
    :mask-closable="false"
    :showIcon="false"
  >
    <NForm
      ref="formRef"
      :model="formData"
      :rules="rules"
      label-placement="left"
      label-align="left"
      label-width="auto"
      class="mt-6"
      require-mark-placement="right-hanging"
    >
      <NFormItem :label="t('generate.topicMapping.form.mappingName')" path="mapping_name">
        <NInput v-model:value="formData.mapping_name" :placeholder="t('generate.topicMapping.placeholder.input')" />
      </NFormItem>

      <NFormItem :label="t('generate.topicMapping.form.direction')" path="direction">
        <NSelect
          v-model:value="formData.direction"
          :options="dataDirectionOptions"
          :placeholder="t('generate.topicMapping.placeholder.selectDirection')"
        />
      </NFormItem>

      <NFormItem :label="t('generate.topicMapping.form.originalTopic')" path="original_topic">
        <NInput v-model:value="formData.original_topic" :placeholder="t('generate.topicMapping.placeholder.input')" />
        <template #feedback>
          <div class="form-tip">
            <NPopover trigger="click">
              <template #trigger>
                <NButton text size="small" class="text-primary">
                  {{ t('generate.topicMapping.viewGuide') }}
                </NButton>
              </template>
              <div class="detailed-tip">
                <template v-if="formData.direction === 'up'">
                  <div class="tip-title">{{ t('generate.topicMapping.tips.uplink.title') }}</div>
                  <div class="tip-content">
                    <div class="tip-section">
                      <div class="tip-label">{{ t('generate.topicMapping.tips.definitionLabel') }}</div>
                      <MarkdownTip :text="t('generate.topicMapping.tips.uplink.definition')" />
                    </div>
                    <div class="tip-section">
                      <div class="tip-label">{{ t('generate.topicMapping.tips.referenceLabel') }}</div>
                      <MarkdownTip :text="t('generate.topicMapping.tips.uplink.reference')" />
                    </div>
                    <div class="tip-section">
                      <div class="tip-label">{{ t('generate.topicMapping.tips.messageIdLabel') }}</div>
                      <MarkdownTip :text="t('generate.topicMapping.tips.uplink.messageIdLine1')" />
                      <MarkdownTip :text="t('generate.topicMapping.tips.uplink.messageIdLine2')" />
                    </div>
                  </div>
                </template>
                <template v-else>
                  <div class="tip-title">{{ t('generate.topicMapping.tips.downlink.title') }}</div>
                  <div class="tip-content">
                    <div class="tip-section">
                      <div class="tip-label">{{ t('generate.topicMapping.tips.definitionLabel') }}</div>
                      <MarkdownTip :text="t('generate.topicMapping.tips.downlink.definition')" />
                    </div>
                    <div class="tip-section">
                      <div class="tip-label">{{ t('generate.topicMapping.tips.referenceLabel') }}</div>
                      <MarkdownTip :text="t('generate.topicMapping.tips.downlink.reference')" />
                    </div>
                    <div class="tip-section">
                      <div class="tip-label">{{ t('generate.topicMapping.tips.multiMappingLabel') }}</div>
                      <MarkdownTip :text="t('generate.topicMapping.tips.downlink.multiMapping')" />
                      <MarkdownTip :text="t('generate.topicMapping.tips.downlink.multiMappingConfig')" />
                      <MarkdownTip :text="t('generate.topicMapping.tips.downlink.multiMappingProcess')" />
                    </div>
                  </div>
                </template>
              </div>
            </NPopover>
          </div>
        </template>
      </NFormItem>

      <NFormItem :label="t('generate.topicMapping.form.targetTopic')" path="target_topic" class="mt-4">
        <NSelect
          v-model:value="formData.target_topic"
          :options="targetTopicOptions as unknown as SelectMixedOption[]"
          :placeholder="t('generate.topicMapping.placeholder.selectTarget')"
          :render-label="renderTopicLabel"
          :render-tag="renderTopicTag"
        />
      </NFormItem>

      <NFormItem
        v-if="showDataIdentifier"
        :label="t('generate.topicMapping.form.dataIdentifier')"
        path="data_identifier"
      >
        <NInput v-model:value="formData.data_identifier" :placeholder="t('generate.topicMapping.placeholder.input')" />
        <template #feedback>
          <div class="form-tip">
            {{ t('generate.topicMapping.tips.dataIdentifier') }}
          </div>
        </template>
      </NFormItem>

      <NFormItem :label="t('generate.topicMapping.dryRun.testTopic')">
        <div class="dry-run-section">
          <div class="dry-run-input">
            <NInput
              v-model:value="dryRunProbeTopic"
              :placeholder="t('generate.topicMapping.dryRun.testPlaceholder')"
              clearable
            />
            <NButton secondary :disabled="!suggestedProbeTopic" @click="handleUseSuggestedProbeTopic">
              {{ t('generate.topicMapping.dryRun.useSuggestedTopic') }}
            </NButton>
            <NButton type="info" :loading="dryRunLoading" @click="handleDryRun">
              {{ t('generate.topicMapping.dryRun.action') }}
            </NButton>
          </div>

          <NAlert
            v-if="dryRunResult"
            :type="dryRunStatusType === 'success' ? 'success' : 'error'"
            :title="
              dryRunResult.matched
                ? t('generate.topicMapping.dryRun.matched')
                : t('generate.topicMapping.dryRun.notMatched')
            "
            class="dry-run-result"
            :show-icon="false"
          >
            <div class="dry-run-summary">
              <div>
                <span>{{ t('generate.topicMapping.dryRun.resolvedTopic') }}</span>
                <strong>{{ dryRunResult.resolved_topic || '-' }}</strong>
              </div>
              <div v-if="dryRunResult.data_identifier">
                <span>{{ t('generate.topicMapping.form.dataIdentifier') }}</span>
                <strong>{{ dryRunResult.data_identifier }}</strong>
              </div>
            </div>

            <div class="dry-run-diagnostics">
              <div v-for="item in dryRunResult.diagnostics" :key="`${item.scope}-${item.message}`" class="dry-run-row">
                <NTag size="small" :type="item.severity === 'success' ? 'success' : item.severity">
                  {{ item.severity }}
                </NTag>
                <span>{{ item.message }}</span>
              </div>
            </div>

            <div v-if="dryRunResult.next_steps?.length" class="dry-run-next">
              <div class="dry-run-next-title">{{ t('generate.topicMapping.dryRun.nextSteps') }}</div>
              <div v-for="step in dryRunResult.next_steps" :key="step" class="dry-run-next-step">
                {{ step }}
              </div>
            </div>

            <div v-if="mqttTestCommand" class="dry-run-command">
              <div class="dry-run-command-header">
                <span>{{ t('generate.topicMapping.dryRun.testCommand') }}</span>
                <NButton size="tiny" secondary @click="handleCopyTestCommand">
                  {{ t('generate.topicMapping.dryRun.copyCommand') }}
                </NButton>
              </div>
              <code>{{ mqttTestCommand }}</code>
            </div>
          </NAlert>
        </div>
      </NFormItem>

      <NFormItem :label="t('generate.topicMapping.form.description')" path="description">
        <NInput
          v-model:value="formData.description"
          :placeholder="t('generate.topicMapping.placeholder.input')"
          type="textarea"
          :rows="3"
        />
      </NFormItem>
    </NForm>

    <template #action>
      <div class="modal-footer">
        <NButton @click="handleCancel">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" @click="handleSave">{{ t('common.save') }}</NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped lang="scss">
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.form-tip {
  margin-top: 4px;
  font-size: 12px;
  color: #999;
  line-height: 1.5;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.dry-run-section {
  width: 100%;
}

.dry-run-input {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 8px;
  width: 100%;
}

.dry-run-result {
  margin-top: 10px;
}

.dry-run-summary {
  display: grid;
  gap: 6px;
  margin-bottom: 10px;

  div {
    display: flex;
    gap: 8px;
    align-items: baseline;
    min-width: 0;
  }

  span {
    flex: 0 0 auto;
    color: #666;
  }

  strong {
    min-width: 0;
    word-break: break-all;
  }
}

.dry-run-diagnostics,
.dry-run-next {
  display: grid;
  gap: 6px;
}

.dry-run-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 8px;
  align-items: center;
}

.dry-run-next {
  margin-top: 10px;
}

.dry-run-next-title {
  font-weight: 600;
  color: #333;
}

.dry-run-next-step {
  color: #555;
  line-height: 1.5;
}

.dry-run-command {
  margin-top: 10px;
  display: grid;
  gap: 6px;

  code {
    display: block;
    padding: 8px;
    border-radius: 4px;
    background: #f5f5f5;
    color: #333;
    word-break: break-all;
    line-height: 1.5;
  }
}

.dry-run-command-header {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
  font-weight: 600;
  color: #333;
}

.detailed-tip {
  min-width: 450px;
  width: 750px;
  padding: 8px 12px;

  .tip-title {
    font-size: 16px;
    font-weight: 600;
    color: #333;
    margin-bottom: 8px;
  }

  .tip-content {
    font-size: 14px;
    line-height: 1.6;
  }

  .tip-section {
    margin-bottom: 12px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  .tip-label {
    font-weight: 500;
    color: #333;
    margin-bottom: 4px;
  }

  .tip-text {
    color: #666;
    margin-bottom: 6px;
    padding-left: 8px;
    white-space: pre-line;

    &:last-child {
      margin-bottom: 0;
    }

    :deep(code) {
      background-color: #f5f5f5;
      padding: 2px 4px;
      border-radius: 3px;
      font-family: 'Courier New', monospace;
      font-size: 12px;
      color: #e83e8c;
    }

    :deep(.code-block) {
      background-color: #f5f5f5;
      padding: 8px 12px;
      border-radius: 4px;
      margin: 8px 0;
      overflow-x: auto;
      white-space: pre;
      font-family: 'Courier New', monospace;
      font-size: 12px;
      line-height: 1.5;
    }

    :deep(.code-block code) {
      background-color: transparent;
      padding: 0;
      color: #333;
    }

    :deep(strong) {
      font-weight: 600;
      color: #333;
    }
  }
}

.topic-option {
  padding: 4px 0;

  .topic-option-label {
    font-size: 14px;
    color: #333;
    line-height: 1.5;
    margin-bottom: 2px;
  }

  .topic-option-desc {
    font-size: 12px;
    color: #999;
    line-height: 1.4;
  }
}
</style>
