<!--
  文件用途：提供页面或局部内容刷新按钮。
  核心逻辑：触发刷新事件或路由刷新流程，并在界面上展示刷新图标状态。
  关键注意事项：刷新可能影响缓存、路由 keep-alive 和未保存表单，调用方需明确刷新边界。
  重构建议：可将刷新调度与按钮展示拆开，方便在列表、看板和详情页复用。
-->
<script setup lang="ts">
import { $t } from '@/locales'

defineOptions({
  name: 'ReloadButton'
})

interface Props {
  loading?: boolean
}

defineProps<Props>()
</script>

<template>
  <ButtonIcon :tooltip-content="$t('icon.reload')">
    <IconAntDesignReloadOutlined :class="{ 'animate-spin animate-duration-750': loading }" />
  </ButtonIcon>
</template>

<style scoped></style>
