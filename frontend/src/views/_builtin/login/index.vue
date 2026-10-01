<!--
  文件用途：登录页外壳（背景、卡片、工具栏、模块切换与文档标题）。
  核心逻辑：首次安装/市场注册流程在 useLoginSetupFlow；工具栏在 modules/login-toolbar.vue；
    本文件只负责选择当前登录模块并渲染。
  关键注意事项：测试通过 setupState 读取本文件顶层绑定（effectiveModule、normalizeMarketUrl 等），
    重命名顶层绑定需同步 __tests__/index.test.ts。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, watch } from 'vue'
import type { Component } from 'vue'
import { useTitle } from '@vueuse/core'
import { NEllipsis, NSpin } from 'naive-ui'
import { useRoute } from 'vue-router'
import { $t } from '@/locales'
import { useAppStore } from '@/store/modules/app'
import { useThemeStore } from '@/store/modules/theme'
import { loginModuleRecord } from '@/constants/app'
import { useSysSettingStore } from '@/store/modules/sys-setting'
import { resolveDocumentTitle } from '@/router/guard/title-helper'
import PwdLogin from './modules/pwd-login.vue'
import LoginBg from './modules/login-bg.vue'
import LoginToolbar from './modules/login-toolbar.vue'
import { normalizeMarketUrl as normalizeMarketUrlWith, setupGuideKeys, useLoginSetupFlow } from './useLoginSetupFlow'

const Register = defineAsyncComponent(() => import('./modules/register.vue'))
const RegisterByEmail = defineAsyncComponent(() => import('./modules/register-email.vue'))
const RegisterSuperAdmin = defineAsyncComponent(() => import('./modules/register-super-admin.vue'))
const ResetPwd = defineAsyncComponent(() => import('./modules/reset-pwd.vue'))
const BindWechat = defineAsyncComponent(() => import('./modules/bind-wechat.vue'))

interface Props {
  /** The login module */
  module?: UnionKey.LoginModule
}

const props = withDefaults(defineProps<Props>(), {
  module: 'pwd-login'
})

const appStore = useAppStore()
const themeStore = useThemeStore()
const sysSetting = useSysSettingStore()
const route = useRoute()

const localeButtonLabel = computed(
  () => appStore.localeOptions.find((item) => item.key === appStore.locale)?.label || 'Lang'
)
const cycleLocale = () => {
  const currentIndex = appStore.localeOptions.findIndex((item) => item.key === appStore.locale)
  const nextIndex = currentIndex >= 0 ? (currentIndex + 1) % appStore.localeOptions.length : 0
  appStore.changeLocale(appStore.localeOptions[nextIndex].key)
}

// 市场地址：优先使用环境变量 VITE_MARKET_URL；未配置时保持为空，避免误跳示例域名。
const fallbackMarketUrl = import.meta.env.VITE_MARKET_URL || ''
const normalizeMarketUrl = (baseUrl?: string) => normalizeMarketUrlWith(baseUrl, fallbackMarketUrl)

// 首次安装 / 市场注册流程（状态机见 useLoginSetupFlow）。
const {
  setupState,
  loading,
  redirectingToMarket,
  returnedFromMarket,
  marketEmail,
  marketSource,
  marketRegisterUrl,
  needsSuperAdminInit,
  setupNextStep,
  loadSetupState
} = useLoginSetupFlow({ searchParams: new URLSearchParams(window.location.search), fallbackMarketUrl })

onMounted(() => {
  loadSetupState()
})

interface LoginModule {
  key: UnionKey.LoginModule
  label: string
  component: Component
}

const modules: LoginModule[] = [
  { key: 'pwd-login', label: loginModuleRecord['pwd-login'], component: PwdLogin },
  { key: 'register', label: loginModuleRecord.register, component: Register },
  { key: 'register-email', label: loginModuleRecord.register, component: RegisterByEmail },
  { key: 'register-super-admin', label: loginModuleRecord.register, component: RegisterSuperAdmin },
  { key: 'reset-pwd', label: loginModuleRecord['reset-pwd'], component: ResetPwd },
  { key: 'bind-wechat', label: loginModuleRecord['bind-wechat'], component: BindWechat }
]

// 实际使用的 module（props 覆盖优先；未初始化时强制进入超管初始化）。
const effectiveModule = computed<UnionKey.LoginModule>(() => {
  if (props.module && props.module !== 'pwd-login') return props.module
  return needsSuperAdminInit.value ? 'register-super-admin' : 'pwd-login'
})

const activeModule = computed(() => modules.find((item) => item.key === effectiveModule.value) || modules[0])

