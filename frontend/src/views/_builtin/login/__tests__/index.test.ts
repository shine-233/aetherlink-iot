/**
 * 文件用途：验证 frontend/src/views/_builtin/login/__tests__/index 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchTenantSetupState: vi.fn(),
  useTitle: vi.fn(),
  changeLocale: vi.fn()
}))

vi.mock('@/service/api/auth', () => ({
  fetchTenantSetupState: hoisted.fetchTenantSetupState
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => ({
    locale: 'zh-CN',
    localeOptions: [
      { key: 'zh-CN', label: 'CN' },
      { key: 'en-US', label: 'English' }
    ],
    changeLocale: hoisted.changeLocale
  })
}))

vi.mock('@/store/modules/theme', () => ({
  useThemeStore: () => ({
    darkMode: false,
    themeColor: '#6366f1',
    page: { animateMode: 'fade' },
    toggleThemeScheme: vi.fn()
  })
}))

vi.mock('@/store/modules/sys-setting', () => ({
  useSysSettingStore: () => ({
    home_background: '',
    system_name: 'AetherLink IoT'
  })
}))

vi.mock('@/constants/app', () => ({
  loginModuleRecord: {
    'pwd-login': 'page.login.pwdLogin.title',
    register: 'page.login.register.title',
    'reset-pwd': 'page.login.resetPwd.title',
    'bind-wechat': 'page.login.bindWeChat.title'
  }
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() })
}))

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => ({ path: '/login', meta: {}, query: {} }),
    useRouter: () => ({ push: vi.fn(), back: vi.fn() })
  }
})

vi.mock('@vueuse/core', () => ({
  useTitle: hoisted.useTitle
}))

import LoginIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(LoginIndex, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NSpace: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NSpin: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NEllipsis: defineComponent({
          setup(_, { slots }) {
            return () => h('span', slots.default?.())
          }
        }),
        SystemLogo: true,
        PwdLogin: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        Register: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        RegisterByEmail: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        RegisterSuperAdmin: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        ResetPwd: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        BindWechat: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        LoginBg: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        Transition: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('LoginIndex', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchTenantSetupState.mockResolvedValue({ data: { has_admin: true, entry: 'login' } })
  })

  afterEach(() => {
    mountedWrappers.forEach((w) => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and call loadSetupState', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.fetchTenantSetupState).toHaveBeenCalledTimes(1)
    expect(getState(wrapper).setupState).toEqual({ has_admin: true, entry: 'login' })
  })

  it('should set loading to false after loadSetupState', async () => {
    hoisted.fetchTenantSetupState.mockResolvedValue({ data: { has_admin: true, entry: 'login' } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.loading).toBe(false)
  })

  it('should default to pwd-login module', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.effectiveModule).toBe('pwd-login')
  })

  it('should use props module when provided and not pwd-login', async () => {
    const wrapper = mountComponent({ module: 'reset-pwd' })
    await flushPromises()
    const state = getState(wrapper)
    expect(state.effectiveModule).toBe('reset-pwd')
  })

  it('should compute moduleTitle based on effectiveModule', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.moduleTitle).toBe('page.login.pwdLogin.title')
  })

  it('should keep the system title suffix when login module title updates', async () => {
    const wrapper = mountComponent({ module: 'reset-pwd' })
    await flushPromises()

    expect(hoisted.useTitle).toHaveBeenCalledWith('page.login.resetPwd.title-AetherLink IoT')
    wrapper.unmount()
  })

  it('should compute cardBgColor for light mode', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.cardBgColor).toBe('rgba(255, 255, 255, 0.95)')
  })

  it('should compute borderColor for light mode', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.borderColor).toBe('#e5e7eb')
  })

  it('should handle fetchTenantSetupState error gracefully', async () => {
    hoisted.fetchTenantSetupState.mockRejectedValue(new Error('Network error'))
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.setupState).toEqual({
      has_admin: true,
      has_tenant_admin: true,
      has_tenant: true,
      entry: 'login',
      next_step: 'login'
    })
    expect(state.loading).toBe(false)
  })

  it('should explain tenant-admin setup when backend reports missing tenant', async () => {
    hoisted.fetchTenantSetupState.mockResolvedValue({
      data: {
        has_admin: true,
        has_tenant_admin: false,
        has_tenant: false,
        entry: 'login',
        next_step: 'create_tenant_admin'
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.setupNextStep).toBe('create_tenant_admin')
    expect(state.setupGuide.title).toContain('custom.login.setup.createTenantAdminTitle')
    expect(wrapper.text()).toContain('custom.login.setup.createTenantAdminTitle')
  })

  it('should have correct modules list', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.modules).toHaveLength(6)
  })

  it('should compute localeButtonLabel', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.localeButtonLabel).toBe('CN')
  })

  it('should cycle locale on cycleLocale', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.cycleLocale()

    expect(hoisted.changeLocale).toHaveBeenCalledTimes(1)
    expect(hoisted.changeLocale).toHaveBeenCalledWith('en-US')
  })

  it('should compute activeModule based on effectiveModule', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.activeModule.key).toBe('pwd-login')
  })

  it('should compute activeModuleProps as empty for non-super-admin', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.activeModuleProps).toEqual({})
  })

  it('should normalizeMarketUrl with localhost', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const result = state.normalizeMarketUrl('http://localhost:3000')
    expect(result).toBe(import.meta.env.VITE_MARKET_URL || '')
  })

  it('should ignore relative market register URL', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const result = state.normalizeMarketUrl('/register')
    expect(result).toBe(import.meta.env.VITE_MARKET_URL || '')
  })

  it('should normalizeMarketUrl with valid URL', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const result = state.normalizeMarketUrl('https://example.com')
    expect(result).toBe('https://example.com')
  })
})
