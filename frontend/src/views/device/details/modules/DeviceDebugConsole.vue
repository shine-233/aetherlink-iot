<!--
  文件用途：设备调试控制台（纯展示）：调试模式开关、复制支持摘要、诊断时间线、类终端日志窗口。
  数据来源：父组件通过 `useDeviceDebugConsole` 注入；本组件只渲染并发出 `update:logEnabled` / `copy-summary`。
  交互细节：日志窗口只在用户停留在底部时自动滚到底，向上翻看历史时不会被 3 秒轮询拽回底部。
  使用注意：调试模式会持续记录设备通信报文，存在额外存储、隐私与性能成本，不应长期默认开启。
-->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { HelpCircleOutline } from '@vicons/ionicons5'

import { $t } from '@/locales'
import type { DiagnosticTimelineItem } from './useDeviceDiagnosticsStats'

const props = defineProps<{
  logEnabled: boolean
  logSwitching?: boolean
  debugLogs: string[]
  timeline: DiagnosticTimelineItem[]
  nextSteps: string[]
  logsError?: unknown
  statusError?: unknown
}>()

const emit = defineEmits<{
  'update:logEnabled': [value: boolean]
  'copy-summary': []
}>()

// 距底部小于该像素视为“贴底”，新日志到达时继续自动滚动。
const STICK_TO_BOTTOM_THRESHOLD_PX = 24

const logContainerRef = ref<HTMLElement | null>(null)
const stickToBottom = ref(true)

const handleLogScroll = () => {
  const el = logContainerRef.value
  if (!el) return
  stickToBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight <= STICK_TO_BOTTOM_THRESHOLD_PX
}

watch(
  () => props.debugLogs,
  async () => {
    if (!stickToBottom.value) return
    await nextTick()
    const el = logContainerRef.value
    if (el) el.scrollTop = el.scrollHeight
  },
  { flush: 'post' }
)

const describeError = (err: unknown) => {
  if (!err) return ''
  if (typeof err === 'string') return err
  const message = (err as { message?: unknown }).message
  return typeof message === 'string' ? message : ''
}

const consoleErrorText = computed(() => {
  const err = props.statusError || props.logsError
  if (!err) return ''
  const detail = describeError(err)
  return detail ? `${$t('common.loadFailed')}: ${detail}` : $t('common.loadFailed')
})
</script>

<template>
  <div class="mt-4">
    <div class="flex items-center justify-between mb-4">
      <div class="text-18px">{{ $t('custom.device_details.diagnosisDebugLog') }}</div>
      <div class="flex items-center gap-2">
        <NButton size="small" secondary type="primary" data-testid="copy-summary" @click="emit('copy-summary')">
          {{ $t('custom.device_details.diagnosisCopySummary') }}
        </NButton>
        <NTooltip trigger="hover">
          <template #trigger>
            <div class="flex items-center gap-1 cursor-help">
              <span>{{ $t('custom.device_details.diagnosisDebugMode') }}</span>
              <NIcon size="14" class="text-gray-400">
                <HelpCircleOutline />
              </NIcon>
            </div>
          </template>
          {{ $t('custom.device_details.diagnosisDebugModeHint') }}
        </NTooltip>
        <NSwitch
          :value="logEnabled"
          :loading="logSwitching"
          :aria-label="$t('custom.device_details.diagnosisDebugMode')"
          @update:value="(value: boolean) => emit('update:logEnabled', value)"
        />
      </div>
    </div>

    <NAlert v-if="consoleErrorText" type="warning" class="mb-3" data-testid="debug-console-error">
      {{ consoleErrorText }}
    </NAlert>

    <NCard class="mb-4" size="small" :title="$t('custom.device_details.diagnosisTimelineTitle')">
      <NEmpty v-if="timeline.length === 0" :description="$t('custom.device_details.diagnosticEvidenceEmpty')">
        <template #extra>
          <div class="text-left text-13px leading-6 text-gray-500">
            <div v-for="step in nextSteps" :key="step">- {{ step }}</div>
          </div>
        </template>
      </NEmpty>
      <NTimeline v-else>
        <NTimelineItem
          v-for="(item, index) in timeline"
          :key="`${item.title}-${index}`"
          :type="item.type"
          :time="item.time"
          :title="item.title"
        >
          <div class="whitespace-pre-wrap break-all text-12px">{{ item.detail }}</div>
          <div class="mt-1 text-12px text-gray-500">
            {{ $t('custom.device_details.diagnosisNextStep') }}{{ item.nextAction }}
          </div>
        </NTimelineItem>
      </NTimeline>
    </NCard>

    <div
      ref="logContainerRef"
      class="bg-[#1e1e1e] text-[#d4d4d4] font-mono p-4 rounded h-[400px] overflow-auto whitespace-pre-wrap break-all text-xs"
      role="log"
      aria-live="off"
      tabindex="0"
      data-testid="debug-log-container"
      @scroll.passive="handleLogScroll"
    >
      <div v-if="debugLogs.length === 0" class="text-center text-gray-500 py-10">
        {{ $t('custom.device_details.diagnosisNoLogs') }}
      </div>
      <div
        v-for="(log, index) in debugLogs"
        :key="index"
        class="mb-1 border-b border-gray-700/50 pb-1 last:border-0 hover:bg-[#2a2d2e]"
      >
        {{ log }}
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
:deep(.n-card-header) {
  font-size: 16px;
}
</style>
