<!--
  文件用途：承载 frontend/src/views/_builtin/login/modules/register-super-admin.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, watch } from 'vue'
import { NAutoComplete, NButton, NForm, NFormItem, NInput } from 'naive-ui'
import { $t } from '@/locales'
import { useFormRules, useNaiveForm } from '@/hooks/common/form'
import { useAuthStore } from '@/store/modules/auth'
import { fetchSuperAdminInit } from '@/service/api/auth'

defineOptions({
  name: 'SuperAdminRegisterPage'
})

interface Props {
  marketUrl?: string
  marketEmail?: string
  marketRegistered?: boolean
  marketSource?: string
}

const props = withDefaults(defineProps<Props>(), {
  marketUrl: '',
  marketEmail: '',
  marketRegistered: false,
  marketSource: ''
})

const auth = useAuthStore()
const { formRef, validate } = useNaiveForm()
const fallbackMarketUrl = import.meta.env.VITE_MARKET_URL || ''

interface FormModel {
  email: string
  pwd: string
}

const model: FormModel = reactive({
  email: props.marketEmail || '',
  pwd: ''
})

const marketUrl = computed(() => {
  const configured = (props.marketUrl || fallbackMarketUrl).trim()
  if (!configured) return fallbackMarketUrl
  if (configured.includes('localhost') || configured.includes('127.0.0.1') || configured.includes('0.0.0.0')) {
    return fallbackMarketUrl
  }
  return configured
})

const emailLocked = computed(() => props.marketEmail.trim() !== '')

const canSubmit = computed(() => {
  return model.email.trim() !== '' && model.pwd.trim() !== ''
})

const commonDomains = ['qq.com', '163.com', 'gmail.com', 'outlook.com', 'sina.com', 'hotmail.com', 'yahoo.com']

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
  const filteredDomains = commonDomains.filter(domain => domain.startsWith(domainInput) && domain !== domainInput)
  return filteredDomains.map(domain => `${username}@${domain}`)
})

const rules = computed<Record<keyof FormModel, App.Global.FormRule[]>>(() => {
  const { formRules } = useFormRules()
  return {
    email: formRules.email,
    pwd: formRules.pwd
  }
})

function buildMarketRegisterUrl() {
  const base = marketUrl.value
  if (!base) {
    throw new Error($t('custom.login.registerSuperAdmin.marketUrlMissing'))
  }
  const url = base.endsWith('/register') ? new URL(base) : new URL('/register', base)
  url.searchParams.set('callback', window.location.href)
  url.searchParams.set('return_to', window.location.href)
  return url.toString()
}

function goToMarketRegister(options: { silentMissingMarketUrl?: boolean } = {}) {
  if (!marketUrl.value) {
    if (!options.silentMissingMarketUrl) {
      window.$message?.warning($t('custom.login.registerSuperAdmin.marketUrlConfigRequired'))
    }
    return false
  }
  window.location.href = buildMarketRegisterUrl()
  return true
}

async function handleSubmit() {
  try {
    await validate()
    const resp = (await fetchSuperAdminInit({
      email: model.email.trim(),
      password: model.pwd,
      market_registered: props.marketRegistered,
      market_email: props.marketEmail.trim() || model.email.trim(),
      market_source: props.marketSource || 'horizon'
    })) as any

    if (resp?.error) {
      const code = resp?.error?.code ?? resp?.code
      if (code === 200055) {
        window.$message?.warning($t('custom.login.registerSuperAdmin.marketRegistrationRequired'))
        goToMarketRegister({ silentMissingMarketUrl: true })
        return
      }
    }

    if (!resp.error) {
      window.$message?.success($t('custom.login.registerSuperAdmin.initSuccess'))
      if (resp.data && resp.data.token) {
        // 通过 loginByToken 完成登录流程，确保 userInfo 被正确存储到 localStorage
        // 这样后续依赖用户信息的兼容初始化流程就能读取到正确上下文
        const loginToken: Api.Auth.LoginToken = {
          token: resp.data.token,
          expires_in: resp.data.expires_in || 3600
        }
        await auth.loginByToken(loginToken)

        setTimeout(() => {
          window.location.href = '/'
        }, 500)
      }
    }
  } catch (error: any) {
    const code = error.response?.data?.code
    const msg = error.response?.data?.message
    if (code === 200055) {
      window.$message?.warning(msg || $t('custom.login.registerSuperAdmin.marketRegistrationRequired'))
      goToMarketRegister({ silentMissingMarketUrl: true })
    } else {
      window.$message?.error(msg || error?.message || $t('custom.login.registerSuperAdmin.initFailed'))
      console.error('Initialization failed:', error)
    }
  }
}

watch(
  () => props.marketEmail,
  value => {
    if (value) {
      model.email = value
    }
  },
  { immediate: true }
)

onMounted(() => {
  if (props.marketEmail) {
    model.email = props.marketEmail
  }
})
</script>

<template>
  <NForm ref="formRef" :model="model" :rules="rules" size="large" :show-label="false" autocomplete="off">
    <NFormItem path="email">
      <NAutoComplete
        v-model:value="model.email"
        :disabled="emailLocked"
        :options="emailOptions"
        :placeholder="$t('page.login.register.emailPlaceholder')"
        clearable
        autocomplete="off"
        @keydown.enter="handleSubmit"
      />
    </NFormItem>
    <NFormItem path="pwd">
      <NInput
        v-model:value="model.pwd"
        type="password"
        show-password-on="click"
        :placeholder="$t('page.login.common.passwordPlaceholder')"
        autocomplete="new-password"
      />
    </NFormItem>

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
  </NForm>
</template>

<style scoped>
input:-webkit-autofill,
input:-webkit-autofill:hover,
input:-webkit-autofill:focus,
input:-webkit-autofill:active {
  -webkit-box-shadow: 0 0 0 30px white inset !important;
  -webkit-text-fill-color: inherit !important;
  transition: background-color 5000s ease-in-out 0s;
}
</style>
