<!--
文件用途：承载 RDI 告警概览 的页面级视图。
核心逻辑：组合表格、表单、弹窗、接口请求和国际化文案，完成页面初始化、查询与交互反馈。
关键注意事项：页面通常依赖权限、分页、远端接口和路由状态，改动时需同步检查测试与接口契约。
重构建议：后续可继续拆分数据编排、列配置和弹窗流程，降低页面级组件复杂度。
-->
<script setup lang="ts">
import RdiOverview from '@/views/dashboard/rdi-overview/index.vue'
</script>

<template>
  <RdiOverview active-systems-only />
</template>
