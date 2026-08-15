<!--
文件用途：实现 SimpleScrollbar 滚动容器组件。
核心逻辑：引入 simplebar-vue 及默认样式，并通过默认插槽渲染内容。
关键注意事项：组件当前只做薄封装，滚动行为主要由 simplebar-vue 决定。
重构建议：后续扩展滚动事件或主题样式时，优先保持兼容 simplebar-vue 的 props 语义。
-->
<script setup lang="ts">
import Simplebar from 'simplebar-vue'
import 'simplebar-vue/dist/simplebar.min.css'

defineOptions({
  name: 'SimpleScrollbar'
})
</script>

<template>
  <div class="h-full flex-1-hidden">
    <Simplebar class="h-full">
      <slot />
    </Simplebar>
  </div>
</template>

<style scoped></style>
