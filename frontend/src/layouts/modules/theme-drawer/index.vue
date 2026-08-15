<!--
文件用途：渲染全局主题配置抽屉。
核心逻辑：组合主题颜色、暗色模式、布局模式、页面功能和配置操作模块。
关键注意事项：抽屉会修改全局 theme store，需保持模块顺序和配置持久化稳定。
重构建议：可将配置分组和模块注册配置化，方便裁剪。
-->
<script setup lang="ts">
import { useAppStore } from '@/store/modules/app'
import { $t } from '@/locales'
import DarkMode from './modules/dark-mode.vue'
import LayoutMode from './modules/layout-mode.vue'
import ThemeColor from './modules/theme-color.vue'
import PageFun from './modules/page-fun.vue'
import ConfigOperation from './modules/config-operation.vue'

defineOptions({
  name: 'ThemeDrawer'
})

const appStore = useAppStore()
</script>

<template>
  <NDrawer v-model:show="appStore.themeDrawerVisible" display-directive="show" :width="360">
    <NDrawerContent :title="$t('theme.themeDrawerTitle')" :native-scrollbar="false" closable>
      <DarkMode />
      <LayoutMode />
      <ThemeColor />
      <PageFun />
      <template #footer>
        <ConfigOperation />
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped></style>
