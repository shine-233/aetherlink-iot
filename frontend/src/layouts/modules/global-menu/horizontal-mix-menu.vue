<!--
文件用途：提供横向混合菜单布局。
核心逻辑：读取混合菜单上下文并在选择一级菜单后更新活跃 key。
关键注意事项：需要与 base-menu 和 route store 的选中逻辑保持一致。
重构建议：可合并横向/纵向混合菜单中的公共选择逻辑。
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRouterPush } from '@/hooks/common/router'
import { useMixMenuContext } from '../../hooks/use-mix-menu'
import FirstLevelMenu from './first-level-menu.vue'

defineOptions({
  name: 'HorizontalMixMenu'
})

const mixMenuContext = useMixMenuContext()
const activeFirstLevelMenuKey = mixMenuContext?.activeFirstLevelMenuKey || ref('')
const setActiveFirstLevelMenuKey = mixMenuContext?.setActiveFirstLevelMenuKey || (() => {})
const { routerPushByKey } = useRouterPush()

function handleSelectMixMenu(menu: App.Global.Menu) {
  setActiveFirstLevelMenuKey(menu.key)

  if (!menu.children?.length) {
    routerPushByKey(menu.routeKey)
  }
}
</script>

<template>
  <FirstLevelMenu :active-menu-key="activeFirstLevelMenuKey" @select="handleSelectMixMenu">
    <slot></slot>
  </FirstLevelMenu>
</template>

<style scoped></style>
