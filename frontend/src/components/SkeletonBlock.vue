<!--
  文件用途：通用骨架屏组件，替代 n-spin 转圈提升首屏感知速度。
  核心逻辑：提供表格/卡片/文本三种预设形状的骨架屏布局。
-->
<script setup lang="ts">
defineOptions({ name: 'SkeletonBlock' })

interface Props {
  type?: 'table' | 'cards' | 'text'
  count?: number
}

withDefaults(defineProps<Props>(), {
  type: 'table',
  count: 5
})
</script>

<template>
  <div v-if="type === 'table'" class="skeleton-table">
    <div class="skeleton-shimmer skeleton-header" />
    <div v-for="i in count" :key="i" class="skeleton-shimmer skeleton-row" />
  </div>
  <div v-else-if="type === 'cards'" class="skeleton-cards">
    <div v-for="i in count" :key="i" class="skeleton-shimmer skeleton-card" />
  </div>
  <div v-else class="skeleton-text">
    <div class="skeleton-shimmer skeleton-line" style="width: 40%" />
    <div v-for="i in count" :key="i" class="skeleton-shimmer skeleton-line" style="width: 100%" />
  </div>
</template>

<style scoped>
.skeleton-table { display: flex; flex-direction: column; gap: 8px; padding: 12px 0; }
.skeleton-header { height: 36px; border-radius: 4px; opacity: 0.7; }
.skeleton-row { height: 44px; border-radius: 4px; }
.skeleton-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 16px; }
.skeleton-card { height: 120px; border-radius: 8px; }
.skeleton-text { display: flex; flex-direction: column; gap: 10px; padding: 8px 0; }
.skeleton-line { height: 16px; border-radius: 4px; }
</style>
