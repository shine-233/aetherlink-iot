<!--
文件用途：渲染全局侧栏容器。
核心逻辑：根据布局模式、主题反色和折叠状态展示 Logo 与菜单。
关键注意事项：侧栏是主导航核心区域，改动需验证多布局模式与暗色主题。
重构建议：可进一步明确普通侧栏与混合侧栏的职责边界。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '@/store/modules/app'
import { useThemeStore } from '@/store/modules/theme'
import { useRouteStore } from '@/store/modules/route'
import GlobalLogo from '../global-logo/index.vue'
import VerticalMenu from '../global-menu/base-menu.vue'
import VerticalMixMenu from '../global-menu/vertical-mix-menu.vue'
import HorizontalMixMenu from '../global-menu/horizontal-mix-menu.vue'

defineOptions({
  name: 'GlobalSider'
})

const appStore = useAppStore()
const themeStore = useThemeStore()
const routeStore = useRouteStore()

const isVerticalMix = computed(() => themeStore.layout.mode === 'vertical-mix')
const isHorizontalMix = computed(() => themeStore.layout.mode === 'horizontal-mix')
const darkMenu = computed(() => !themeStore.darkMode && !isHorizontalMix.value && themeStore.sider.inverted)
const showLogo = computed(() => !isVerticalMix.value && !isHorizontalMix.value)
</script>

<template>
  <DarkModeContainer class="wh-full flex-vertical-stretch shadow-sider" :inverted="darkMenu">
    <GlobalLogo
      v-if="showLogo"
      :show-title="!appStore.siderCollapse"
      :style="{ height: themeStore.header.height + 'px' }"
    />
    <VerticalMixMenu v-if="isVerticalMix">
      <GlobalLogo :show-title="false" :style="{ height: themeStore.header.height + 'px' }" />
    </VerticalMixMenu>
    <HorizontalMixMenu v-else-if="isHorizontalMix" />
    <VerticalMenu v-else :dark-theme="darkMenu" :menus="routeStore.menus" />
  </DarkModeContainer>
</template>

<style scoped></style>
