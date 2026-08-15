<!--
  文件用途：承载 frontend/src/views/_builtin/login/modules/register.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<script setup lang="ts">
import { computed, reactive, ref, toRefs } from 'vue'
import { $t } from '@/locales'
import { useRouterPush } from '@/hooks/common/router'
import { useFormRules, useNaiveForm } from '@/hooks/common/form'
import useSmsCode from '@/hooks/business/use-sms-code'
import { useAuthStore } from '@/store/modules/auth'
import { getConfirmPwdRule } from '@/utils/form/rule'

defineOptions({
  name: 'RegisterPage'
})

const auth = useAuthStore()
const { toggleLoginModule } = useRouterPush()
const { formRef, validate } = useNaiveForm()

interface FormModel {
  phone: string
  code: string
  pwd: string
  confirmPwd: string
}

const model: FormModel = reactive({
  phone: '',
  code: '',
  pwd: '',
  confirmPwd: ''
})
const { label, isCounting, loading: smsLoading, start } = useSmsCode()

const rules = computed<Record<keyof FormModel, App.Global.FormRule[]>>(() => {
  const { formRules } = useFormRules() // inside computed to make locale reactive

  return {
    phone: formRules.phone,
    code: formRules.code,
    pwd: formRules.pwd,
    confirmPwd: getConfirmPwdRule(toRefs(model).pwd)
  }
})
const agreement = ref(false)
function handleSmsCode() {
  start()
}
async function handleSubmit() {
  await validate()
  window.$message?.success($t('page.login.common.validateSuccess'))
}
</script>

<template>
  <NForm ref="formRef" :model="model" :rules="rules" size="large" :show-label="false">
    <NFormItem path="phone">
      <NInput v-model:value="model.phone" :placeholder="$t('page.login.common.phonePlaceholder')" />
    </NFormItem>
    <NFormItem path="code">
      <div class="w-full flex-y-center">
        <NInput v-model:value="model.code" :placeholder="$t('page.login.common.codePlaceholder')" />
        <div class="w-18px"></div>
        <NButton size="large" :disabled="isCounting" :loading="smsLoading" @click="handleSmsCode">
          {{ label }}
        </NButton>
      </div>
    </NFormItem>

    <NFormItem path="pwd">
      <NInput
        v-model:value="model.pwd"
        type="password"
        show-password-on="click"
        :placeholder="$t('page.login.common.passwordPlaceholder')"
      />
    </NFormItem>
    <NFormItem path="confirmPwd">
      <NInput
        v-model:value="model.confirmPwd"
        type="password"
        show-password-on="click"
        :placeholder="$t('page.login.common.confirmPasswordPlaceholder')"
      />
    </NFormItem>

    <NSpace vertical :size="18" class="w-full">
      <LoginAgreement v-model:value="agreement" />
      <NButton type="primary" size="large" round block :loading="auth.loginLoading" @click="handleSubmit">
        {{ $t('common.confirm') }}
      </NButton>
      <NButton size="large" round block @click="toggleLoginModule('pwd-login')">
        {{ $t('page.login.common.back') }}
      </NButton>
    </NSpace>
  </NForm>
</template>

<style scoped></style>
