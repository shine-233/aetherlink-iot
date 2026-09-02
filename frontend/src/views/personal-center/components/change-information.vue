<!--
  文件用途：承载 frontend/src/views/personal-center/components/change-information.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<!--
 * @Descripttion:
 * @version:
 * @Author: zhaoqi
 * @Date: 2024-03-17 13:31:30
 * @LastEditors: zhaoqi
 * @LastEditTime: 2024-03-20 17:13:33
-->
<script setup lang="ts">
import { computed, ref, toRefs } from 'vue'
import type { FormItemRule, FormRules } from 'naive-ui'
import { useNaiveForm } from '@/hooks/common/form'
import { getConfirmPwdRule } from '@/utils/form/rule'
import { changeInformation, passwordModification } from '@/service/api/personal-center'
import { $t } from '@/locales'
import { generateRandomHexString, validName, validPasswordByExp } from '@/utils/common/tool'
import { encryptDataByRsa } from '@/utils/security/rsa-encrypt'

export interface Props {
  /** 弹窗可见性 */
  visible: boolean
  type?: 'amend' | 'changePassword'
}

export type ModalType = NonNullable<Props['type']>

defineOptions({ name: 'TableActionModal' })
const props = withDefaults(defineProps<Props>(), {
  type: 'amend',
  editData: null
})

interface Emits {
  (e: 'update:visible', visible: boolean): void

  (e: 'modification', name?: string): void
}
const { formRef, validate } = useNaiveForm()
const emit = defineEmits<Emits>()

const modalVisible = computed({
  get() {
    return props.visible
  },
  set(visible) {
    emit('update:visible', visible)
  }
})
const title = computed(() => {
  const titles: Record<ModalType, string> = {
    amend: $t('custom.personalCenter.modifyBasicInfo'),
    changePassword: $t('custom.personalCenter.changePassword')
  }
  return titles[props.type]
})
const estimate = computed(() => {
  const titles: Record<ModalType, string> = {
    amend: 'amend',
    changePassword: 'changePassword'
  }
  return titles[props.type]
})

/** 初始from数据 */
const formData = ref({
  name: '',
  old_password: '',
  password: '',
  passwords: ''
})
/** 关闭弹框 */
const closeModal = () => {
  modalVisible.value = false
  formData.value.name = ''
}
// 提交进行中标记：防止重复提交，并驱动保存按钮 loading 态。
const submitLoading = ref(false)
/**
 * 修改姓名
 *
 * @param name
 */
const editName = async () => {
  await validate()
  const data = { name: formData.value.name }
  submitLoading.value = true
  try {
    const res = await changeInformation(data)

    // 失败时保持弹窗打开，保留用户输入；错误提示由请求层全局 onError 统一弹出。
    if (!res.error) {
      modalVisible.value = false
      emit('modification', formData.value.name)
    }
  } finally {
    submitLoading.value = false
  }
}
/** passwordModification */
const password = async () => {
  await validate()
  const enableConfigRaw = localStorage.getItem('enableZcAndYzm')
  const data: Array<{ name?: string; enable_flag?: string }> = enableConfigRaw ? JSON.parse(enableConfigRaw) : []
  let salt: string | null = null
  let password1 = formData.value.password
  if (data.find(v => v.name === 'frontend_res')?.enable_flag === 'enable') {
    salt = generateRandomHexString(16)
    // RSA helper 已迁移为 WebCrypto 异步实现（RSA-OAEP/SHA-256）
    password1 = await encryptDataByRsa(password1 + salt)
  }
  const param = {
    old_password: formData.value.old_password,
    password: password1,
    salt
  }
  submitLoading.value = true
  try {
    const res = await passwordModification(param)
    if (!res.error) {
      modalVisible.value = false
      emit('modification')
    }
  } finally {
    submitLoading.value = false
  }
}

function submit() {
  if (estimate.value === 'amend') {
    editName()
  } else {
    password()
  }
}
const rules: FormRules = {
  name: [
    {
      required: true,
      validator(rule: FormItemRule, value: string) {
        if (rule && !validName(value)) {
          return new Error($t('custom.personalCenter.nameFieldNotEmpty'))
        }
        return true
      },
      trigger: ['input', 'blur']
    }
  ],
  password: [
    {
      required: true,
      validator(rule: FormItemRule, value: string) {
        if (value.length < 8 || value.length > 20) {
          return Promise.reject(rule.message)
        }
        if (!validPasswordByExp(value)) {
          return Promise.reject(rule.message)
        }
        return Promise.resolve()
      },
      message: $t('form.pwd.tip'),
      trigger: ['input', 'blur']
    }
  ],
  passwords: getConfirmPwdRule(toRefs(formData.value).password)
}
</script>

<template>
  <NModal v-model:show="modalVisible" preset="card" :title="title" class="w-500px">
    <NForm ref="formRef" label-placement="left" :model="formData" :rules="rules">
      <NGrid :cols="2" :x-gap="18">
        <NFormItemGridItem v-if="estimate === 'amend'" :span="24" :label="$t('page.manage.user.userName')" path="name">
          <NInput v-model:value="formData.name" />
        </NFormItemGridItem>
        <NFormItemGridItem
          v-if="estimate === 'changePassword'"
          :span="24"
          label-width="100"
          type="password"
          show-password-on="mousedown"
          :label="$t('generate.old-password')"
          path="old_password"
        >
          <NInput v-model:value="formData.old_password" type="password" show-password-on="click" />
        </NFormItemGridItem>
        <NFormItemGridItem
          v-if="estimate === 'changePassword'"
          label-width="100"
          :span="24"
          :label="$t('generate.new-password')"
          path="password"
        >
          <NInput v-model:value="formData.password" type="password" show-password-on="click" />
        </NFormItemGridItem>
        <NFormItemGridItem
          v-if="estimate === 'changePassword'"
          :span="24"
          label-width="100"
          :label="$t('generate.repeat-new-password')"
          path="passwords"
        >
          <NInput v-model:value="formData.passwords" type="password" show-password-on="click" />
        </NFormItemGridItem>
      </NGrid>
      <NSpace class="w-full pt-16px" :size="24" justify="end">
        <NButton class="w-72px" @click="closeModal">{{ $t('generate.cancel') }}</NButton>
        <NButton class="w-72px" type="primary" :loading="submitLoading" @click="submit">{{ $t('common.save') }}</NButton>
      </NSpace>
    </NForm>
  </NModal>
</template>

<style scoped></style>
