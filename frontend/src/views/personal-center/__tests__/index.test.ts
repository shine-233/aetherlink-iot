/**
 * 文件用途：验证 frontend/src/views/personal-center/__tests__/index 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchUserInfo: vi.fn(),
  changeInformation: vi.fn(),
  passwordModification: vi.fn(),
  changeAccountEmail: vi.fn(),
  fetchEmailCodeByEmail: vi.fn(),
  changeLocale: vi.fn(),
  localStgSet: vi.fn(),
  authUserInfo: {
    email: 'test@test.com',
    userEmail: 'test@test.com',
    name: 'Test',
    userName: 'Test',
    default_language: 'en-US',
    additional_info: '{}',
    avatar_url: ''
  }
}))

vi.mock('@/service/api/personal-center', () => ({
  fetchUserInfo: hoisted.fetchUserInfo,
  changeInformation: hoisted.changeInformation,
  passwordModification: hoisted.passwordModification,
  changeAccountEmail: hoisted.changeAccountEmail
}))

vi.mock('@/service/api/auth', () => ({
  fetchEmailCodeByEmail: hoisted.fetchEmailCodeByEmail
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: vi.fn(() => 'fake-token'),
    set: hoisted.localStgSet
  }
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: vi.fn(() => ({
    locale: 'en-US',
    localeOptions: [
      { label: 'Chinese', key: 'zh-CN' },
      { label: 'English', key: 'en-US' },
      { label: 'Francais', key: 'fr-FR' },
      { label: 'Espanol', key: 'es-ES' }
    ],
    changeLocale: hoisted.changeLocale
  }))
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: vi.fn(() => ({
    userInfo: hoisted.authUserInfo
  }))
}))

vi.mock('@/hooks/common/form', () => ({
  useNaiveForm: () => ({
    formRef: { value: null },
    validate: vi.fn(() => Promise.resolve())
  })
}))

vi.mock('@/utils/form/rule', () => ({
  getConfirmPwdRule: () => []
}))

vi.mock('@/utils/common/tool', () => ({
  generateRandomHexString: vi.fn(() => 'randomhex'),
  getPlatformApiBaseUrl: vi.fn(() => 'http://localhost/api/v1'),
  validName: vi.fn(() => true),
  validPasswordByExp: vi.fn(() => true)
}))

vi.mock('@/utils/security/rsa-encrypt', () => ({
  encryptDataByRsa: vi.fn((v: string) => `encrypted-${v}`)
}))

vi.mock('~/env.config', () => ({
  createProxyPattern: vi.fn(() => '/api'),
  createServiceConfig: vi.fn(() => ({
    otherBaseURL: {
      platform: 'http://localhost/api/v1'
    }
  }))
}))

vi.mock('@/components/common/ProvinceCityDistrictSelector.vue', () => ({
  default: defineComponent({
    props: ['province', 'city', 'district'],
    emits: ['change'],
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  })
}))

vi.mock('@/views/management/setting/components/warning-email-setting.vue', () => ({
  default: defineComponent({
    setup() {
      return () => h('div')
    }
  })
}))

import PersonalCenter from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(PersonalCenter, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NDivider: defineComponent({
          props: ['style', 'titlePlacement'],
          setup(_, { slots }) {
            return () => h('hr', slots.default?.())
          }
        }),
        NButton: defineComponent({
          props: ['type', 'size', 'loading', 'title'],
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NUpload: defineComponent({
          props: ['action', 'showFileList', 'headers', 'data'],
          emits: ['finish'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NAvatar: defineComponent({
          props: ['src', 'round', 'class'],
          setup() {
            return () => h('div')
          }
        }),
        NForm: defineComponent({
          props: ['model', 'rules', 'labelPlacement', 'labelAlign', 'labelWidth'],
          setup(_, { slots }) {
            return () => h('form', slots.default?.())
          }
        }),
        NFormItem: defineComponent({
          props: ['path', 'label'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NInput: defineComponent({
          props: ['value', 'placeholder', 'type', 'showPasswordOn', 'disabled', 'class'],
          emits: ['update:value'],
          setup(_, { slots }) {
            return () => h('input', slots.default?.())
          }
        }),
        NSelect: defineComponent({
          props: ['value', 'options', 'placeholder', 'class'],
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NSpace: defineComponent({
          props: ['vertical', 'size'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NModal: defineComponent({
          props: ['show', 'preset', 'title', 'class'],
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NAlert: defineComponent({
          props: ['type', 'showIcon'],
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

describe('PersonalCenter', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.authUserInfo.email = 'test@test.com'
    hoisted.authUserInfo.userEmail = 'test@test.com'
    hoisted.authUserInfo.name = 'Test'
    hoisted.authUserInfo.userName = 'Test'
    hoisted.authUserInfo.default_language = 'en-US'
    hoisted.authUserInfo.additional_info = '{}'
    hoisted.authUserInfo.avatar_url = ''
    hoisted.fetchUserInfo.mockResolvedValue({
      data: {
        name: 'Test User',
        email: 'test@test.com',
        phone_number: '+8613800138000',
        authority: 'admin',
        additional_info: '{}',
        organization: '',
        timezone: '',
        default_language: '',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and fetch user info', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.fetchUserInfo).toHaveBeenCalledTimes(1)
    expect(wrapper.vm.userInfoData.name).toBe('Test User')
  })

  it('should normalize fetched preferred language codes', async () => {
    hoisted.fetchUserInfo.mockResolvedValue({
      data: {
        name: 'Test User',
        email: 'test@test.com',
        phone_number: '+8613800138000',
        authority: 'admin',
        additional_info: '{}',
        organization: '',
        timezone: '',
        default_language: 'fr_fr',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.vm.userInfoData.default_language).toBe('fr-FR')
  })

  it('should enter edit mode', () => {
    const wrapper = mountComponent()
    wrapper.vm.editName()
    expect(wrapper.vm.editType).toBe(true)
  })

  it('should exit edit mode', () => {
    const wrapper = mountComponent()
    wrapper.vm.editName()
    wrapper.vm.closeEdit()
    expect(wrapper.vm.editType).toBe(false)
  })

  it('should update user info', async () => {
    hoisted.changeInformation.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.userInfoData.name = 'Updated User'
    await wrapper.vm.updataUserInfo()
    expect(hoisted.changeInformation).toHaveBeenCalledTimes(1)
    expect(hoisted.changeInformation).toHaveBeenCalledWith({
      additional_info: '{}',
      name: 'Updated User',
      email: 'test@test.com',
      phone_number: '+8613800138000',
      authority: 'admin',
      avatar_url: '',
      organization: '',
      timezone: '',
      default_language: '',
      address: { province: '', city: '', district: '', detailed_address: '' }
    })
    expect(hoisted.authUserInfo.name).toBe('Updated User')
    expect(hoisted.authUserInfo.userName).toBe('Updated User')
    expect(wrapper.vm.editType).toBe(false)
  })

  it('should switch locale after saving a new preferred language', async () => {
    hoisted.changeInformation.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.userInfoData.default_language = 'fr-FR'

    await wrapper.vm.updataUserInfo()

    expect(hoisted.changeLocale).toHaveBeenCalledWith('fr-FR', { persistRemote: false })
    expect(hoisted.localStgSet).toHaveBeenCalled()
  })

  it('should reset password form', async () => {
    const wrapper = mountComponent()
    wrapper.vm.formData.old_password = 'old'
    wrapper.vm.formData.password = 'new'
    wrapper.vm.formData.passwords = 'new'
    await wrapper.vm.resetPass()
    expect(wrapper.vm.formData.old_password).toBe('')
    expect(wrapper.vm.formData.password).toBe('')
    expect(wrapper.vm.formData.passwords).toBe('')
  })

  it('should submit password change', async () => {
    hoisted.passwordModification.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.formData.old_password = 'oldpass'
    wrapper.vm.formData.password = 'newpassword1'
    wrapper.vm.formData.passwords = 'newpassword1'
    await wrapper.vm.submitPass()
    expect(hoisted.passwordModification).toHaveBeenCalledTimes(1)
    expect(hoisted.passwordModification).toHaveBeenCalledWith({
      old_password: 'oldpass',
      password: 'newpassword1',
      salt: null
    })
  })

  it('should open email change modal', () => {
    const wrapper = mountComponent()
    wrapper.vm.openEmailChangeModal()
    expect(wrapper.vm.emailModalVisible).toBe(true)
    expect(wrapper.vm.emailChangeForm.new_email).toBe('')
    expect(wrapper.vm.emailChangeForm.verify_code).toBe('')
  })

  it('should send email change code', async () => {
    hoisted.fetchEmailCodeByEmail.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.emailChangeForm.new_email = 'new@test.com'
    await wrapper.vm.sendEmailChangeCode()
    expect(hoisted.fetchEmailCodeByEmail).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchEmailCodeByEmail).toHaveBeenCalledWith('test@test.com')
    expect(wrapper.vm.emailCodeCounting).toBe(true)
    expect(String(wrapper.vm.emailCodeLabel)).toContain('60')
  })

  it('should not send email code without new email', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.emailChangeForm.new_email = ''
    await wrapper.vm.sendEmailChangeCode()
    expect(hoisted.fetchEmailCodeByEmail).toHaveBeenCalledTimes(0)
  })

  it('should block email code when new email matches current email', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.emailChangeForm.new_email = 'test@test.com'
    await wrapper.vm.sendEmailChangeCode()
    expect(hoisted.fetchEmailCodeByEmail).toHaveBeenCalledTimes(0)
  })

  it('should submit email change', async () => {
    hoisted.changeAccountEmail.mockResolvedValue({ error: null, data: { new_email: 'changed@test.com', devices_migrated: 3 } })
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.emailChangeForm.new_email = 'changed@test.com'
    wrapper.vm.emailChangeForm.verify_code = '123456'
    await wrapper.vm.submitEmailChange()
    expect(hoisted.changeAccountEmail).toHaveBeenCalledWith({ new_email: 'changed@test.com', verify_code: '123456' })
  })

  it('should not submit email change without required fields', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.emailChangeForm.new_email = ''
    wrapper.vm.emailChangeForm.verify_code = ''
    await wrapper.vm.submitEmailChange()
    expect(hoisted.changeAccountEmail).toHaveBeenCalledTimes(0)
    expect(wrapper.vm.emailModalVisible).toBe(false)
  })

  it('should block email change when new email matches current email', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    wrapper.vm.emailChangeForm.new_email = 'test@test.com'
    wrapper.vm.emailChangeForm.verify_code = '123456'
    await wrapper.vm.submitEmailChange()
    expect(hoisted.changeAccountEmail).toHaveBeenCalledTimes(0)
  })

  it('should handle address change', () => {
    const wrapper = mountComponent()
    wrapper.vm.handleAddressChange({ province: 'Beijing', city: 'Beijing', district: 'Haidian' })
    expect(wrapper.vm.userInfoData.address.province).toBe('Beijing')
    expect(wrapper.vm.userInfoData.address.city).toBe('Beijing')
    expect(wrapper.vm.userInfoData.address.district).toBe('Haidian')
  })

  it('should parse phone number with country code', async () => {
    hoisted.fetchUserInfo.mockResolvedValue({
      data: {
        name: 'Test',
        email: 'test@test.com',
        phone_number: '+8613800138000',
        authority: '',
        additional_info: '{}',
        organization: '',
        timezone: '',
        default_language: '',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.vm.userInfoData.country_code).toBe('+86')
    expect(wrapper.vm.userInfoData.phone_only).toBe('13800138000')
  })

  it('should compute displayPhoneNumber', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.vm.displayPhoneNumber).toContain('+86')
  })

  it('should compute fullPhoneNumber', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.vm.fullPhoneNumber).toContain('+86')
  })

  it('should handle upload finish', async () => {
    hoisted.changeInformation.mockResolvedValue({ error: null })
    hoisted.fetchUserInfo.mockResolvedValue({
      data: {
        name: 'Test',
        email: 'test@test.com',
        phone_number: '+8613800138000',
        authority: '',
        additional_info: '{}',
        organization: '',
        timezone: '',
        default_language: '',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const mockEvent = { target: { response: JSON.stringify({ data: { path: '/uploads/avatar.png' } }) } }
    await wrapper.vm.handleFinish({ event: mockEvent as any })
    expect(hoisted.changeInformation).toHaveBeenCalledTimes(1)
    expect(hoisted.changeInformation).toHaveBeenCalledWith({
      additional_info: '{"user_icon":"/uploads/avatar.png"}',
      name: 'Test',
      email: 'test@test.com',
      phone_number: '+8613800138000',
      authority: '',
      organization: '',
      timezone: '',
      default_language: '',
      avatar_url: '/uploads/avatar.png',
      address: { province: '', city: '', district: '', detailed_address: '' }
    })
    expect(hoisted.fetchUserInfo).toHaveBeenCalledTimes(2)
    expect(hoisted.authUserInfo.additional_info).toBe('{"user_icon":"/uploads/avatar.png"}')
    expect(hoisted.authUserInfo.avatar_url).toBe('/uploads/avatar.png')
    expect(hoisted.localStgSet).toHaveBeenCalled()
    expect(wrapper.vm.header).toBe(true)
    expect(wrapper.vm.headUrl).toBe('http://localhost/uploads/avatar.png')
  })

  it('should handle upload finish when additional_info is malformed json', async () => {
    hoisted.changeInformation.mockResolvedValue({ error: null })
    hoisted.fetchUserInfo.mockResolvedValue({
      data: {
        name: 'Test',
        email: 'test@test.com',
        phone_number: '+8613800138000',
        authority: '',
        additional_info: '{bad json',
        organization: '',
        timezone: '',
        default_language: '',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const mockEvent = { target: { response: JSON.stringify({ data: { path: '/uploads/avatar-2.png' } }) } }
    await wrapper.vm.handleFinish({ event: mockEvent as any })
    expect(hoisted.changeInformation).toHaveBeenCalledWith(expect.objectContaining({
      additional_info: '{"user_icon":"/uploads/avatar-2.png"}',
      avatar_url: '/uploads/avatar-2.png'
    }))
    expect(wrapper.vm.headUrl).toBe('http://localhost/uploads/avatar-2.png')
  })

  it('should handle avatar with user_icon in additional_info', async () => {
    hoisted.fetchUserInfo.mockResolvedValue({
      data: {
        name: 'Test',
        email: 'test@test.com',
        phone_number: '',
        authority: '',
        additional_info: '{"user_icon":"/uploads/icon.png"}',
        organization: '',
        timezone: '',
        default_language: '',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.vm.header).toBe(true)
  })

  it('should handle empty additional_info', async () => {
    hoisted.fetchUserInfo.mockResolvedValue({
      data: {
        name: 'Test',
        email: 'test@test.com',
        phone_number: '',
        authority: '',
        additional_info: '{}',
        organization: '',
        timezone: '',
        default_language: '',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.vm.header).toBe(false)
  })

  it('should getSubmitUserInfoData exclude country_code and phone_only', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const data = wrapper.vm.getSubmitUserInfoData()
    expect(data).not.toHaveProperty('country_code')
    expect(data).not.toHaveProperty('phone_only')
    expect(data.phone_number).toBe('+8613800138000')
  })
})
