<!--
  文件用途：提供暗色模式适配容器。
  核心逻辑：根据当前主题状态切换容器样式，让子内容继承暗色背景和前景色。
  关键注意事项：样式变量需与全局主题保持一致，避免局部暗色容器和 Naive UI 主题冲突。
  重构建议：可将主题 class 计算收敛到统一主题工具，减少各组件重复判断。
-->
<script setup lang="ts">
defineOptions({ name: 'DarkModeContainer' })

interface Props {
  inverted?: boolean
}

defineProps<Props>()
</script>

<template>
  <div class="bg-container text-base_text transition-300" :class="{ 'bg-inverted text-#1f1f1f': inverted }">
    <slot></slot>
  </div>
</template>

<style scoped></style>
