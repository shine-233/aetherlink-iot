<!--
文件用途：提供全局标签页右键菜单。
核心逻辑：根据当前标签和禁用 key 生成菜单项，并向父级派发关闭、刷新等动作。
关键注意事项：菜单动作会影响路由和缓存，禁用状态需要与标签页策略一致。
重构建议：可将菜单项生成和动作分派拆成可测函数。
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { VNode } from 'vue'
import { useSvgIconRender } from '@aetherlink/hooks'
import { $t } from '@/locales'
import { useTabStore } from '@/store/modules/tab'
import SvgIcon from '@/components/custom/svg-icon.vue'

defineOptions({
  name: 'ContextMenu'
})

interface Props {
  /** ClientX */
  x: number
  /** ClientY */
  y: number
  tabId: string
  excludeKeys?: App.Global.DropdownKey[]
  disabledKeys?: App.Global.DropdownKey[]
}

const props = withDefaults(defineProps<Props>(), {
  excludeKeys: () => [],
  disabledKeys: () => []
})

const visible = defineModel<boolean>('visible')

const { removeTab, clearTabs, clearLeftTabs, clearRightTabs } = useTabStore()
const { SvgIconVNode } = useSvgIconRender(SvgIcon)

type DropdownOption = {
  key: App.Global.DropdownKey
  label: string
  icon?: () => VNode
  disabled?: boolean
}

const options = computed(() => {
  const opts: DropdownOption[] = [
    {
      key: 'closeCurrent',
      label: $t('dropdown.closeCurrent'),
      icon: SvgIconVNode({ icon: 'ant-design:close-outlined', fontSize: 18 })
    },
    {
      key: 'closeOther',
      label: $t('dropdown.closeOther'),
      icon: SvgIconVNode({ icon: 'ant-design:column-width-outlined', fontSize: 18 })
    },
    {
      key: 'closeLeft',
      label: $t('dropdown.closeLeft'),
      icon: SvgIconVNode({ icon: 'mdi:format-horizontal-align-left', fontSize: 18 })
    },
    {
      key: 'closeRight',
      label: $t('dropdown.closeRight'),
      icon: SvgIconVNode({ icon: 'mdi:format-horizontal-align-right', fontSize: 18 })
    },
    {
      key: 'closeAll',
      label: $t('dropdown.closeAll'),
      icon: SvgIconVNode({ icon: 'ant-design:line-outlined', fontSize: 18 })
    }
  ]
  const { excludeKeys, disabledKeys } = props

  const result = opts.filter(opt => !excludeKeys.includes(opt.key))

  disabledKeys.forEach(key => {
    const opt = result.find(item => item.key === key)

    if (opt) {
      opt.disabled = true
    }
  })

  return result
})

function hideDropdown() {
  visible.value = false
}

const dropdownAction: Record<App.Global.DropdownKey, () => void> = {
  closeCurrent() {
    removeTab(props.tabId)
  },
  closeOther() {
    clearTabs([props.tabId])
  },
  closeLeft() {
    clearLeftTabs(props.tabId)
  },
  closeRight() {
    clearRightTabs(props.tabId)
  },
  closeAll() {
    clearTabs()
  }
}

function handleDropdown(optionKey: App.Global.DropdownKey) {
  dropdownAction[optionKey]?.()
  hideDropdown()
}
</script>

<template>
  <NDropdown
    :show="visible"
    placement="bottom-start"
    trigger="manual"
    :x="x"
    :y="y"
    :options="options"
    @clickoutside="hideDropdown"
    @select="handleDropdown"
  />
</template>

<style scoped></style>
