<script setup lang="ts">
import { computed } from 'vue'
import { NAlert, NEmpty, NFlex, NTag } from 'naive-ui'
import { $t } from '@/locales'
import { buildActionExecutionSummaryItems } from './linkageActionExecutionSummary'

interface Props {
  actionGroups: any[]
  deviceOptions: any[]
  deviceConfigOptions: any[]
  sceneOptions: any[]
  alarmOptions: any[]
}

const props = withDefaults(defineProps<Props>(), {
  actionGroups: () => [],
  deviceOptions: () => [],
  deviceConfigOptions: () => [],
  sceneOptions: () => [],
  alarmOptions: () => []
})

const summaryItems = computed(() =>
  buildActionExecutionSummaryItems(
    props.actionGroups,
    {
      deviceOptions: props.deviceOptions,
      deviceConfigOptions: props.deviceConfigOptions,
      sceneOptions: props.sceneOptions,
      alarmOptions: props.alarmOptions
    },
    {
      unset: $t('generate.actionExecutionUnset'),
      singleDevice: $t('common.singleDevice'),
      singleClassDevice: $t('common.singleClassDevice'),
      operateDevice: $t('common.operateDevice'),
      activateScene: $t('common.activateScene'),
      triggerAlarm: $t('common.triggerAlarm'),
      activate: $t('generate.activate'),
      trigger: $t('generate.trigger')
    }
  )
)
</script>

<template>
  <section class="mb-12px action-execution-summary">
    <NFlex vertical :size="10">
      <NAlert type="info" :show-icon="false">
        <template #header>{{ $t('generate.actionExecutionPreview') }}</template>
        {{ $t('generate.actionExecutionPreviewHint') }}
      </NAlert>
      <NEmpty v-if="summaryItems.length === 0" :description="$t('generate.actionExecutionEmpty')" />
      <NFlex v-else vertical :size="8">
        <NFlex v-for="(item, index) in summaryItems" :key="item.key" align="flex-start" :size="8">
          <NTag round type="info">#{{ index + 1 }} {{ item.tag }}</NTag>
          <NFlex vertical :size="4" class="min-w-0 flex-1">
            <NFlex v-for="line in item.lines" :key="line.key" align="center" :size="6" class="summary-line">
              <NTag size="small" round>{{ line.tag }}</NTag>
              <span class="summary-text">{{ line.text }}</span>
            </NFlex>
          </NFlex>
        </NFlex>
      </NFlex>
    </NFlex>
  </section>
</template>

<style scoped>
.action-execution-summary {
  padding: 12px;
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(24, 160, 88, 0.08), rgba(32, 128, 240, 0.08));
}

.summary-line {
  min-width: 0;
}

.summary-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
