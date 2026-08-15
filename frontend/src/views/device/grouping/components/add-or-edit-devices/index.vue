<!--
文件用途: 承载新增或编辑设备相关的设备页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script lang="ts" setup>
import { computed, getCurrentInstance, onMounted, ref, watch } from 'vue'
import type { FormInst, FormRules } from 'naive-ui'
// import {useMessage} from 'naive-ui';
import { deviceGroup, deviceGroupTree, putDeviceGroup } from '@/service/api/device'
import { $t } from '@/locales'

interface Group {
  id: string
  parent_id: string
  tier: number
  name: string
  description: string | null
  created_at: string
  updated_at: string
  remark: string | null
  tenant_id: string
}

interface TreeNode {
  group: Group
  children?: TreeNode[] | undefined // TreeNode类型的可选数组，用于描述子节点
}

const showModal = ref<boolean>(false)
defineExpose({ showModal })

// Props received from parent component
interface Props {
  isPidNoEdit?: boolean
  isEdit?: boolean
  editData?: {
    id: string
    parent_id: string
    name: string
    description: string
  }
  refreshData: () => void
}

const props = defineProps<Props>()
// const message = useMessage();
const formRef = ref<HTMLElement & FormInst>()
const formItem = ref({
  id: '', // Used for identification in edit mode
  parent_id: '',
  name: '',
  description: ''
})

// Tree select options
const options = ref()

// Form validation rules
const rules: FormRules = {
  parent_id: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('custom.groupPage.selectParentGroup')
  },
  Name: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('custom.groupPage.enterGroupName')
  }
}

interface opNode {
  id?: string
  name?: string
  children?: opNode[] // TreeNode类型的可选数组，用于描述子节点
}

// Extract id and name for tree select options
const normalizeTreeNodes = (value: unknown): TreeNode[] => {
  if (Array.isArray(value)) return value as TreeNode[]
  if (!value || typeof value !== 'object') return []

  const record = value as { data?: unknown; list?: unknown }
  if (Array.isArray(record.data)) return record.data as TreeNode[]
  if (Array.isArray(record.list)) return record.list as TreeNode[]
  return []
}

const extractIdAndName = (data: TreeNode[]): opNode[] => {
  const res = normalizeTreeNodes(data).map((node) => ({
    id: node.group.id,
    name: node.group.name,
    children: node.children ? extractIdAndName(node.children) : undefined
  }))
  return res
}
// Fetch options for tree select and handle edit mode data echo back
const getOptions = async () => {
  if (props.editData) {
    formItem.value = { ...props.editData }
  }

  const { data } = await deviceGroupTree({})
  const treeNodes = normalizeTreeNodes(data)
  options.value = [
    {
      id: '0', // Root node for tree select
      name: $t('custom.groupPage.group'),
      children: treeNodes.map((item) => ({
        id: item.group.id,
        name: item.group.name,
        children: item.children ? extractIdAndName(item.children) : undefined
      }))
    }
  ]
}

// Submit form data
const handleSubmit = async () => {
  await formRef?.value?.validate()
  showModal.value = false
  if (props.isEdit) {
    await putDeviceGroup(formItem.value)
    // message.success($t('custom.groupPage.modificationSuccess'));
  } else {
    await deviceGroup(formItem.value)
    // message.success($t('custom.groupPage.additionSuccess'));
  }

  await getOptions()
  props.refreshData()

  // eslint-disable-next-line require-atomic-updates
  if (formItem?.value) {
    formItem.value = {
      id: '',
      parent_id: '',
      name: '',
      description: ''
    }
  }

  // Implement API call for form submission here
}

// Close modal and reset form fields
const handleClose = () => {
  showModal.value = false
  formItem.value = {
    id: '',
    parent_id: '',
    name: '',
    description: ''
  }
}

onMounted(getOptions)

// Watch for editData changes to handle edit mode data echo back
watch(
  () => props.editData,
  (newVal) => {
    if (props.isEdit && newVal) {
      formItem.value = { ...newVal }
    }
  },
  { deep: true, immediate: true }
)
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
// Expose showModal for parent component
</script>

<template>
  <!-- Modal component to display a form with tree selection, input field, and textarea -->
  <NModal v-model:show="showModal" @after-enter="getOptions">
    <NCard
      :bordered="false"
      :title="!props.isEdit ? $t('custom.groupPage.addGroup') : $t('custom.groupPage.editGroup')"
      size="huge"
      :class="getPlatform ? 'w-90%' : 'w-600px'"
    >
      <NForm ref="formRef" :model="formItem" :rules="rules" label-placement="left" label-width="auto">
        <!-- Parent group selection using tree select component -->
        <NFormItem :rules="[rules.parent_id]" :label="$t('generate.parent-group')" path="parent_id">
          <NTreeSelect
            v-model:value="formItem.parent_id"
            :disabled="props.isPidNoEdit"
            :options="options"
            default-expand-all
            key-field="id"
            label-field="name"
          ></NTreeSelect>
        </NFormItem>
        <!-- Group name input field -->
        <NFormItem :rules="[rules.name]" :label="$t('generate.group-name')" path="name">
          <NInput v-model:value="formItem.name" />
        </NFormItem>
        <!-- Description textarea for optional input -->
        <NFormItem :label="$t('custom.groupPage.description')" path="description">
          <NInput v-model:value="formItem.description" type="textarea" />
        </NFormItem>
        <!-- Form action buttons -->
        <div style="display: flex; justify-content: flex-end; gap: 8px">
          <NButton @click="handleClose">{{ $t('custom.groupPage.cancel') }}</NButton>
          <NButton type="primary" @click="handleSubmit">{{ $t('custom.groupPage.confirm') }}</NButton>
        </div>
      </NForm>
    </NCard>
  </NModal>
</template>
