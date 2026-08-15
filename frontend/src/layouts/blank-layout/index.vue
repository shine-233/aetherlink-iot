<!--
文件用途：提供空白路由布局。
核心逻辑：仅渲染当前路由出口，不附加主导航和主题抽屉。
关键注意事项：适用于登录或独立页面，不应加入主框架副作用。
重构建议：如需要公共能力，应考虑新建专用布局而不是膨胀空白布局。
-->
<script setup lang="ts">
import GlobalContent from '../modules/global-content/index.vue'

defineOptions({
  name: 'BlankLayout'
})
</script>

<template>
  <GlobalContent :show-padding="false" />
</template>

<style scoped></style>
