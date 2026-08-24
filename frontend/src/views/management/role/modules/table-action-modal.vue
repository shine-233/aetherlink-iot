<!--
文件用途：角色新增/编辑弹窗，负责承接角色基础资料表单并向父页面回传成功事件。
核心逻辑：根据 props.type 在“新增空表单”和“编辑现有角色”之间切换，同一套弹窗表单分别调用 createRole 与 updateRole。
关键状态流：父页面控制 visible/type/editData；弹窗打开时按 type 回填表单；提交校验通过后调用接口，成功后通知父页面重新拉取列表。
使用注意事项：当前编辑模式依赖 props.editData 直接灌入表单对象，updateRole 所需的隐藏字段也沿着这条链路一并注入，属于隐式契约，后续改接口时要一起核对。
静态审查建议：
1. handleSubmit 在接口完成前就 closeModal，失败时弹窗已关闭，排错体验较弱，后续可改为成功后再关闭或补 loading/错误提示。
2. 编辑模式把 props.editData 直接 Object.assign 进 formModel，updateRole 对 id 等字段的依赖不够显式，建议后续单独定义编辑 DTO。
-->
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { FormInst, FormItemRule } from 'naive-ui'
import { createRequiredFormRule } from '@/utils/form/rule'
import { createRole, updateRole } from '@/service/api'
import { $t } from '@/locales'

// dom树形结构数据

export interface Props {
  /** 弹窗可见性 */
  visible: boolean
  /** 弹窗类型 add: 新增 edit: 编辑 */
  type?: 'add' | 'edit'
  /** 编辑的表格行数据 */
  editData?: UserManagement.User | null
}

export type ModalType = NonNullable<Props['type']>

defineOptions({ name: 'TableActionModal' })

const props = withDefaults(defineProps<Props>(), {
  type: 'add',
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
  const titles: Record<ModalType, string> = {
    add: $t('page.manage.role.title'),
    edit: $t('page.manage.role.editRole')
  }
  return titles[props.type]
})

const formRef = ref<HTMLElement & FormInst>()

type FormModel = Pick<UserManagement.User, 'name' | 'description' | 'email'>

const formModel = reactive<FormModel>(createDefaultFormModel())

const rules: Record<keyof FormModel, FormItemRule | FormItemRule[]> = {
  name: createRequiredFormRule($t('page.manage.role.form.roleName')),
  description: createRequiredFormRule($t('page.manage.role.form.roleDesc')),
  email: {}
}

function createDefaultFormModel(): FormModel {
  return {
    name: '',
    email: '',
    description: ''
  }
}

function handleUpdateFormModel(model: Partial<FormModel>) {
  Object.assign(formModel, model)
}

function handleUpdateFormModelByModalType() {
  const handlers: Record<ModalType, () => void> = {
    add: () => {
      // 新增场景必须用默认值覆盖旧弹窗残留，避免用户连续操作时串数据。
      const defaultFormModel = createDefaultFormModel()
      handleUpdateFormModel(defaultFormModel)
    },
    edit: () => {
      if (props.editData) {
        // 编辑场景直接复用父页面传来的角色行数据，当前实现默认 editData 已经包含更新接口需要的上下文字段。
        handleUpdateFormModel(props.editData)
      }
    }
  }

  handlers[props.type]()
}

async function handleSubmit() {
  await formRef.value?.validate()

  // 当前实现先关闭再请求，界面切换更快，但失败时用户需要重新打开弹窗查看上下文。
  closeModal()
  let data: any
  if (props.type === 'add') {
    data = await createRole(formModel)
  } else if (props.type === 'edit') {
    data = await updateRole(formModel)
  }
  if (!data.error) {
    emit('success')
  }
}

watch(
  () => props.visible,
  newValue => {
    if (newValue) {
      // 仅在弹窗打开时回填，关闭时不清空，减少不必要的响应式写入。
      handleUpdateFormModelByModalType()
    }
  }
)
</script>

<template>
  <n-modal v-model:show="modalVisible" preset="card" :title="title" :aria-label="title">
    <n-form ref="formRef" label-placement="left" :label-width="80" :model="formModel" :rules="rules">
      <n-form-item :label="$t('page.manage.role.roleName')" path="name">
        <n-input v-model:value="formModel.name" />
      </n-form-item>
      <n-form-item :label="$t('generate.role-description')">
        <n-input v-model:value="formModel.description" type="textarea" />
      </n-form-item>
      <n-space class="w-full pt-16px" :size="24" justify="end">
        <n-button class="w-72px" @click="closeModal">{{ $t('generate.cancel') }}</n-button>
        <n-button class="w-72px" type="primary" @click="handleSubmit">{{ $t('page.login.common.confirm') }}</n-button>
      </n-space>
    </n-form>
  </n-modal>
</template>

<style scoped></style>
