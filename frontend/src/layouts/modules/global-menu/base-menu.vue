<!--
文件用途：提供全局基础菜单组件。
核心逻辑：封装 Naive UI 菜单的选中项、展开项、折叠状态和路由跳转。
关键注意事项：菜单 key、权限路由和布局模式高度耦合，改动需验证各模式导航。
重构建议：建议拆出 selected/expanded key 计算，降低组件复杂度。
-->
<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NTooltip } from 'naive-ui'
import type { MentionOption, MenuProps } from 'naive-ui'
import { SimpleScrollbar } from '@aetherlink/materials'
import type { RouteKey } from '@elegant-router/types'
import { useAppStore } from '@/store/modules/app'
import { useThemeStore } from '@/store/modules/theme'
import { useRouteStore } from '@/store/modules/route'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'

defineOptions({
  name: 'BaseMenu'
})

interface Props {
  darkTheme?: boolean
  mode?: MenuProps['mode']
  menus: App.Global.Menu[]
}

const props = withDefaults(defineProps<Props>(), {
  mode: 'vertical'
})

const route = useRoute()
const appStore = useAppStore()
const themeStore = useThemeStore()
const routeStore = useRouteStore()
const { routerPushByKey } = useRouterPush()

const naiveMenus = computed(() => props.menus as unknown as MentionOption[])

const isHorizontal = computed(() => props.mode === 'horizontal')

const siderCollapse = computed(() => themeStore.layout.mode === 'vertical' && appStore.siderCollapse)

const headerHeight = computed(() => `${themeStore.header.height}px`)

const selectedKey = computed(() => {
  const { hideInMenu, activeMenu } = route.meta
  const name = route.name as string

  const routeName = (hideInMenu ? activeMenu : name) || name

  return routeName
})

const expandedKeys = ref<string[]>([])

function updateExpandedKeys() {
  if (isHorizontal.value || siderCollapse.value || !selectedKey.value) {
    expandedKeys.value = []
    return
  }
  if (selectedKey.value) {
    const newKeys = routeStore.getSelectedMenuKeyPath(selectedKey.value)
    if (newKeys && newKeys.length > 0) {
      expandedKeys.value = newKeys
    }
  }
}

function handleClickMenu(key: RouteKey) {
  // const { query } = routeStore.getSelectedMenuMetaByKey(key) || {};
  routerPushByKey(key)
}

const renderMenuLabel = op => {
  return op.label as string
}

watch(
  () => route.name,
  () => {
    updateExpandedKeys()
  },
  { immediate: true }
)
</script>

<template>
  <SimpleScrollbar>
    <NMenu
      v-model:expanded-keys="expandedKeys"
      :mode="mode"
      :value="selectedKey"
      :collapsed="siderCollapse"
      :collapsed-width="themeStore.sider.collapsedWidth"
      :collapsed-icon-size="22"
      :options="naiveMenus"
      :render-label="renderMenuLabel"
      :inverted="darkTheme"
      :indent="18"
      responsive
      @update:value="handleClickMenu"
    />
  </SimpleScrollbar>
</template>

<style scoped>
:deep(.n-menu--horizontal) {
  --n-item-height: v-bind(headerHeight) !important;
}
</style>
