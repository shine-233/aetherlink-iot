/**
 * 文件用途：验证 frontend/src/views/_builtin/login/modules/__tests__/register-super-admin 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'

interface RegisterSuperAdminVm {
  model: { email: string; pwd: string }
  emailLocked: boolean
  canSubmit: boolean
  handleSubmit: () => Promise<unknown>
}
import { mount } from '@vue/test-utils'
import RegisterSuperAdmin from '../register-super-admin.vue'

const mockLoginByToken = vi.fn().mockResolvedValue(undefined)
const mockValidate = vi.fn().mockResolvedValue(undefined)

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ params: {}, query: {} })
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
    formRules: {
      email: [{ required: true, message: 'email required' }],
      pwd: [{ required: true, message: 'pwd required' }]
    }
  }),
  useNaiveForm: () => ({
    formRef: { value: null },
    validate: mockValidate
  })
}))

const mockFetchSuperAdminInit = vi.fn()
vi.mock('@/service/api/auth', () => ({
  fetchSuperAdminInit: (...args: unknown[]) => mockFetchSuperAdminInit(...args)
}))

function withNaiveAliases(stubs: Record<string, unknown>) {
  const aliases = { ...stubs }
  const pairs: Array<[string, string[]]> = [
    ['NForm', ['Form', 'n-form']],
    ['NFormItem', ['FormItem', 'n-form-item']],
    ['NAutoComplete', ['AutoComplete', 'n-auto-complete']],
    ['NInput', ['Input', 'n-input']],
    ['NButton', ['Button', 'n-button']]
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
    template: '<input :disabled="disabled" />',
    props: ['disabled', 'options', 'placeholder', 'clearable', 'autocomplete']
  },
  NInput: {
    name: 'NInput',
    props: ['type', 'showPasswordOn', 'placeholder', 'autocomplete'],
    template: '<input :type="type" :placeholder="placeholder" />'
  },
  NButton: {
    name: 'NButton',
    props: ['disabled', 'loading', 'type', 'size', 'round', 'block'],
    emits: ['click'],
    template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>'
  }
})

describe('RegisterSuperAdmin', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    mockFetchSuperAdminInit.mockResolvedValue({ error: null, data: { token: 'test-token', expires_in: 3600 } })
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('should bind the super-admin init form fields and rules', () => {
    const wrapper = mount(RegisterSuperAdmin, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as unknown as RegisterSuperAdminVm
    expect(vm.model.email).toBe('')
    expect(vm.model.pwd).toBe('')

    const form = wrapper.getComponent({ name: 'NForm' })
    expect(form.props('model')).toBe(vm.model)
    expect(form.props('rules')).toMatchObject({
      email: [{ required: true, message: 'email required' }],
      pwd: [{ required: true, message: 'pwd required' }]
    })
    expect(wrapper.findAllComponents({ name: 'NFormItem' }).map(item => item.props('path'))).toEqual(['email', 'pwd'])
  })

  it('should disable email field when marketEmail prop is provided', () => {
    const wrapper = mount(RegisterSuperAdmin, {
      props: { marketEmail: 'admin@test.com' },
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as unknown as RegisterSuperAdminVm
    expect(vm.emailLocked).toBe(true)
    expect(vm.model.email).toBe('admin@test.com')
  })

  it('should enable email field when marketEmail prop is empty', () => {
    const wrapper = mount(RegisterSuperAdmin, {
      props: { marketEmail: '' },
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as unknown as RegisterSuperAdminVm
    expect(vm.emailLocked).toBe(false)
    expect(vm.model.email).toBe('')
  })

  it('should disable submit button when form is incomplete', () => {
    const wrapper = mount(RegisterSuperAdmin, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as unknown as RegisterSuperAdminVm
    expect(vm.canSubmit).toBe(false)
  })

  it('should submit a trimmed market registration payload for local initialization', async () => {
    const wrapper = mount(RegisterSuperAdmin, {
      props: { marketEmail: ' admin@test.com ', marketRegistered: true, marketSource: 'market-app' },
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as RegisterSuperAdminVm
    vm.model.pwd = 'InitPass1'
    await vm.handleSubmit()

    expect(mockValidate).toHaveBeenCalledTimes(1)
    expect(mockFetchSuperAdminInit).toHaveBeenCalledWith({
      email: 'admin@test.com',
      password: 'InitPass1',
      market_registered: true,
      market_email: 'admin@test.com',
      market_source: 'market-app'
    })
  })

  it('should call loginByToken on successful registration', async () => {
    mockFetchSuperAdminInit.mockResolvedValue({
      error: null,
      data: { token: 'test-token', expires_in: 3600 }
    })

    const wrapper = mount(RegisterSuperAdmin, {
      props: { marketEmail: 'admin@test.com' },
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as RegisterSuperAdminVm
    await vm.handleSubmit()

    expect(mockLoginByToken).toHaveBeenCalledWith({
      token: 'test-token',
      expires_in: 3600
    })
  })

  it('should handle error with code 200055', async () => {
    mockFetchSuperAdminInit.mockResolvedValue({
      error: { code: 200055 },
      data: null
    })

    const wrapper = mount(RegisterSuperAdmin, {
      props: { marketEmail: 'admin@test.com' },
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as RegisterSuperAdminVm
    await vm.handleSubmit()

    expect((globalThis as unknown as { $message: { warning: Mock } }).$message.warning).toHaveBeenCalledTimes(1)
    expect((globalThis as unknown as { $message: { warning: Mock } }).$message.warning).toHaveBeenCalledWith('custom.login.registerSuperAdmin.marketRegistrationRequired')
  })

  it('should prefill email from marketEmail prop on mount', () => {
    const wrapper = mount(RegisterSuperAdmin, {
      props: { marketEmail: 'prefilled@test.com' },
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as unknown as RegisterSuperAdminVm
    expect(vm.model.email).toBe('prefilled@test.com')
  })
})
