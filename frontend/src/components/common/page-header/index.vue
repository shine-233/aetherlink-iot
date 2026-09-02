<!--
文件用途: 全站统一的页面页头组件——标题 + 副标题 + 右侧操作区。
核心逻辑: 收敛此前各页面自绘的 .page-header/.page-title/.page-subtitle 重复实现（审计发现 4 种写法并存）。
关键注意事项: 样式引用 styles/scss/global.scss 的 --font-size-* 设计 token，禁止在页面里再手写标题字号。
-->
<script setup lang="ts">
defineOptions({ name: 'PageHeader' })

interface Props {
  /** 页面主标题（通常传 $t(...) 结果） */
  title: string
  /** 可选副标题/说明文案 */
  subtitle?: string
}

withDefaults(defineProps<Props>(), {
  subtitle: ''
})
</script>

<template>
  <div class="page-header">
    <div class="page-header__body">
      <div class="page-header__title">{{ title }}</div>
      <div v-if="subtitle" class="page-header__subtitle">{{ subtitle }}</div>
      <slot name="body" />
    </div>
    <NSpace class="page-header__actions">
      <slot />
    </NSpace>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.page-header__title {
  font-size: var(--font-size-page-title);
  font-weight: 700;
  line-height: 1.3;
}

.page-header__subtitle {
  margin-top: 4px;
  color: var(--text-color-3);
  font-size: var(--font-size-secondary);
}

@media (max-width: 640px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
