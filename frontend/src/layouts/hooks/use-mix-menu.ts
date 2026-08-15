/*
 * 文件用途：提供混合菜单布局状态 Hook。
 * 核心逻辑：维护当前一级菜单 key，并从 route store 派生对应子菜单集合。
 * 关键注意事项：路由命名规则和菜单树结构变化会影响混合菜单选中态。
 * 重构建议：建议抽离 route name 到一级菜单 key 的解析函数并补测试。
 */
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useContext } from '@aetherlink/hooks'
import { useRouteStore } from '@/store/modules/route'

export function useMixMenu() {
  const route = useRoute()
  const routeStore = useRouteStore()

  const activeFirstLevelMenuKey = ref('')

  function setActiveFirstLevelMenuKey(key: string) {
    activeFirstLevelMenuKey.value = key
  }

  function getActiveFirstLevelMenuKey() {
    const { hideInMenu, activeMenu } = route.meta
    const name = route.name as string

    const routeName = (hideInMenu ? activeMenu : name) || name

    // 确保 routeName 存在且为字符串，否则使用默认值
    const safeRouteName = routeName || 'home'
    const [firstLevelRouteName] = safeRouteName.split('_')

    setActiveFirstLevelMenuKey(firstLevelRouteName)
  }

  const menus = computed(
    () => routeStore.menus.find(menu => menu.key === activeFirstLevelMenuKey.value)?.children || []
  )

  watch(
    () => route.name,
    () => {
      getActiveFirstLevelMenuKey()
    },
    { immediate: true }
  )

  return {
    activeFirstLevelMenuKey,
    setActiveFirstLevelMenuKey,
    getActiveFirstLevelMenuKey,
    menus
  }
}

export const { setupStore: setupMixMenuContext, useStore: useMixMenuContext } = useContext('mix-menu', useMixMenu)
