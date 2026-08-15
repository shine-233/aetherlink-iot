<!--
  文件用途：承载 frontend/src/views/_builtin/login/modules/reset-pwd.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { NAutoComplete, NButton, NForm, NFormItem, NInput, NSpace } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { $t } from '@/locales'
import { useRouterPush } from '@/hooks/common/router'
import { useFormRules, useNaiveForm } from '@/hooks/common/form'
import useSmsCode from '@/hooks/business/use-sms-code'
import { useAuthStore } from '@/store/modules/auth'
import { editUserPassWord, fetchEmailCodeByEmail, requestPasswordResetLink } from '@/service/api/auth'

defineOptions({
  name: 'ResetPwd'
})

const { locale } = useI18n()
const auth = useAuthStore()
const { toggleLoginModule } = useRouterPush()
const { formRef, validate } = useNaiveForm()
const readOnly = ref(true)
const route = useRoute()
const linkLoading = ref(false)

function firstQueryValue(value: unknown) {
  if (Array.isArray(value)) {
    return typeof value[0] === 'string' ? value[0] : ''
  }
  return typeof value === 'string' ? value : ''
}

const resetToken = computed(() => firstQueryValue(route.query.reset_token).trim())

interface FormModel {
  email: string
  verify_code: string
  password: string
}

const model: FormModel = reactive({
  email: firstQueryValue(route.query.email).trim(),
  verify_code: '',
  password: ''
})

// 常用邮箱后缀列表
const commonDomains = ['qq.com', '163.com', 'gmail.com', 'outlook.com', 'sina.com', 'hotmail.com', 'yahoo.com']

// 计算邮箱自动补全选项
const emailOptions = computed(() => {
  const email = model.email
  if (!email || !email.includes('@')) {
    return []
  }

  const parts = email.split('@')
  const username = parts[0]
  const domainInput = parts[1] || ''

  if (username === '') {
    return []
  }

  const filteredDomains = commonDomains.filter((domain) => domain.startsWith(domainInput) && domain !== domainInput)

  return filteredDomains.map((domain) => `${username}@${domain}`)
})

const { label, isCounting, loading: smsLoading, start, isValidEmail } = useSmsCode()

// 判断表单是否可以提交
const canSubmit = computed(() => {
  return (
    model.email.trim() !== '' &&
    model.password.trim() !== '' &&
    (resetToken.value !== '' || model.verify_code.trim() !== '')
  )
})

const rules = computed<Record<keyof FormModel, App.Global.FormRule[]>>(() => {
  const { formRules } = useFormRules()

  return {
    email: formRules.email,
    verify_code: resetToken.value ? [] : formRules.code,
    password: formRules.pwd
  }
})

async function handleSmsCode() {
  if (model.email) {
    if (await isValidEmail(model.email)) {
      const { error } = await fetchEmailCodeByEmail(model.email)
      if (!error) {
        start()
        window.$message?.success($t('page.login.common.codeSent'))
      }
    }
  } else {
    window.$message?.error($t('form.email.required'))
  }
}

async function handleResetLink() {
  if (!model.email) {
    window.$message?.error($t('form.email.required'))
    return
  }
  if (!model.verify_code.trim()) {
    window.$message?.error($t('page.login.common.codePlaceholder'))
    return
  }
  if (!(await isValidEmail(model.email))) {
    return
  }

  linkLoading.value = true
  try {
    const { error } = await requestPasswordResetLink({
      email: model.email,
      verify_code: model.verify_code
    })
    if (!error) {
      window.$message?.success($t('page.login.resetPwd.resetLinkSent'))
    }
  } finally {
    linkLoading.value = false
  }
}

async function handleSubmit() {
  try {
    await validate()

    const payload: Record<string, string | number> = {
      email: model.email,
      password: model.password,
      is_register: 2
    }
    if (resetToken.value) {
      payload.reset_token = resetToken.value
    } else {
      payload.verify_code = model.verify_code
    }

    const { error } = await editUserPassWord(payload)

    if (!error) {
      window.$message?.success($t('page.login.common.validateSuccess'))
      toggleLoginModule('pwd-login')
    }
  } catch (error) {
    console.error('Reset password failed:', error)
  }
}

setTimeout(() => {
  readOnly.value = false
}, 1000)
</script>

<template>
  <NForm ref="formRef" :key="locale" :model="model" :rules="rules" size="large" :show-label="false" autocomplete="off">
    <NFormItem path="email">
      <NAutoComplete
        v-model:value="model.email"
        :options="emailOptions"
        :placeholder="$t('page.login.register.emailPlaceholder')"
        clearable
        autocomplete="off"
        @keydown.enter="handleSubmit"
      />
    </NFormItem>
    <NFormItem v-if="!resetToken" path="verify_code">
      <div class="w-full">
        <div class="w-full flex-y-center">
          <NInput
            v-model:value="model.verify_code"
            :readonly="readOnly"
            :placeholder="$t('page.login.common.codePlaceholder')"
            autocomplete="off"
          />
          <div class="w-18px"></div>
          <NButton
            size="large"
            :disabled="isCounting || !model.email.trim()"
            :loading="smsLoading"
            @click="handleSmsCode"
          >
            {{ label }}
          </NButton>
        </div>
        <NButton
          size="large"
          class="mt-12px"
          block
          :disabled="!model.email.trim() || !model.verify_code.trim()"
          :loading="linkLoading"
          @click="handleResetLink"
        >
          {{ $t('page.login.resetPwd.sendResetLink') }}
        </NButton>
      </div>
    </NFormItem>
    <div v-else class="mb-12px text-xs text-gray-500">{{ $t('page.login.resetPwd.linkMode') }}</div>

    <NFormItem path="password">
      <div class="w-full">
        <NInput
          v-model:value="model.password"
          type="password"
          autocomplete="new-password"
          show-password-on="click"
          :placeholder="$t('page.login.common.passwordPlaceholder')"
        />
        <div class="mt-1 text-xs text-gray-500">{{ $t('form.pwd.tip') }}</div>
      </div>
    </NFormItem>

    <NSpace vertical :size="18" class="w-full">
      <NButton
        type="primary"
        size="large"
        round
        block
        :loading="auth.loginLoading"
        :disabled="!canSubmit"
        @click="handleSubmit"
      >
        {{ $t('common.confirm') }}
      </NButton>
      <NButton size="large" round block @click="toggleLoginModule('pwd-login')">
        {{ $t('page.login.common.back') }}
      </NButton>
    </NSpace>
  </NForm>
</template>

<style scoped>
/* 保留这个样式，防止自动填充背景变黄 */
input:-webkit-autofill,
input:-webkit-autofill:hover,
input:-webkit-autofill:focus,
input:-webkit-autofill:active {
  -webkit-box-shadow: 0 0 0 30px white inset !important;
  -webkit-text-fill-color: inherit !important;
  transition: background-color 5000s ease-in-out 0s;
}
</style>
