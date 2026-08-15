<!--
  文件用途: 设备详情自动化关联面板。
  核心逻辑: 展示与当前设备相关的自动化规则或触发配置。
  关键注意事项: 规则启停和设备绑定会影响自动执行，需确认权限和副作用提示。
  重构建议: 复用自动化列表数据映射，并补规则启停与空绑定测试。
-->
<script setup lang="ts">
import sceneLinkage from '@/views/automation/scene-linkage/modules/dataList.vue'

defineProps<{
  id: string
}>()
</script>

<template>
  <sceneLinkage :device_id="id" back-type="device" />
</template>

<style scoped></style>