const activeModuleProps = computed(() => {
  if (activeModule.value.key !== 'register-super-admin') return {}
  return {
    marketUrl: marketRegisterUrl.value,
    marketEmail: marketEmail.value,
    marketRegistered: returnedFromMarket.value,
    marketSource: marketSource.value
  }
})

const setupGuide = computed(() => {
  const keys = setupGuideKeys(setupNextStep.value)
  return { title: $t(keys.title as never), description: $t(keys.description as never) }
})

const MODULE_TITLE_KEYS: Partial<Record<UnionKey.LoginModule, string>> = {
  'pwd-login': 'page.login.pwdLogin.title',
  'register-email': 'page.login.register.title',
  'register-super-admin': 'custom.login.completeInitialization',
  'reset-pwd': 'page.login.resetPwd.title'
}

// 计算当前模块的标题
const moduleTitle = computed(() =>
  $t((MODULE_TITLE_KEYS[effectiveModule.value] ?? 'page.login.pwdLogin.title') as never)
)

// 卡片背景色 / 边框颜色
const cardBgColor = computed(() => (themeStore.darkMode ? 'rgba(31, 41, 55, 0.95)' : 'rgba(255, 255, 255, 0.95)'))
const borderColor = computed(() => (themeStore.darkMode ? '#374151' : '#e5e7eb'))

function resolveLoginModulePath(module: UnionKey.LoginModule) {
  return module === 'pwd-login' ? '/login' : `/login/${module}`
}

function resolveLoginDocumentTitle() {
  const appTitle = sysSetting.system_name === '' ? $t('title') : sysSetting.system_name
  return resolveDocumentTitle(
    { path: resolveLoginModulePath(effectiveModule.value), meta: route.meta },
    appTitle || $t('title'),
    $t
  )
}

watch(
  [effectiveModule, () => sysSetting.system_name],
  () => {
    useTitle(resolveLoginDocumentTitle())
  },
  { immediate: true }
)
</script>

<template>
  <div
    class="relative size-full flex-center overflow-hidden min-h-screen"
    :style="{
      fontFamily: '-apple-system, BlinkMacSystemFont, Segoe UI, PingFang SC, Microsoft YaHei, sans-serif',
      background: sysSetting.home_background
        ? 'none'
        : themeStore.darkMode
          ? 'linear-gradient(135deg, #1f2937 0%, #374151 50%, #111827 100%)'
          : 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #06b6d4 100%)'
    }"
  >
    <!-- 使用 LoginBg 组件显示后端配置的背景图片 -->
    <LoginBg v-if="sysSetting.home_background" :theme-color="themeStore.themeColor" :sys-setting="sysSetting" />

    <!-- 默认背景动画效果 -->
    <div v-else class="bg-animation">
      <div class="bg-animation-inner" :class="{ 'dark-theme': themeStore.darkMode }"></div>
    </div>

    <!-- Loading / redirect 状态 -->
    <div v-if="loading || redirectingToMarket" class="flex-center">
      <n-spin size="large" />
    </div>

    <!-- 登录卡片 -->
    <div
      v-if="!loading"
      class="relative z-10 w-full max-w-md mx-4 p-8 rounded-2xl shadow-2xl backdrop-blur-xl animate-in slide-in-from-bottom-4 duration-500"
      :style="{
        width: '380px',
        background: cardBgColor,
        border: `1px solid ${borderColor}`,
        boxShadow: themeStore.darkMode ? '0 20px 60px rgba(0, 0, 0, 0.3)' : '0 20px 60px rgba(0, 0, 0, 0.1)'
      }"
    >
      <!-- 顶部控制栏 -->
      <LoginToolbar
        :dark-mode="themeStore.darkMode"
        :border-color="borderColor"
        :locale-label="localeButtonLabel"
        @toggle-theme="themeStore.toggleThemeScheme"
        @cycle-locale="cycleLocale"
      />

      <!-- Logo区域 -->
      <div class="text-center mb-6">
        <div
          class="inline-flex items-center justify-center w-12 h-12 rounded-xl mb-3 shadow-lg transition-transform duration-300 hover:scale-110"
          :style="{ background: themeStore.themeColor }"
        >
          <SystemLogo width="32" class="text-white" />
        </div>
        <div class="title-container">
          <n-ellipsis
            :line-clamp="2"
            class="text-xl font-semibold mb-1 title-artistic"
            :style="{
              color: themeStore.darkMode ? '#f9fafb' : '#1f2937',
              lineHeight: '1.4',
              letterSpacing: '0.02em',
              textAlign: 'center'
            }"
          >
            {{ $t('system.title') }}
          </n-ellipsis>
        </div>
        <p class="text-xs opacity-60" :style="{ color: themeStore.darkMode ? '#9ca3af' : '#6b7280' }">
          {{ $t('system.description') }}
        </p>
        <div
          v-if="setupState"
          class="mt-3 rounded-lg px-3 py-2 text-left"
          :style="{
            background: themeStore.darkMode ? 'rgba(55, 65, 81, 0.85)' : 'rgba(243, 244, 246, 0.95)',
            border: `1px solid ${borderColor}`
          }"
        >
          <p class="text-sm font-medium" :style="{ color: themeStore.darkMode ? '#f9fafb' : '#111827' }">
            {{ setupGuide.title }}
          </p>
          <p class="mt-1 text-xs leading-5" :style="{ color: themeStore.darkMode ? '#d1d5db' : '#4b5563' }">
            {{ setupGuide.description }}
          </p>
        </div>
      </div>

      <!-- 表单区域 -->
      <div class="space-y-6">
        <!-- <h2
          class="text-lg font-medium text-center"
          :style="{ color: themeStore.darkMode ? '#f9fafb' : '#1f2937' }"
        >
          {{ $t(activeModule.label as any) }}
        </h2> -->

        <div class="transition-all duration-300">
          <Transition :name="themeStore.page.animateMode" mode="out-in" appear>
            <component :is="activeModule.component" v-bind="activeModuleProps" />
          </Transition>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.flex-center {
  display: flex;
  justify-content: center;
  align-items: center;
}

