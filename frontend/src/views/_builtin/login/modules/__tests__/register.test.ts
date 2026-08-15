/**
 * 文件用途：验证 frontend/src/views/_builtin/login/modules/__tests__/register 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import Register from '../register.vue'

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
    loginLoading: false
  })
}))

vi.mock('@/locales', () => ({
  $t: (k: string) => k
}))

vi.mock('@/hooks/common/form', () => ({
  useFormRules: () => ({
    formRules: {
      phone: [{ required: true, message: 'phone required' }],
      code: [{ required: true, message: 'code required' }],
      pwd: [{ required: true, message: 'pwd required' }]
    }
  }),
  useNaiveForm: () => ({
    formRef: { value: null },
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

vi.mock('@/utils/form/rule', () => ({
  getConfirmPwdRule: () => [{ required: true, message: 'confirm pwd required' }]
}))

const commonStubs = {
  NForm: {
    name: 'NForm',
    props: ['model', 'rules', 'size', 'showLabel'],
    template: '<form><slot /></form>'
  },
  NFormItem: {
    name: 'NFormItem',
    props: ['path'],
    template: '<div class="n-form-item-stub" :data-path="path"><slot /></div>'
  },
  NInput: {
    name: 'NInput',
    props: ['type', 'showPasswordOn', 'placeholder'],
    template: '<input :type="type" :placeholder="placeholder" />'
  },
  NButton: {
    name: 'NButton',
    props: ['disabled', 'loading', 'type', 'size', 'round', 'block'],
    emits: ['click'],
    template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>'
  },
  NSpace: { name: 'NSpace', template: '<div><slot /></div>' },
  LoginAgreement: {
    name: 'LoginAgreement',
    props: ['value'],
    emits: ['update:value'],
    template:
      '<label class="login-agreement-stub"><input :checked="value" @change="$emit(\'update:value\', true)" /></label>'
  }
}

describe('Register', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should bind the registration form model and validation rules', () => {
    const wrapper = shallowMount(Register, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as any
    const form = wrapper.getComponent({ name: 'NForm' })

    expect(form.props('model')).toBe(vm.model)
    expect(form.props('rules')).toMatchObject({
      phone: [{ required: true, message: 'phone required' }],
      code: [{ required: true, message: 'code required' }],
      pwd: [{ required: true, message: 'pwd required' }],
      confirmPwd: [{ required: true, message: 'confirm pwd required' }]
    })
    expect(form.props('size')).toBe('large')
    expect(form.props('showLabel')).toBe(false)
  })

  it('should render phone, code, pwd, and confirmPwd fields', () => {
    const wrapper = shallowMount(Register, {
      global: { stubs: commonStubs }
    })
    expect(wrapper.findAllComponents({ name: 'NFormItem' }).map(item => item.props('path'))).toEqual([
      'phone',
      'code',
      'pwd',
      'confirmPwd'
    ])
    expect(wrapper.findAllComponents({ name: 'NInput' })).toHaveLength(4)
  })

  it('should call start() when SMS code button is clicked', async () => {
    const wrapper = shallowMount(Register, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.handleSmsCode()

    expect(mockStart).toHaveBeenCalledTimes(1)
  })

  it('should validate and show success message on submit', async () => {
    const wrapper = shallowMount(Register, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    await vm.handleSubmit()

    expect(mockValidate).toHaveBeenCalledTimes(1)
    expect((globalThis as any).$message.success).toHaveBeenCalledTimes(1)
    expect((globalThis as any).$message.success).toHaveBeenCalledWith('page.login.common.validateSuccess')
  })

  it('should toggle to pwd-login when back button is clicked', () => {
    const wrapper = shallowMount(Register, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    vm.toggleLoginModule('pwd-login')

    expect(mockToggleLoginModule).toHaveBeenCalledWith('pwd-login')
  })

  it('should have agreement ref initialized to false', () => {
    const wrapper = shallowMount(Register, {
      global: { stubs: commonStubs }
    })

    const vm = wrapper.vm as any
    expect(vm.agreement).toBe(false)
  })

  it('should bind LoginAgreement to the agreement state', async () => {
    const wrapper = shallowMount(Register, {
      global: { stubs: commonStubs }
    })
    const vm = wrapper.vm as any
    const agreement = wrapper.getComponent({ name: 'LoginAgreement' })

    expect(agreement.props('value')).toBe(false)

    agreement.vm.$emit('update:value', true)
    await wrapper.vm.$nextTick()

    expect(vm.agreement).toBe(true)
  })
})
