<!--
文件用途：管理全局标签页导航。
核心逻辑：处理标签滚动定位、右键菜单、刷新、关闭、批量关闭和路由切换。
关键注意事项：标签页与页面缓存、路由状态紧密相关，改动风险较高。
重构建议：建议继续拆分滚动计算、关闭策略和下拉菜单状态。
-->
<script setup lang="ts">
import { nextTick, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useElementBounding } from '@vueuse/core'
import { PageTab } from '@aetherlink/materials'
import { useAppStore } from '@/store/modules/app'
import { useThemeStore } from '@/store/modules/theme'
import { useTabStore } from '@/store/modules/tab'
import ContextMenu from './context-menu.vue'

defineOptions({
  name: 'GlobalTab'
})

const route = useRoute()
const appStore = useAppStore()
const themeStore = useThemeStore()
const tabStore = useTabStore()

const bsWrapper = ref<HTMLElement>()
const { width: bsWrapperWidth, left: bsWrapperLeft } = useElementBounding(bsWrapper)
const tabRef = ref<HTMLElement>()

const TAB_DATA_ID = 'data-tab-id'

type TabNamedNodeMap = NamedNodeMap & {
  [TAB_DATA_ID]: Attr
}

async function scrollToActiveTab() {
  await nextTick()
  if (!tabRef.value) return

  const { children } = tabRef.value

  for (let i = 0; i < children.length; i += 1) {
    const child = children[i]

    const { value: tabId } = (child.attributes as TabNamedNodeMap)[TAB_DATA_ID]

    if (tabId === tabStore.activeTabId) {
      const { left, width } = child.getBoundingClientRect()
      const clientX = left + width / 2

      setTimeout(() => {
        scrollByClientX(clientX)
      }, 50)

      break
    }
  }
}

function scrollByClientX(clientX: number) {
  const wrapper = bsWrapper.value
  if (!wrapper) return

  const currentX = clientX - bsWrapperLeft.value
  const deltaX = currentX - bsWrapperWidth.value / 2
  const targetLeft = Math.max(0, Math.min(wrapper.scrollLeft + deltaX, wrapper.scrollWidth - wrapper.clientWidth))
  wrapper.scrollTo({ left: targetLeft, behavior: 'smooth' })
}

function getContextMenuDisabledKeys(tabId: string) {
  const disabledKeys: App.Global.DropdownKey[] = []

  if (tabStore.isTabRetain(tabId)) {
    disabledKeys.push('closeCurrent')
  }

  return disabledKeys
}

async function handleCloseTab(tab: App.Global.Tab) {
  const currentIndex = tabStore.tabs.findIndex(t => t.id === tab.id)
  const nextTab = tabStore.tabs[currentIndex + 1] || tabStore.tabs[currentIndex - 1]

  await tabStore.removeTab(tab.id)

  if (nextTab) {
    await tabStore.switchRouteByTab(nextTab)
  }
}

async function refresh() {
  appStore.reloadPage(500)
}

interface DropdownConfig {
  visible: boolean
  x: number
  y: number
  tabId: string
}

const dropdown: DropdownConfig = reactive({
  visible: false,
  x: 0,
  y: 0,
  tabId: ''
})

function setDropdown(config: Partial<DropdownConfig>) {
  Object.assign(dropdown, config)
}

let isClickContextMenu = false

function handleDropdownVisible(visible?: boolean) {
  if (!isClickContextMenu) {
    setDropdown({ visible: Boolean(visible) })
  }
}

async function handleContextMenu(e: MouseEvent, tabId: string) {
  e.preventDefault()

  const { clientX, clientY } = e

  isClickContextMenu = true

  const DURATION = dropdown.visible ? 150 : 0

  setDropdown({ visible: false })

  setTimeout(() => {
    setDropdown({
      visible: true,
      x: clientX,
      y: clientY,
      tabId
    })
    isClickContextMenu = false
  }, DURATION)
}

function init() {
  tabStore.initTabStore(route)
}

// watch
watch(
  () => route.fullPath,
  () => {
    tabStore.addTab(route)
  }
)
watch(
  () => tabStore.activeTabId,
  () => {
    scrollToActiveTab()
  }
)

// init
init()
</script>

<template>
  <DarkModeContainer class="wh-full flex-y-center px-16px shadow-tab">
    <div ref="bsWrapper" class="global-tab-scroll h-full flex-1-hidden">
      <div
        ref="tabRef"
        class="h-full flex pr-18px"
        :class="[themeStore.tab.mode === 'chrome' ? 'items-end' : 'items-center gap-12px']"
      >
        <PageTab
          v-for="tab in tabStore.tabs"
          :key="tab.id"
          :[TAB_DATA_ID]="tab.id"
          :mode="themeStore.tab.mode"
          :dark-mode="themeStore.darkMode"
          :active="tab.id === tabStore.activeTabId"
          :active-color="themeStore.themeColor"
          :closable="!tabStore.isTabRetain(tab.id)"
          @click="tabStore.switchRouteByTab(tab)"
          @close="handleCloseTab(tab)"
          @contextmenu="handleContextMenu($event, tab.id)"
        >
          <template #prefix>
            <SvgIcon :icon="tab.icon" :local-icon="tab.localIcon" class="inline-block align-text-bottom text-16px" />
          </template>
          <span>{{ tab.label }}</span>
        </PageTab>
      </div>
    </div>
    <ReloadButton :loading="!appStore.reloadFlag" @click="refresh" />
    <!--    <FullScreen :full="appStore.fullContent" @click="appStore.toggleFullContent" />-->
  </DarkModeContainer>
  <ContextMenu
    :visible="dropdown.visible"
    :tab-id="dropdown.tabId"
    :disabled-keys="getContextMenuDisabledKeys(dropdown.tabId)"
    :x="dropdown.x"
    :y="dropdown.y"
    @update:visible="handleDropdownVisible"
  ></ContextMenu>
</template>

<style scoped>
.global-tab-scroll {
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
  scroll-behavior: smooth;
  touch-action: pan-x;
}

.global-tab-scroll::-webkit-scrollbar {
  display: none;
}
</style>