/* 背景动画效果 */
.bg-animation {
  position: absolute;
  width: 100%;
  height: 100%;
  overflow: hidden;
  opacity: 0.6;
  /* 纯装饰背景层：必须放行指针事件，否则会持续拦截登录卡片上方按钮的点击
     （Playwright "subtree intercepts pointer events"，E2E 语言切换用例超时的根因）。 */
  pointer-events: none;
}

.bg-animation-inner::before {
  content: '';
  position: absolute;
  width: 100%;
  height: 100%;
  background:
    radial-gradient(circle at 20% 30%, rgba(255, 255, 255, 0.1) 0%, transparent 50%),
    radial-gradient(circle at 80% 70%, rgba(255, 255, 255, 0.1) 0%, transparent 50%),
    radial-gradient(circle at 40% 80%, rgba(255, 255, 255, 0.1) 0%, transparent 50%);
  animation: bgFloat 10s ease-in-out infinite;
}

.bg-animation-inner.dark-theme::before {
  background:
    radial-gradient(circle at 20% 30%, rgba(99, 102, 241, 0.1) 0%, transparent 50%),
    radial-gradient(circle at 80% 70%, rgba(99, 102, 241, 0.1) 0%, transparent 50%),
    radial-gradient(circle at 40% 80%, rgba(99, 102, 241, 0.1) 0%, transparent 50%);
}

@keyframes bgFloat {
  0%,
  100% {
    transform: translateY(0px) rotate(0deg);
  }

  50% {
    transform: translateY(-20px) rotate(180deg);
  }
}

/* 进入动画 */
@keyframes slide-in-from-bottom {
  from {
    opacity: 0;
    transform: translateY(30px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.animate-in {
  animation: slide-in-from-bottom 0.5s ease-out;
}

/* 标题容器样式 */
.title-container {
  position: relative;
  margin: 0 auto;
  text-align: center;
  max-width: 90%;
}

/* 移除了横线装饰
.title-container::before,
.title-container::after {
  content: '';
  position: absolute;
  top: 50%;
  width: 15%;
  height: 1px;
  background: currentColor;
  opacity: 0.3;
}

.title-container::before {
  left: 0;
  transform: translateX(-50%);
}

.title-container::after {
  right: 0;
  transform: translateX(50%);
}
*/

/* 标题艺术化样式 */
.title-artistic {
  word-break: break-word;
  hyphens: auto;
  display: block;
  padding: 0 1em;
  text-align: center;
  line-height: 1.4;
  letter-spacing: 0.02em;
}

.title-artistic::first-line {
  font-size: 0.9em;
}

.title-artistic::first-letter {
  font-size: 1.2em;
}

/* 响应式适配 */
@media (max-width: 640px) {
  .size-full {
    padding: 1rem;
  }

  .title-container {
    max-width: 95%;
  }

  .title-container::before,
  .title-container::after {
    width: 10%;
  }

  .title-artistic {
    font-size: 1rem;
    padding: 0 0.5em;
  }

  .title-artistic::first-line {
    font-size: 0.85em;
  }
}

/* 过渡动画 */
.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

.slide-enter-active,
.slide-leave-active {
  transition: all 0.3s ease;
}

.slide-enter-from {
  opacity: 0;
  transform: translateX(20px);
}

.slide-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}
</style>
