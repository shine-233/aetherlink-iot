<!--
  文件用途：承载 frontend/src/views/_builtin/login/modules/pwd-login.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { InputHTMLAttributes } from 'vue'
import { NAutoComplete } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { $t } from '@/locales'
import { loginModuleRecord } from '@/constants/app'
import { useRouterPush } from '@/hooks/common/router'
import { useNaiveForm } from '@/hooks/common/form'
import { useAuthStore } from '@/store/modules/auth'
import { getFunction } from '@/service/api/setting'

defineOptions({
  name: 'PwdLogin'
})

const { locale } = useI18n()
const isRememberPath = ref(true)
const rememberMe = ref(false)
const authStore = useAuthStore()
const { toggleLoginModule } = useRouterPush()
const { formRef, validate } = useNaiveForm()
const showYzm = ref(false)
const showZc = ref(false)
const loginEmailInputProps = { 'data-testid': 'login-email' } as InputHTMLAttributes
const loginPasswordInputProps = { 'data-testid': 'login-password' } as InputHTMLAttributes

interface FormModel {
  userName: string
  password: string
}

const model: FormModel = reactive({
  userName: '',
  password: ''
})

const rules = computed<Record<keyof FormModel, App.Global.FormRule[]>>(() => {
  // 定义邮箱和手机号的基础格式校验。
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  const phoneRegex = /^(\+|\d)\d{5,15}$/

  return {
    userName: [
      {
        required: true,
        message: () => $t('form.userName.required'),
        trigger: ['blur']
      },
      {
        validator: (_rule, value) => {
          // 仅在有值时校验格式，空值交给 required 规则处理。
          if (value && !emailRegex.test(value) && !phoneRegex.test(value)) {
            return Promise.reject(new Error($t('form.userName.invalidFormat')))
          }
          return Promise.resolve()
        },
        message: () => $t('form.userName.invalidFormat'),
        trigger: ['input', 'blur']
      }
    ],
    password: [
      {
        required: true,
        message: () => $t('form.pwd.required'),
        trigger: ['input', 'blur']
      }
    ]
  }
})

// 常用邮箱后缀列表，用于账号输入框的自动补全。
const commonDomains = ['qq.com', '163.com', 'gmail.com', 'outlook.com', 'sina.com', 'hotmail.com', 'yahoo.com']

// 根据 @ 后的输入内容生成邮箱自动补全选项。
const emailOptions = computed(() => {
  const userName = model.userName
  if (!userName || !userName.includes('@')) {
    return []
  }

  const parts = userName.split('@')
  const username = parts[0]
  const domainInput = parts[1] || ''
  if (username === '') {
    return []
  }

  // 只提示仍然匹配且尚未完整输入的邮箱域名。
  const filteredDomains = commonDomains.filter(domain => domain.startsWith(domainInput) && domain !== domainInput)

  return filteredDomains.map(domain => `${username}@${domain}`)
})

async function handleSubmit() {
  await validate()
  await authStore.login(model.userName.trim(), model.password)

  if (rememberMe.value) {
    localStorage.setItem('rememberMe', 'true')
    localStorage.setItem('rememberedUserName', model.userName.trim())
    localStorage.removeItem('rememberedPassword')
  } else {
    localStorage.removeItem('rememberMe')
    localStorage.removeItem('rememberedUserName')
    localStorage.removeItem('rememberedPassword')
  }
}

async function getFunctionOption() {
  const { data } = await getFunction()
  if (data) {
    localStorage.setItem('enableZcAndYzm', JSON.stringify(data))
    showZc.value = data.find(v => v.name === 'enable_reg')?.enable_flag === 'enable'

    showYzm.value = data.find(v => v.name === 'use_captcha')?.enable_flag === 'enable'
  }
}

function loadSavedCredentials() {
  const savedRememberMe = localStorage.getItem('rememberMe')
  if (savedRememberMe === 'true') {
    rememberMe.value = true
    const savedUserName = localStorage.getItem('rememberedUserName')

    if (savedUserName) {
      model.userName = savedUserName
    }
  }
  localStorage.removeItem('rememberedPassword')
}

