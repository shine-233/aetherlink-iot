<!--
文件用途：提供头部主题设置按钮。
核心逻辑：触发主题抽屉或主题相关入口展示。
关键注意事项：按钮行为影响全局主题配置入口，需保持与 theme drawer 状态一致。
重构建议：可通过 emits 或 store action 明确打开抽屉的边界。
-->
<script setup lang="ts">
import { useAppStore } from '@/store/modules/app'
import { $t } from '@/locales'

defineOptions({
  name: 'ThemeButton'
})

const appStore = useAppStore()
</script>

<template>
  <ButtonIcon
    icon="majesticons:color-swatch-line"
    :tooltip-content="$t('icon.themeConfig')"
    @click="appStore.openThemeDrawer"
  />
</template>

<style scoped></style>
