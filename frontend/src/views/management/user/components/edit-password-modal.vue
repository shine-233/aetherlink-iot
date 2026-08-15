<!--
文件用途：提供用户管理页的密码修改弹窗，负责回显账号、校验新密码并调用保存接口。
核心逻辑：组件通过 `visible` 与父页面共享开关状态，通过 `editData` 接收待修改用户快照，
  在本地表单中维护密码输入与确认密码校验，提交成功后通知父页面刷新列表。
关键注意事项：
1. 本组件只负责“修改密码”这条链路，不应在子组件内扩展用户资料编辑、角色变更等越权能力。
2. `email` 作为只读账号标识参与提交，真正的权限边界仍由后端接口校验，前端只做最小化展示与校验。
3. 目前关闭弹窗时不会主动清空未提交密码，若后续允许多次打开同一实例，建议补充显式 reset 流程。
静态审查建议：
1. `handleSubmit` 缺少独立 loading 与 `try/finally` 收口，连续点击时可能重复提交。
2. 组件通过 `watch(visible)` 回填表单，但没有在切换不同用户时主动重置旧密码输入，后续可补更严格的初始化策略。
3. 当前提交仅依赖 `editUser` 返回结构判断成功，后续可统一接入异常提示与审计日志能力。
-->
<script setup lang="ts">
import { computed, reactive, ref, toRefs, watch } from 'vue'
import type { FormInst, FormItemRule } from 'naive-ui'
import { editUser } from '@/service/api/auth'
import { formRules, getConfirmPwdRule } from '@/utils/form/rule'
import { $t } from '@/locales'

export interface Props {
  /** 弹窗可见性 */
  visible: boolean
  /** 编辑的表格行数据 */
  editData?: UserManagement.User | null
}

defineOptions({ name: 'EditPasswordModal' })

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

// 统一通过受控弹窗回写父组件，避免子组件直接篡改外层状态。
const closeModal = () => {
  modalVisible.value = false
}

const formRef = ref<HTMLElement & FormInst>()

type FormModel = Pick<UserManagement.User, 'email'> & {
  password: string
  confirmPwd: string
}

// 默认表单只保留密码修改所需最小字段，避免把完整用户对象直接绑定到弹窗状态。
const formModel = reactive<FormModel>(createDefaultFormModel())

const rules: Record<keyof FormModel, FormItemRule | FormItemRule[]> = {
  email: formRules.email,
  password: formRules.pwd,
  confirmPwd: getConfirmPwdRule(toRefs(formModel).password)
}

function createDefaultFormModel(): FormModel {
  return {
    email: '',
    password: '',
    confirmPwd: ''
  }
}

// 通过局部合并更新表单，避免重复书写字段赋值逻辑。
function handleUpdateFormModel(model: Partial<FormModel>) {
  Object.assign(formModel, model)
}

// 弹窗打开时仅回填账号标识等既有信息，密码字段仍维持当前默认值策略。
function handleUpdateFormModelByModalType() {
  if (props.editData) {
    handleUpdateFormModel(props.editData)
  }
}

async function handleSubmit() {
  // 提交前先走 Naive UI 表单校验，确保密码规则与确认密码一致性成立。
  await formRef.value?.validate()

  // 确认密码只用于前端校验，不应进入后端保存载荷。
  const submitData = { ...formModel } as Record<string, unknown>
  delete submitData.confirmPwd

  // 实际权限边界由后端 editUser 校验，这里只负责触发保存与成功回调。
  const data: any = await editUser(submitData)
  if (!data.error) {
    window.$message?.success(data.msg)
    emit('success')

    // 保存成功后回到默认表单，避免旧密码值残留在下一次打开时被误提交。
    handleUpdateFormModel(createDefaultFormModel())
    closeModal()
  }
}

watch(
  () => props.visible,
  newValue => {
    if (newValue) {
      // 每次打开弹窗时按父页面最新选中行回填账号信息，保持表单默认值与列表当前上下文一致。
      handleUpdateFormModelByModalType()
    }
  }
)
</script>

<template>
  <NModal v-model:show="modalVisible" preset="card" :title="$t('page.login.resetPwd.title')" class="w-700px">
    <NForm ref="formRef" label-placement="left" :label-width="80" :model="formModel" :rules="rules">
      <NGrid :cols="24" :x-gap="18">
        <NFormItemGridItem :span="24" :label="$t('page.manage.user.userName')" path="email">
          <NInput v-model:value="formModel.email" readonly />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.manage.user.password')" path="password">
          <NInput v-model:value="formModel.password" type="password" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.manage.user.confirmPwd')" path="confirmPwd">
          <NInput v-model:value="formModel.confirmPwd" type="password" />
        </NFormItemGridItem>
      </NGrid>
      <NSpace class="w-full pt-16px" :size="24" justify="end">
        <NButton class="w-72px" @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton class="w-72px" type="primary" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </NForm>
  </NModal>
</template>

<style scoped></style>
