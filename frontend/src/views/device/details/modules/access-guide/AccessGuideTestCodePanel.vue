<!--
  文件用途: 接入测试代码与原始连接信息（默认只展开第一段测试代码）。
  核心逻辑: DeviceAccessGuide 拆分出的展示分区；复制与调试动作通过 emit 交回 DeviceAccessGuide 统一转发。
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { $t } from '@/locales'
import type { DeviceAccessGuideState } from '../device-access-guide-state'

const props = defineProps<{
  accessGuide: DeviceAccessGuideState
  connectInfo: Record<string, unknown>
}>()

const emit = defineEmits<{
  copy: [text: unknown]
  downloadAccessPacket: []
}>()

const commandTestCodeVisible = ref(false)
const visibleCommands = computed(() =>
  commandTestCodeVisible.value ? props.accessGuide.commands : props.accessGuide.commands.slice(0, 1)
)
const hiddenCommandCount = computed(() => Math.max(props.accessGuide.commands.length - visibleCommands.value.length, 0))
const hiddenCommandCountText = computed(() =>
  String($t('custom.device_details.accessGuideHiddenTestCodeCount')).replace(
    '{count}',
    String(hiddenCommandCount.value)
  )
)
</script>

<template>
  <NScrollbar class="access-guide-scroll">
    <NCard class="mb-4" data-testid="device-access-guide-test-code">
      <div class="access-guide-section-heading">
        <div>
          <div class="access-guide-section-title">{{ $t('custom.device_details.accessGuideTestCode') }}</div>
          <div class="access-guide-section-subtitle">
            {{ $t('custom.device_details.accessGuideTestCodeHint') }}
          </div>
        </div>
        <div class="access-guide-section-actions">
          <NButton size="small" type="primary" secondary @click="emit('copy', accessGuide.sdkBundle)">
            {{ $t('custom.device_details.accessGuideCopySdkBundle') }}
          </NButton>
          <NButton
            size="small"
            secondary
            data-testid="device-access-guide-download-access-packet"
            @click="emit('downloadAccessPacket')"
          >
            {{ $t('custom.device_details.accessGuideDownloadSdkBundle') }}
          </NButton>
        </div>
      </div>
      <div class="access-guide-command-list">
        <div v-for="command in visibleCommands" :key="command.titleKey" class="access-guide-command">
          <div class="access-guide-command-header">
            <strong>{{ $t(command.titleKey) }}</strong>
            <NButton size="small" tertiary @click="emit('copy', command.code)">{{ $t('generate.copy') }}</NButton>
          </div>
          <NCode :code="command.code" :language="command.language" word-wrap />
        </div>
        <div v-if="hiddenCommandCount" class="access-guide-command-more">
          <span>{{ hiddenCommandCountText }}</span>
          <NButton size="small" secondary @click="commandTestCodeVisible = true">
            {{ $t('custom.device_details.accessGuideShowAllTestCode') }}
          </NButton>
        </div>
        <div v-else-if="accessGuide.commands.length > 1" class="access-guide-command-more">
          <NButton size="small" tertiary @click="commandTestCodeVisible = false">
            {{ $t('custom.device_details.accessGuideCollapseTestCode') }}
          </NButton>
        </div>
      </div>
    </NCard>

    <NCard>
      <div class="access-guide-section-title">{{ $t('custom.device_details.accessGuideRawInfo') }}</div>
      <NDescriptions :column="1">
        <NDescriptionsItem v-for="(value, key) in connectInfo" :key="key" :label="key">
          <button type="button" class="access-guide-copy" @click="emit('copy', value)">{{ value }}</button>
        </NDescriptionsItem>
      </NDescriptions>
    </NCard>
  </NScrollbar>
</template>

<style scoped src="./access-guide-shared.css"></style>

<style scoped>
.access-guide-scroll {
  max-height: 560px;
}

.access-guide-section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.access-guide-section-heading .access-guide-section-title {
  margin-bottom: 4px;
}

.access-guide-section-subtitle {
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
  line-height: 1.4;
}

.access-guide-section-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.access-guide-command-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.access-guide-command {
  min-width: 0;
}

.access-guide-command-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.access-guide-command-more {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px dashed var(--border-color);
  border-radius: 10px;
  color: var(--text-color-3);
}

@media (max-width: 720px) {
  .access-guide-section-heading {
    grid-template-columns: 1fr;
  }
}
</style>
