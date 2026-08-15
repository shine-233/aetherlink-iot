<!--
文件用途：提供桌面端基础布局壳层。
核心逻辑：组合头部、侧栏、内容、页脚、页签和主题抽屉，并根据布局模式计算尺寸。
关键注意事项：该组件影响所有主框架路由，改动需要验证多种布局模式。
重构建议：可将宽度计算和模板结构进一步拆分，降低主壳层复杂度。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { AdminLayout, LAYOUT_SCROLL_EL_ID } from '@aetherlink/materials'
import type { LayoutMode } from '@aetherlink/materials'
import { useAppStore } from '@/store/modules/app'
import { useThemeStore } from '@/store/modules/theme'
import GlobalHeader from '../modules/global-header/index.vue'
import GlobalSider from '../modules/global-sider/index.vue'
import GlobalTab from '../modules/global-tab/index.vue'
import GlobalContent from '../modules/global-content/index.vue'
import GlobalFooter from '../modules/global-footer/index.vue'
import ThemeDrawer from '../modules/theme-drawer/index.vue'
import { setupMixMenuContext } from '../hooks/use-mix-menu'
defineOptions({
  name: 'BaseLayout'
})

const appStore = useAppStore()
const themeStore = useThemeStore()

const layoutMode = computed(() => {
  const vertical: LayoutMode = 'vertical'
  const horizontal: LayoutMode = 'horizontal'
  return themeStore.layout.mode.includes(vertical) ? vertical : horizontal
})

const headerPropsConfig: Record<UnionKey.ThemeLayoutMode, App.Global.HeaderProps> = {
  vertical: {
    showLogo: false,
    showMenu: false,
    showMenuToggler: true
  },
  'vertical-mix': {
    showLogo: false,
    showMenu: false,
    showMenuToggler: false
  },
  horizontal: {
    showLogo: true,
    showMenu: true,
    showMenuToggler: false
  },
  'horizontal-mix': {
    showLogo: true,
    showMenu: true,
    showMenuToggler: false
  }
}

const headerProps = computed(() => headerPropsConfig[themeStore.layout.mode])

const siderVisible = computed(() => themeStore.layout.mode !== 'horizontal')

const isVerticalMix = computed(() => themeStore.layout.mode === 'vertical-mix')

const isHorizontalMix = computed(() => themeStore.layout.mode === 'horizontal-mix')

const siderWidth = computed(() => getSiderWidth())

const siderCollapsedWidth = computed(() => getSiderCollapsedWidth())

function getSiderWidth() {
  const { width, mixWidth, mixChildMenuWidth } = themeStore.sider
  let w = isVerticalMix.value || isHorizontalMix.value ? mixWidth : width
  if (isVerticalMix.value && appStore.mixSiderFixed) {
    w += mixChildMenuWidth
  }
  return w
}

function getSiderCollapsedWidth() {
  const { collapsedWidth, mixCollapsedWidth, mixChildMenuWidth } = themeStore.sider
  let w = isVerticalMix.value || isHorizontalMix.value ? mixCollapsedWidth : collapsedWidth

  if (isVerticalMix.value && appStore.mixSiderFixed) {
    w += mixChildMenuWidth
  }
  return w
}

setupMixMenuContext()
</script>

<template>
  <!-- 移动端布局 -->
  <div v-if="appStore.isMobile" class="mobile-layout">
    <!-- 主内容区 -->
    <main :id="LAYOUT_SCROLL_EL_ID" class="mobile-main">
      <GlobalContent :show-padding="true" />
    </main>

    <ThemeDrawer />
  </div>

  <!-- 桌面端布局 -->
  <AdminLayout
    v-else
    v-model:sider-collapse="appStore.siderCollapse"
    :mode="layoutMode"
    :scroll-el-id="LAYOUT_SCROLL_EL_ID"
    :scroll-mode="themeStore.layout.scrollMode"
    :is-mobile="appStore.isMobile"
    :full-content="appStore.fullContent"
    :fixed-top="themeStore.fixedHeaderAndTab"
    :header-height="themeStore.header.height"
    :tab-visible="themeStore.tab.visible"
    :tab-height="themeStore.tab.height"
    :content-class="appStore.contentXScrollable ? 'overflow-x-hidden' : ''"
    :sider-visible="siderVisible"
    :sider-width="siderWidth"
    :sider-collapsed-width="siderCollapsedWidth"
    :footer-visible="themeStore.footer.visible"
    :fixed-footer="themeStore.footer.fixed"
    :right-footer="themeStore.footer.right"
  >
    <template #header>
      <GlobalHeader v-bind="headerProps" />
    </template>
    <template #tab>
      <GlobalTab />
    </template>
    <template #sider>
      <GlobalSider />
    </template>
    <GlobalContent />
    <ThemeDrawer />
    <template #footer>
      <GlobalFooter />
    </template>
  </AdminLayout>
</template>

<style lang="scss">
#__SCROLL_EL_ID__ {
  @include scrollbar();
}

// 移动端布局样式
.mobile-layout {
  @apply h-screen flex flex-col bg-layout;
}

.mobile-main {
  @apply flex-1 overflow-auto;

  @include scrollbar();
}
</style>
