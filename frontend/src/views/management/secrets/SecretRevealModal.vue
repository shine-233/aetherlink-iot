<!--
文件用途：安全凭证解密查看（Reveal）弹窗。
核心逻辑：展示明文并 15 秒倒计时自动销毁；关闭时清空明文与定时器，避免内存滞留。
-->
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { NAlert, NButton, NModal, NSpace } from 'naive-ui'

const props = defineProps<{
  show: boolean
  secretKey: string
  loading: boolean
  plaintext: string
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'copy'): void
}>()

const COUNTDOWN_FROM = 15
const countdownSeconds = ref(COUNTDOWN_FROM)
let countdownTimer: number | null = null

function stopCountdown() {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
}

function startCountdown() {
  stopCountdown()
  countdownSeconds.value = COUNTDOWN_FROM
  countdownTimer = window.setInterval(() => {
    countdownSeconds.value -= 1
    if (countdownSeconds.value <= 0) emit('update:show', false)
  }, 1000)
}

watch(
  () => props.show,
  show => {
    if (show) startCountdown()
    else stopCountdown()
  }
)

onBeforeUnmount(stopCountdown)
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="安全凭证解密查看"
    style="width: 520px"
    :segmented="{ content: 'soft', footer: 'soft' }"
    @update:show="emit('update:show', $event)"
  >
    <div class="space-y-3">
      <NAlert type="warning" title="安全警告" size="small">
        本次解密查看已记录至系统安全审计日志。为防泄密，请勿截屏或共享给无关人员。 弹窗将在
        <span class="font-bold text-error">{{ countdownSeconds }}</span>
        秒后自动销毁关闭。
      </NAlert>

      <div>
        <div class="text-xs text-gray-500 mb-1">密钥标识 (Key):</div>
        <div class="font-mono text-sm font-semibold">{{ secretKey }}</div>
      </div>

      <div>
        <div class="text-xs text-gray-500 mb-1">解密明文 (Plaintext):</div>
        <div v-if="loading" class="text-xs text-gray-400 py-2">正在安全解密信封密文...</div>
        <div v-else class="p-2 bg-gray-100 dark:bg-gray-800 rounded font-mono text-sm break-all select-all">
          {{ plaintext || '（解密结果为空）' }}
        </div>
      </div>
    </div>

    <template #footer>
      <NSpace justify="space-between" align="center">
        <span class="text-xs text-gray-400">倒计时：{{ countdownSeconds }}s</span>
        <NSpace>
          <NButton type="primary" secondary @click="emit('copy')">复制明文</NButton>
          <NButton @click="emit('update:show', false)">立即关闭</NButton>
        </NSpace>
      </NSpace>
    </template>
  </NModal>
</template>
