<!--
  文件用途：提供全屏切换按钮组件。
  核心逻辑：调用浏览器全屏 API 或组合式封装，在进入和退出全屏之间切换图标状态。
  关键注意事项：全屏 API 存在浏览器差异，修改时需保留失败兜底和用户触发约束。
  重构建议：可将全屏状态监听抽为 composable，供图表、看板等模块复用。
-->
<script setup lang="ts">
import { $t } from '@/locales'

defineOptions({
  name: 'FullScreen'
})

interface Props {
  full?: boolean
}

defineProps<Props>()
</script>

<template>
  <ButtonIcon :key="String(full)" :tooltip-content="full ? $t('icon.fullscreenExit') : $t('icon.fullscreen')">
    <IconMdiFullscreenExit v-if="full" />
    <IconMdiFullscreen v-else />
  </ButtonIcon>
</template>

<style scoped></style>