// 从 URL 参数加载自动登录预填信息，并立即清理敏感参数。
async function loadAutoLoginCredentials() {
  const urlParams = new URLSearchParams(window.location.search)
  const autoLogin = urlParams.get('auto') === 'true'
  const urlUsername = urlParams.get('username')
  const urlPassword = urlParams.get('password')

  if (urlPassword) {
    const url = new URL(window.location.href)
    url.searchParams.delete('password')
    url.searchParams.delete('auto')
    window.history.replaceState(null, '', `${url.pathname}${url.search}${url.hash}`)
  }

  if (autoLogin && urlUsername) {
    model.userName = urlUsername
  }
}

// 从市场登录页返回时预填邮箱。
function loadMarketEmail() {
  const urlParams = new URLSearchParams(window.location.search)
  const marketEmailFromUrl = urlParams.get('market_email')?.trim()
  if (marketEmailFromUrl) {
    model.userName = marketEmailFromUrl
  }
}

onMounted(() => {
  const is_remember_rath = localStorage.getItem('isRememberPath')
  if (is_remember_rath === '0' || is_remember_rath === '1') {
    isRememberPath.value = is_remember_rath === '1'
  }
  getFunctionOption()
  loadSavedCredentials()
  loadAutoLoginCredentials()
  loadMarketEmail()
})
</script>

<template>
  <NForm ref="formRef" :key="locale" :model="model" :rules="rules" size="large" :show-label="false">
    <NFormItem path="userName">
      <NAutoComplete
        v-model:value="model.userName"
        :options="emailOptions"
        :input-props="loginEmailInputProps"
        :placeholder="$t('page.login.common.userNamePlaceholder')"
        clearable
        @keydown.enter="handleSubmit"
      >
        <template #prefix>
          <svg class="w-18px h-18px ml--1" fill="#888" viewBox="0 0 24 24">
            <path
              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"
            ></path>
          </svg>
        </template>
      </NAutoComplete>
    </NFormItem>
    <NFormItem path="password">
      <NInput
        v-model:value="model.password"
        class="h-40px"
        type="password"
        show-password-on="click"
        :input-props="loginPasswordInputProps"
        :placeholder="$t('page.login.common.passwordPlaceholder')"
      >
        <template #prefix>
          <svg class="w-18px h-18px ml--1" fill="#888" viewBox="0 0 24 24">
            <path
              d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zm-6 9c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zm3.1-9H8.9V6c0-1.71 1.39-3.1 3.1-3.1 1.71 0 3.1 1.39 3.1 3.1v2z"
            ></path>
          </svg>
        </template>
      </NInput>
    </NFormItem>
    <NSpace vertical :size="18">
      <div class="flex-y-center justify-between">
        <NCheckbox v-model:checked="rememberMe">{{ $t('page.login.pwdLogin.rememberMe') }}</NCheckbox>
        <NButton quaternary @click="toggleLoginModule('reset-pwd')">
          {{ $t('page.login.pwdLogin.forgetPassword') }}
        </NButton>
      </div>
      <NButton
        style="border-radius: 8px"
        type="primary"
        size="large"
        block
        data-testid="login-submit"
        :loading="authStore.loginLoading"
        @click="handleSubmit"
      >
        {{ $t('route.login') }}
      </NButton>
      <n-divider v-if="showZc" title-placement="center" style="padding: 0px; margin: 0px">
        {{ $t('generate.or') }}
      </n-divider>
      <div class="flex-y-center justify-between gap-12px mt--4">
        <NButton
          v-if="showZc"
          class="flex-1"
          block
          type="primary"
          quaternary
          @click="toggleLoginModule('register-email')"
        >
          {{ $t('page.login.common.noAccount') }}
          {{ $t(loginModuleRecord.register) }}
        </NButton>
      </div>
    </NSpace>
  </NForm>
</template>

<style scoped></style>
