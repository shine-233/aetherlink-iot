<script setup lang="ts">
import { computed } from 'vue'
import { buildCommandPayloadInsight } from './commandCenterPayloadAssistant'

interface Props {
  commandValue: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:commandValue': [value: string]
}>()

const commandPayloadInsight = computed(() => buildCommandPayloadInsight(props.commandValue))

const formatCommandPayload = () => {
  if (!commandPayloadInsight.value.canFormat) return
  emit('update:commandValue', commandPayloadInsight.value.formatted)
}
</script>

<template>
  <div class="command-payload-assistant" :class="`command-payload-assistant--${commandPayloadInsight.type}`">
    <div class="command-payload-assistant__copy">
      <NTag size="small" :type="commandPayloadInsight.type">
        {{ $t(commandPayloadInsight.titleKey) }}
      </NTag>
      <span>
        {{
          $t(commandPayloadInsight.descKey)
            .replace('{fields}', String(commandPayloadInsight.fieldCount))
        }}
      </span>
    </div>
    <NSpace size="small">
      <NButton size="tiny" secondary :disabled="!commandPayloadInsight.canFormat" @click="formatCommandPayload">
        {{ $t('custom.commandCenter.payloadAssistantFormatJson') }}
      </NButton>
      <NButton size="tiny" secondary :disabled="!commandValue.trim()" @click="emit('update:commandValue', '')">
        {{ $t('custom.commandCenter.payloadAssistantClear') }}
      </NButton>
    </NSpace>
  </div>
</template>

<style scoped>
.command-payload-assistant {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fafc;
}

.command-payload-assistant--success {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.command-payload-assistant--error {
  border-color: #fecaca;
  background: #fef2f2;
}

.command-payload-assistant--info {
  border-color: #bae6fd;
  background: #f0f9ff;
}

.command-payload-assistant__copy {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.command-payload-assistant__copy span {
  color: #475569;
  font-size: 12px;
  line-height: 1.45;
}

@media (max-width: 900px) {
  .command-payload-assistant {
    flex-direction: column;
  }
}
</style>
