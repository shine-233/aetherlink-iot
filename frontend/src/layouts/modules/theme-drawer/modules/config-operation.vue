<!--
文件用途：提供主题配置复制、重置和导出操作。
核心逻辑：读取当前主题配置并生成剪贴板文本，同时支持恢复默认设置。
关键注意事项：复制和重置是全局配置操作，需避免误覆盖用户设置。
重构建议：可把配置序列化和剪贴板交互拆成独立工具。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'
import { useThemeStore } from '@/store/modules/theme'
import { writeClipboardText } from '@/utils/clipboard'

defineOptions({
  name: 'ConfigOperation'
})

const themeStore = useThemeStore()

async function handleCopyConfig() {
  if (await writeClipboardText(dataClipboardText.value)) {
    window.$message?.success($t('theme.configOperation.copySuccessMsg'))
  } else {
    window.$message?.error($t('common.copyFailed'))
  }
}

function getClipboardText() {
  const reg = /"\w+":/g

  const json = themeStore.settingsJson

  return json.replace(reg, (match) => match.replace(/"/g, ''))
}

function handleReset() {
  themeStore.resetStore()

  setTimeout(() => {
    window.$message?.success($t('theme.configOperation.resetSuccessMsg'))
  }, 50)
}

const dataClipboardText = computed(() => getClipboardText())
</script>

<template>
  <div class="w-full flex justify-between">
    <NButton type="error" ghost @click="handleReset">{{ $t('theme.configOperation.resetConfig') }}</NButton>
    <NButton type="primary" @click="handleCopyConfig">{{ $t('theme.configOperation.copyConfig') }}</NButton>
  </div>
</template>

<style scoped></style>
