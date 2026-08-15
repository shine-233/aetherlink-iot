<!--
文件用途：提供主题抽屉设置项容器。
核心逻辑：统一标题、说明和右侧控件布局。
关键注意事项：应保持轻量展示职责，避免加入具体主题修改逻辑。
重构建议：可按需要扩展插槽而不是写死控件类型。
-->
<script setup lang="ts">
defineOptions({
  name: 'SettingItem'
})

interface Props {
  /** Label */
  label: string
}

defineProps<Props>()
</script>

<template>
  <div class="w-full flex-y-center justify-between">
    <div>
      <span class="pr-8px text-base_text">{{ label }}</span>
      <slot name="suffix"></slot>
    </div>
    <slot></slot>
  </div>
</template>

<style scoped></style>
