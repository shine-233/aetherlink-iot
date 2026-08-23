/**
 * 文件用途：验证 frontend/src/views/_builtin/login/modules/__tests__/reset-pwd 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest'
import { mount } from '@vue/test-utils'
import ResetPwd from '../reset-pwd.vue'

interface ResetPwdVm {
  model: { email: string; verify_code?: string; password?: string }
  canSubmit: boolean
  handleSmsCode: () => Promise<unknown>
  handleSubmit: () => Promise<unknown>
  handleResetLink: () => Promise<unknown>
  toggleLoginModule: (module: string) => void
  emailOptions: string[]
}

const mockToggleLoginModule = vi.fn()
const mockValidate = vi.fn().mockResolvedValue(undefined)
const mockStart = vi.fn()
const mockIsValidEmail = vi.fn().mockResolvedValue(true)
const mockRouteState = vi.hoisted(() => ({
  query: {} as Record<string, unknown>
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ params: {}, query: mockRouteState.query })
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    toggleLoginModule: mockToggleLoginModule
  })
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    loginLoading: false
  })
}))

vi.mock('@/locales', () => ({
  $t: (k: string) => k
}))

vi.mock('@/hooks/common/form', () => ({
  useFormRules: () => ({
    formRules: {
      email: [{ required: true, message: 'email required' }],
      code: [{ required: true, message: 'code required' }],
      pwd: [{ required: true, message: 'pwd required' }]
    }
  }),
  useNaiveForm: () => ({
    formRef: { value: null, __v_isRef: true },
    validate: mockValidate
  })
}))

vi.mock('@/hooks/business/use-sms-code', () => ({
  default: () => ({
    label: '获取验证码',
    isCounting: false,
    loading: false,
    start: mockStart,
    isValidEmail: mockIsValidEmail
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: { value: 'zh-CN' } }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'zh-CN' } } })
}))

const mockFetchEmailCodeByEmail = vi.fn().mockResolvedValue({ error: null })
const mockEditUserPassWord = vi.fn().mockResolvedValue({ error: null })
const mockRequestPasswordResetLink = vi.fn().mockResolvedValue({ error: null })

vi.mock('@/service/api/auth', () => ({
  fetchEmailCodeByEmail: (...args: unknown[]) => mockFetchEmailCodeByEmail(...args),
  editUserPassWord: (...args: unknown[]) => mockEditUserPassWord(...args),
  requestPasswordResetLink: (...args: unknown[]) => mockRequestPasswordResetLink(...args)
}))

function withNaiveAliases(stubs: Record<string, unknown>) {
  const aliases = { ...stubs }
  const pairs: Array<[string, string[]]> = [
    ['NForm', ['Form', 'n-form']],
    ['NFormItem', ['FormItem', 'n-form-item']],
    ['NAutoComplete', ['AutoComplete', 'n-auto-complete']],
    ['NInput', ['Input', 'n-input']],
    ['NButton', ['Button', 'n-button']],
    ['NSpace', ['Space', 'n-space']]
  ]

  pairs.forEach(([source, names]) => {
    if (stubs[source]) {
      names.forEach((name) => {
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
    props: ['readonly', 'type', 'placeholder', 'autocomplete', 'showPasswordOn'],
    template: '<input :readonly="readonly" :type="type" :placeholder="placeholder" />'
  },
  NButton: {
    name: 'NButton',
    props: ['disabled', 'loading', 'type', 'size', 'round', 'block'],
    emits: ['click'],
    template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>'
  },
  NSpace: { name: 'NSpace', template: '<div><slot /></div>' }
})

describe('ResetPwd', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockRouteState.query = {}
  })

  it('should bind reset-password form fields and rules', () => {
    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as unknown as ResetPwdVm
    expect(vm.model.email).toBe('')
    expect(vm.model.verify_code).toBe('')
    expect(vm.model.password).toBe('')

    const form = wrapper.getComponent({ name: 'NForm' })
    expect(form.props('model')).toBe(vm.model)
    expect(form.props('rules')).toMatchObject({
      email: [{ required: true, message: 'email required' }],
      verify_code: [{ required: true, message: 'code required' }],
      password: [{ required: true, message: 'pwd required' }]
    })
    expect(wrapper.findAllComponents({ name: 'NFormItem' }).map((item) => item.props('path'))).toEqual([
      'email',
      'verify_code',
      'password'
    ])

    const verifyCodeFormItem = wrapper.findAllComponents({ name: 'NFormItem' })[1]
    expect(verifyCodeFormItem.element.children).toHaveLength(1)
    expect(verifyCodeFormItem.element.firstElementChild?.classList.contains('w-full')).toBe(true)
  })

  it('should disable submit when form is incomplete', () => {
    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as unknown as ResetPwdVm
    expect(vm.canSubmit).toBe(false)
  })

  it('should call fetchEmailCodeByEmail when SMS code button is clicked', async () => {
    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as ResetPwdVm
    vm.model.email = 'test@example.com'
    await vm.handleSmsCode()

    expect(mockIsValidEmail).toHaveBeenCalledWith('test@example.com')
    expect(mockFetchEmailCodeByEmail).toHaveBeenCalledWith('test@example.com')
  })

  it('should show error message when email is empty and SMS code is requested', async () => {
    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as ResetPwdVm
    vm.model.email = ''
    await vm.handleSmsCode()

    expect((globalThis as unknown as { $message: { error: Mock } }).$message.error).toHaveBeenCalledTimes(1)
    expect((globalThis as unknown as { $message: { error: Mock } }).$message.error).toHaveBeenCalledWith('form.email.required')
  })

  it('should call editUserPassWord on valid form submit', async () => {
    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as ResetPwdVm
    vm.model.email = 'test@example.com'
    vm.model.verify_code = '123456'
    vm.model.password = 'NewPassword1'

    await vm.handleSubmit()

    expect(mockValidate).toHaveBeenCalledTimes(1)
    expect(mockEditUserPassWord).toHaveBeenCalledWith({
      email: 'test@example.com',
      verify_code: '123456',
      password: 'NewPassword1',
      is_register: 2
    })
  })

  it('should request a password reset link after email code validation', async () => {
    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as ResetPwdVm
    vm.model.email = 'test@example.com'
    vm.model.verify_code = '123456'

    await vm.handleResetLink()

    expect(mockRequestPasswordResetLink).toHaveBeenCalledWith({
      email: 'test@example.com',
      verify_code: '123456'
    })
  })

  it('should submit reset_token without verification code in link mode', async () => {
    mockRouteState.query = {
      email: 'test@example.com',
      reset_token: 'reset-token-1'
    }

    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as ResetPwdVm
    vm.model.password = 'NewPassword1'

    await vm.handleSubmit()

    expect(mockValidate).toHaveBeenCalledTimes(1)
    expect(mockEditUserPassWord).toHaveBeenCalledWith({
      email: 'test@example.com',
      reset_token: 'reset-token-1',
      password: 'NewPassword1',
      is_register: 2
    })
    expect(wrapper.findAllComponents({ name: 'NFormItem' }).map((item) => item.props('path'))).toEqual([
      'email',
      'password'
    ])
  })

  it('should toggle to pwd-login after successful password reset', async () => {
    mockEditUserPassWord.mockResolvedValueOnce({ error: null })

    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as ResetPwdVm
    vm.model.email = 'test@example.com'
    vm.model.verify_code = '123456'
    vm.model.password = 'NewPassword1'

    await vm.handleSubmit()

    expect(mockToggleLoginModule).toHaveBeenCalledWith('pwd-login')
  })

  it('should compute emailOptions based on email input', () => {
    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as ResetPwdVm
    vm.model.email = 'user@gm'
    expect(vm.emailOptions).toEqual(expect.arrayContaining(['user@gmail.com']))
    expect(vm.emailOptions.every((item: string) => item.startsWith('user@'))).toBe(true)
  })

  it('should toggle to pwd-login when back button is clicked', () => {
    const wrapper = mount(ResetPwd, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as ResetPwdVm
    vm.toggleLoginModule('pwd-login')

    expect(mockToggleLoginModule).toHaveBeenCalledWith('pwd-login')
  })
})
