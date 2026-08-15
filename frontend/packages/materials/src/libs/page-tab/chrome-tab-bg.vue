<!--
  文件用途：Chrome 风格页签的 SVG 背景形状组件。
  核心逻辑：通过左右镜像的 symbol/use 组合生成可伸缩的标签页背景。
  主要逻辑：左半部分使用基础路径，右半部分通过 scale(-1, 1) 镜像复用同一几何形状。
  关键注意事项：修改 viewBox 或路径时要检查 ChromeTab 的高度、圆角和相邻页签拼接效果。
  重构建议：建议将 SVG 几何参数与样式常量集中记录，降低后续微调成本。
-->
<script setup lang="ts">
defineOptions({
  name: 'ChromeTabBg'
})
</script>

<template>
  <svg class="wh-full">
    <defs>
      <symbol id="geometry-left" viewBox="0 0 214 36">
        <path d="M17 0h197v36H0v-2c4.5 0 9-3.5 9-8V8c0-4.5 3.5-8 8-8z"></path>
      </symbol>
      <symbol id="geometry-right" viewBox="0 0 214 36">
        <use xlink:href="#geometry-left"></use>
      </symbol>
      <clipPath>
        <rect width="100%" height="100%" x="0"></rect>
      </clipPath>
    </defs>
    <svg width="51%" height="100%">
      <use xlink:href="#geometry-left" width="214" height="36" fill="currentColor"></use>
    </svg>
    <g transform="scale(-1, 1)">
      <svg width="51%" height="100%" x="-100%" y="0">
        <use xlink:href="#geometry-right" width="214" height="36" fill="currentColor"></use>
      </svg>
    </g>
  </svg>
</template>

<style scoped></style>
