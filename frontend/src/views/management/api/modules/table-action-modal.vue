<!--
文件用途：提供 API 密钥管理页的新增/编辑弹窗，负责表单回填、租户上下文补全与提交成功后的父页回刷通知。
核心逻辑：组件根据 `type` 区分新增与编辑，依赖 `visible`、`editData` 和 `success` 事件与父页面协作。
关键注意事项：
1. `tenant_id` 属于权限边界字段，默认从登录态租户信息注入，不允许在弹窗中由用户自行切换。
2. 子组件只负责表单采集与提交，不直接操作列表数据，避免绕过父页面统一的刷新节奏。
3. 关闭弹窗发生在提交链路内部，若后续要增强失败体验，建议把“关闭时机”与“成功回刷”进一步拆开。
静态审查建议：
1. 当前编辑提交复用了 `formModel`，类型上没有显式声明 `id` 等编辑必需字段，后续可补更严格的表单模型。
2. `tenant_id` 校验规则为空字符串提示，可改成更明确的内部断言，减少维护者误解。
3. 新增与编辑的提交流程高度相似，后续可收口为统一提交函数并集中处理提示文案。
-->
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { FormInst, FormItemRule } from 'naive-ui'
import { createRequiredFormRule } from '@/utils/form/rule'
import { addKey, updateKey } from '@/service/api'
import { $t } from '@/locales'
import { useAuthStore } from '@/store/modules/auth'
const authStore = useAuthStore()

export interface Props {
  /** 弹窗可见性 */
  visible: boolean
  /** 弹窗类型 add: 新增 edit: 编辑 */
  type?: 'add' | 'edit'
  /** 编辑的表格行数据 */
  editData?: UserManagement.UserKey | null
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
    add: $t('page.manage.api.addApiKey'),
    edit: $t('page.manage.api.editAPi')
  }
  return titles[props.type]
})

const formRef = ref<HTMLElement & FormInst>()

type FormModel = Pick<UserManagement.UserKey, 'name' | 'tenant_id'>

const formModel = reactive<FormModel>(createDefaultFormModel())

const rules: Record<keyof FormModel, FormItemRule | FormItemRule[]> = {
  name: createRequiredFormRule('请输入名称'),
  tenant_id: createRequiredFormRule('')
}

function createDefaultFormModel(): FormModel {
  return {
    // 租户 ID 从登录态继承，保证 API Key 始终绑定在当前租户权限边界内。
    name: '',
    tenant_id: authStore.userInfo.tenant_id as string
  }
}

function handleUpdateFormModel(model: Partial<FormModel>) {
  Object.assign(formModel, model)
}

function handleUpdateFormModelByModalType() {
  const handlers: Record<ModalType, () => void> = {
    add: () => {
      // 新增时重置为当前租户下的默认表单，避免沿用上一次编辑残留值。
      const defaultFormModel = createDefaultFormModel()
      handleUpdateFormModel(defaultFormModel)
    },
    edit: () => {
      if (props.editData) {
        // 编辑时沿用父页面选中的当前行数据，但租户边界仍由既有数据决定。
        handleUpdateFormModel(props.editData)
      }
    }
  }

  handlers[props.type]()
}

async function handleSubmit() {
  await formRef.value?.validate()

  let data: any
  if (props.type === 'add') {
    data = await addKey(formModel)
  } else if (props.type === 'edit') {
    data = await updateKey(formModel)
  }
  if (!data.error) {
    emit('success')
  }
  // 当前实现无论成功失败都会关闭弹窗；若后续要保留失败现场，可只在成功后关闭。
  closeModal()
}

watch(
  () => props.visible,
  newValue => {
    if (newValue) {
      // 每次打开都按弹窗模式重新回填，避免新增和编辑之间相互污染。
      handleUpdateFormModelByModalType()
    }
  }
)
</script>

<template>
  <n-modal v-model:show="modalVisible" preset="card" :title="title">
    <n-form ref="formRef" label-placement="left" :label-width="80" :model="formModel" :rules="rules">
      <n-form-item :label="$t('page.manage.api.apiName')" path="name">
        <n-input v-model:value="formModel.name" :placeholder="$t('page.manage.api.form.apiName')" />
      </n-form-item>
      <n-space class="w-full pt-16px" :size="24" justify="end">
        <n-button class="w-72px" @click="closeModal">{{ $t('generate.cancel') }}</n-button>
        <n-button class="w-72px" type="primary" @click="handleSubmit">{{ $t('page.login.common.confirm') }}</n-button>
      </n-space>
    </n-form>
  </n-modal>
</template>

<style scoped></style>
