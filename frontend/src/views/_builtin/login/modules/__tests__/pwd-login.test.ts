/**
 * 文件用途：验证 frontend/src/views/_builtin/login/modules/__tests__/pwd-login 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { ref } from 'vue'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PwdLogin from '../pwd-login.vue'

const mockLogin = vi.fn().mockResolvedValue(undefined)
const mockToggleLoginModule = vi.fn()
const mockValidate = vi.fn().mockResolvedValue(undefined)

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ params: {}, query: {} })
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    toggleLoginModule: mockToggleLoginModule
  })
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    loginLoading: false,
    login: mockLogin
  })
}))

vi.mock('@/locales', () => ({
  $t: (k: string) => k
}))

vi.mock('@/hooks/common/form', () => ({
  useFormRules: () => ({
    formRules: {
      email: [{ required: true, message: 'email required' }],
      pwd: [{ required: true, message: 'pwd required' }]
    }
  }),
  useNaiveForm: () => ({
    formRef: ref(null),
    validate: mockValidate
  })
}))

vi.mock('@/constants/app', () => ({
  loginModuleRecord: {
    'pwd-login': 'page.login.pwdLogin.title',
    register: 'page.login.register.title',
    'reset-pwd': 'page.login.resetPwd.title'
  }
}))

const mockGetFunction = vi.fn().mockResolvedValue({
  data: [
    { name: 'enable_reg', enable_flag: 'enable' },
    { name: 'use_captcha', enable_flag: 'disable' }
  ]
})

vi.mock('@/service/api/setting', () => ({
  getFunction: () => mockGetFunction()
}))

// 登录页挂载时会拉取 SSO 提供方列表；不 mock 则请求层在测试环境 reject，
// 组件虽有降级 catch，仍以确定性 mock 保证用例可复现（空列表 = 无 SSO 入口）。
vi.mock('@/service/api/auth', () => ({
  fetchSsoProviders: vi.fn().mockResolvedValue({ data: [], error: null })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: { value: 'zh-CN' } }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'zh-CN' } } })
}))

function withNaiveAliases(stubs: Record<string, any>) {
  const aliases = { ...stubs }
  const pairs: Array<[string, string[]]> = [
    ['NForm', ['Form', 'n-form']],
    ['NFormItem', ['FormItem', 'n-form-item']],
    ['NAutoComplete', ['AutoComplete', 'n-auto-complete']],
    ['NInput', ['Input', 'n-input']],
    ['NButton', ['Button', 'n-button']],
    ['NSpace', ['Space', 'n-space']],
    ['NCheckbox', ['Checkbox', 'n-checkbox']],
    ['NDivider', ['Divider', 'n-divider']]
  ]

  pairs.forEach(([source, names]) => {
    if (stubs[source]) {
      names.forEach(name => {
        aliases[name] = stubs[source]
      })
    }
  })

  return aliases
}

const commonStubs = withNaiveAliases({
  NForm: { name: 'NForm', template: '<form><slot /></form>' },
  NFormItem: { name: 'NFormItem', template: '<div><slot /></div>' },
  NAutoComplete: { name: 'NAutoComplete', template: '<input />' },
  NInput: { name: 'NInput', template: '<input />' },
  NButton: { name: 'NButton', template: '<button><slot /></button>' },
  NSpace: { name: 'NSpace', template: '<div><slot /></div>' },
  NCheckbox: { name: 'NCheckbox', template: '<input type="checkbox" />' },
  NDivider: { name: 'NDivider', template: '<hr />' }
})

describe('PwdLogin', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should call authStore.login on valid form submit', async () => {
    const wrapper = mount(PwdLogin, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.model.userName = 'admin@test.com'
    vm.model.password = 'Password123'

    await vm.handleSubmit()

    expect(mockValidate).toHaveBeenCalledTimes(1)
    expect(mockLogin).toHaveBeenCalledWith('admin@test.com', 'Password123')
  })

  it('should save credentials to localStorage when rememberMe is true', async () => {
    const wrapper = mount(PwdLogin, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.model.userName = 'admin@test.com'
    vm.model.password = 'Password123'
    vm.rememberMe = true

    await vm.handleSubmit()

    expect(localStorage.getItem('rememberMe')).toBe('true')
    expect(localStorage.getItem('rememberedUserName')).toBe('admin@test.com')
  })

  it('should clear credentials from localStorage when rememberMe is false', async () => {
    localStorage.setItem('rememberMe', 'true')
    localStorage.setItem('rememberedUserName', 'old@test.com')

    const wrapper = mount(PwdLogin, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.model.userName = 'admin@test.com'
    vm.model.password = 'Password123'
    vm.rememberMe = false

    await vm.handleSubmit()

    expect(localStorage.getItem('rememberMe')).toBeNull()
    expect(localStorage.getItem('rememberedUserName')).toBeNull()
  })

  it('should call getFunction on mount', async () => {
    const wrapper = mount(PwdLogin, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as any

    await flushPromises()

    expect(mockGetFunction).toHaveBeenCalledTimes(1)
    expect(localStorage.getItem('enableZcAndYzm')).toContain('enable_reg')
    expect(vm.showZc).toBe(true)
    expect(vm.showYzm).toBe(false)
  })

  it('should compute emailOptions based on userName input', () => {
    const wrapper = mount(PwdLogin, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.model.userName = 'test@q'
    expect(vm.emailOptions).toEqual(expect.arrayContaining(['test@qq.com']))
    expect(vm.emailOptions.every((item: string) => item.startsWith('test@'))).toBe(true)
  })

  it('should load saved credentials from localStorage on mount', () => {
    localStorage.setItem('rememberMe', 'true')
    localStorage.setItem('rememberedUserName', 'saved@test.com')

    const wrapper = mount(PwdLogin, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    expect(vm.model.userName).toBe('saved@test.com')
    expect(vm.rememberMe).toBe(true)
  })
})
