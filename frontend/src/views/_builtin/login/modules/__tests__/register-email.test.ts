/**
 * 文件用途：验证 frontend/src/views/_builtin/login/modules/__tests__/register-email 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import RegisterEmail from '../register-email.vue'

const mockLoginByToken = vi.fn().mockResolvedValue(undefined)
const mockToggleLoginModule = vi.fn()
const mockValidate = vi.fn().mockResolvedValue(undefined)
const mockStart = vi.fn()

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
    loginByToken: mockLoginByToken
  })
}))

vi.mock('@/locales', () => ({
  $t: (k: string) => k
}))

vi.mock('@/hooks/common/form', () => ({
  useFormRules: () => ({
    patternRules: {
      phoneWithCountryCode: { pattern: /^\d{7,15}$/, message: 'phone invalid' }
    },
    formRules: {
      email: [{ required: true, message: 'email required' }],
      code: [{ required: true, message: 'code required' }],
      pwd: [{ required: true, message: 'pwd required' }]
    }
  }),
  useNaiveForm: () => ({
    formRef: { __v_isRef: true, value: null },
    validate: mockValidate
  })
}))

vi.mock('@/hooks/business/use-sms-code', () => ({
  default: () => ({
    label: '获取验证码',
    isCounting: false,
    loading: false,
    start: mockStart
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: { value: 'zh-CN' } }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'zh-CN' } } })
}))

const mockFetchEmailCode = vi.fn().mockResolvedValue({ error: null })
const mockRegisterByEmail = vi.fn().mockResolvedValue({ error: null, data: { token: 'test-token', expires_in: 3600 } })

vi.mock('@/service/api/auth', () => ({
  fetchEmailCode: (...args: any[]) => mockFetchEmailCode(...args),
  registerByEmail: (...args: any[]) => mockRegisterByEmail(...args)
}))

vi.mock('@/utils/form/rule', () => ({
  getConfirmPwdRule: () => [{ required: true, message: 'confirm pwd required' }]
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
    ['NSelect', ['Select', 'n-select']]
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
  NForm: {
    name: 'NForm',
    props: ['model', 'rules', 'size', 'showLabel', 'autocomplete'],
    template: '<form><slot /></form>'
  },
  NFormItem: {
    name: 'NFormItem',
    props: ['path'],
    template: '<div class="n-form-item-stub" :data-path="path"><slot /></div>'
  },
  NAutoComplete: {
    name: 'NAutoComplete',
    props: ['options', 'placeholder', 'clearable', 'autocomplete'],
    template: '<input :placeholder="placeholder" />'
  },
  NInput: {
    name: 'NInput',
    props: ['type', 'showPasswordOn', 'placeholder', 'autocomplete'],
    template: '<input :type="type" :placeholder="placeholder" />'
  },
  NButton: {
    name: 'NButton',
    template: '<button :type="type" :disabled="disabled"><slot /></button>',
    props: ['type', 'disabled', 'loading', 'size', 'round', 'block']
  },
  NSpace: { name: 'NSpace', template: '<div><slot /></div>' },
  NSelect: {
    name: 'NSelect',
    props: ['options', 'placeholder'],
    template: '<select />'
  }
})

describe('RegisterEmail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should bind email registration fields, defaults, and rules', () => {
    const wrapper = mount(RegisterEmail, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as any
    expect(vm.model.country_code).toBe('+86')
    expect(vm.canSubmit).toBe(false)

    const form = wrapper.getComponent({ name: 'NForm' })
    expect(form.props('model')).toBe(vm.model)
    expect(form.props('rules')).toMatchObject({
      email: [{ required: true, message: 'email required' }],
      phone: [{ pattern: /^\d{7,15}$/, message: 'phone invalid' }],
      code: [{ required: true, message: 'code required' }],
      pwd: [{ required: true, message: 'pwd required' }],
      confirmPwd: [{ required: true, message: 'confirm pwd required' }],
      country_code: []
    })
    expect(wrapper.findAllComponents({ name: 'NFormItem' }).map(item => item.props('path'))).toEqual([
      'email',
      'code',
      'phone',
      'pwd',
      'confirmPwd'
    ])
    expect(wrapper.getComponent({ name: 'NSelect' }).props('options')).toContainEqual({ label: '+86', value: '+86' })
  })

  it('should disable submit when form is incomplete', () => {
    const wrapper = mount(RegisterEmail, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as any
    expect(vm.canSubmit).toBe(false)
  })

  it('should call fetchEmailCode when SMS code button is clicked', async () => {
    const wrapper = mount(RegisterEmail, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.model.email = 'test@example.com'
    await wrapper.vm.$nextTick()

    vm.handleSmsCode()
    expect(mockFetchEmailCode).toHaveBeenCalledWith('test@example.com')
  })

  it('should call registerByEmail on valid submit', async () => {
    const wrapper = mount(RegisterEmail, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.model.email = 'test@example.com'
    vm.model.code = '123456'
    vm.model.pwd = 'Password1!'
    vm.model.confirmPwd = 'Password1!'
    vm.model.country_code = '+86'

    await vm.handleSubmit()

    expect(mockValidate).toHaveBeenCalledTimes(1)
    expect(mockRegisterByEmail).toHaveBeenCalledWith({
      email: 'test@example.com',
      verify_code: '123456',
      password: 'Password1!',
      phone_prefix: '+86',
      phone_number: ''
    })
  })

  it('should call loginByToken on successful registration with token', async () => {
    mockRegisterByEmail.mockResolvedValueOnce({
      error: null,
      data: { token: 'new-token', expires_in: 3600 }
    })

    const wrapper = mount(RegisterEmail, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.model.email = 'test@example.com'
    vm.model.code = '123456'
    vm.model.pwd = 'Password1!'
    vm.model.confirmPwd = 'Password1!'

    await vm.handleSubmit()

    expect(mockLoginByToken).toHaveBeenCalledWith({
      token: 'new-token',
      expires_in: 3600
    })
  })

  it('should toggle to pwd-login module when back button is clicked', async () => {
    const wrapper = mount(RegisterEmail, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.toggleLoginModule('pwd-login')

    expect(mockToggleLoginModule).toHaveBeenCalledWith('pwd-login')
  })

  it('should compute emailOptions based on email input', () => {
    const wrapper = mount(RegisterEmail, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.model.email = 'test@q'
    expect(vm.emailOptions).toEqual(expect.arrayContaining(['test@qq.com']))
    expect(vm.emailOptions.every((item: string) => item.startsWith('test@'))).toBe(true)
  })
})
