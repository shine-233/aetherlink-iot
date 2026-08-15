<!--
  文件用途：根据 JSON 配置动态生成表单界面和类型契约。
  核心逻辑：解析字段描述、渲染对应输入控件，并通过声明文件约束外部调用方式。
  关键注意事项：动态字段需要兼容旧配置，默认值和校验规则变化会影响已保存表单。
  重构建议：可把 schema 解析、控件映射和表单状态拆分为独立可测试模块。
-->
<script setup lang="tsx"></script>

<template>
  <div>JSON to Form</div>
</template>

<style scoped></style>
