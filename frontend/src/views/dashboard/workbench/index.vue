<!--
  文件用途：承载 frontend/src/views/dashboard/workbench/index.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<script lang="ts" setup>
import { WorkbenchHeader, WorkbenchMain } from './components'
</script>

<template>
  <NSpace :vertical="true" :size="16">
    <WorkbenchHeader />
    <WorkbenchMain />
  </NSpace>
</template>

<style scoped></style>
