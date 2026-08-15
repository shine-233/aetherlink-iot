<!--
文件用途：角色权限分配弹窗，负责把 UI 元素权限树回显给当前角色，并提交新的权限集合。
核心逻辑：先拉取全量 UI 元素树，再查询角色已有权限并合并默认首页节点，最后把勾选节点和半选节点一起提交给后端。
关键状态流：弹窗 after-enter 时初始化权限树；selectedPermissions 保存显式勾选节点；提交时额外读取树组件半选节点，拼成最终权限集。
使用注意事项：这里配置的是前端元素级权限，会直接影响菜单、按钮和页面显隐，不只是普通展示配置；改动时要同步核对角色授权与元素树接口契约。
静态审查建议：
1. 默认首页节点通过 label === '首页' 查找，依赖中文文案和节点结构，后续建议改成稳定的 element_code 常量。
2. treeOptions 仍使用 any，proxy.$refs.treeRef 也依赖实例代理，类型边界较弱，适合后续补成显式 Tree 组件类型。
3. 当前没有独立 loading、异常提示和重复提交防抖，接口较慢或失败时用户感知较弱，后续可补提交态与错误留存。
-->
<script setup lang="ts">
import { computed, getCurrentInstance, ref } from 'vue'
import type { FormInst } from 'naive-ui'
import { deleteRolePermissions, fetchUIElementList, getRolePermissions, modifyRolePermissions } from '@/service/api'
import { $t } from '@/locales'
const { proxy }: any = getCurrentInstance()
export interface Props {
  /** 弹窗可见性 */
  visible: boolean
  /** 编辑的表格行数据 */
  editData?: any | null
}

interface Element {
  id: string
  parent_id: string
  element_code: string
  element_type: number
  description: string
  children: Element[]
}

interface TreeNode {
  label: string
  key: string
  children: TreeNode[]
}

function convertToTreeNodes(elements: Element[]): TreeNode[] {
  // 统一把后端 UI 元素树转换成 n-tree 所需结构，并在这里集中定义哪些节点不可直接取消。
  return elements.map(item => ({
    label: item.description,
    key: item.id,
    disabled: item.element_code === 'home', // 禁止选中首页
    children: item.children.length > 0 ? convertToTreeNodes(item.children) : []
  }))
}

defineOptions({ name: 'EditPermissionModal' })

const props = withDefaults(defineProps<Props>(), {
  editData: null
})

interface Emits {
  (e: 'update:visible', visible: boolean): void

  /** 点击协议 */
  (e: 'success'): void
}

const emit = defineEmits<Emits>()

const modalVisible = computed({
  get() {
    return props.visible
  },
  set(visible) {
    emit('update:visible', visible)
  }
})
const closeModal = () => {
  modalVisible.value = false
}

const title = computed(() => {
  return `${$t('page.manage.role.editPermission')} - ${props.editData?.name}`
})

const formRef = ref<HTMLElement & FormInst>()

// selectedPermissions 只保存显式勾选节点；半选节点会在提交时从树组件实例中补齐。
const selectedPermissions = ref<string[]>([])
const treeOptions = ref<any>([])

const initRolePermissions = async () => {
  // 首页默认选中
  const data = treeOptions.value.find(item => item.label === '首页')
  if (props.editData) {
    // 角色已有权限与首页节点做去重合并，保证首页不可被误删。
    const permissions = await getRolePermissions(props.editData.id)
    selectedPermissions.value = [...new Set([data.key, ...permissions])]
  } else {
    selectedPermissions.value = [data.key]
  }
}

const initUIElementList = async () => {
  // 权限树必须先有全量元素结构，才能再做角色权限回显。
  const uiElementList = await fetchUIElementList()
  treeOptions.value = convertToTreeNodes(uiElementList)
  initRolePermissions()
}

async function handleSubmit() {
  let data: any
  // n-tree 的级联关闭后，半选节点不会自动进 selectedPermissions，需要手动合并保证父层授权不丢失。
  const indeterminateData = proxy.$refs.treeRef.getIndeterminateData().keys
  const currentPermissions = [...selectedPermissions.value, ...indeterminateData]
  selectedPermissions.value = []
  if (currentPermissions.length === 0) {
    // 角色一个权限都不保留时走清空接口，而不是提交空数组更新。
    data = await deleteRolePermissions(props.editData?.id)
  } else {
    data = await modifyRolePermissions(props.editData?.id, currentPermissions)
  }
  closeModal()
  if (!data.error) {
    emit('success')
  }
}
</script>

<template>
  <n-modal
    v-model:show="modalVisible"
    preset="card"
    :title="title"
    :on-after-enter="
      () => {
        // 每次进入弹窗都重新取元素树与角色授权，避免使用上一次角色的残留勾选状态。
        initUIElementList()
      }
    "
  >
    <n-form ref="formRef" label-placement="left" :label-width="80">
      <div class="h-300px overflow-y-auto">
        <n-tree
          ref="treeRef"
          v-model:checked-keys="selectedPermissions"
          :data="treeOptions"
          :cascade="false"
          checkable
          block-line
        />
      </div>
      <n-space class="w-full pt-16px" :size="24" justify="end">
        <n-button class="w-72px" @click="closeModal">{{ $t('generate.cancel') }}</n-button>
        <n-button class="w-72px" type="primary" @click="handleSubmit">{{ $t('page.login.common.confirm') }}</n-button>
      </n-space>
    </n-form>
  </n-modal>
</template>

<style scoped></style>
