/**
 * 文件用途：覆盖 account-profile-setting 在 系统与账号设置 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchUserInfo: vi.fn(),
  changeInformation: vi.fn(),
  passwordModification: vi.fn(),
  messageSuccess: vi.fn(),
  localStgSet: vi.fn(),
  changeLocale: vi.fn(),
  authUserInfo: {
    id: '1',
    email: 'user@test.com',
    userEmail: 'user@test.com',
    name: 'User',
    userName: 'User',
    default_language: 'en-US',
    additional_info: '{}',
    avatar_url: ''
  }
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => ({
    locale: 'en-US',
    localeOptions: [
      { label: 'Chinese', key: 'zh-CN' },
      { label: 'English', key: 'en-US' }
    ],
    changeLocale: hoisted.changeLocale
  })
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: hoisted.authUserInfo
  })
}))

vi.mock('@/service/api/personal-center', () => ({
  fetchUserInfo: hoisted.fetchUserInfo,
  changeInformation: hoisted.changeInformation,
  passwordModification: hoisted.passwordModification
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    set: hoisted.localStgSet,
    get: vi.fn(),
    remove: vi.fn()
  }
}))

vi.mock('@/utils/form/rule', () => ({
  getConfirmPwdRule: () => ({ required: true, message: 'confirm pwd', trigger: ['input', 'blur'] })
}))

vi.mock('@/utils/common/tool', () => ({
  generateRandomHexString: (len: number) => 'a'.repeat(len),
  validName: (v: string) => !!v && v.trim().length > 0,
  validPasswordByExp: (v: string) => /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,20}$/.test(v)
}))

vi.mock('@/utils/security/rsa-encrypt', () => ({
  encryptDataByRsa: (v: string) => `encrypted-${v}`
}))

import AccountProfileSetting from '../account-profile-setting.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(AccountProfileSetting, {
    global: {
      stubs: {
        NSpin: defineComponent({ props: { show: Boolean }, setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], props: { loading: Boolean }, setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NDivider: defineComponent({ setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/setting/components/account-profile-setting.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.authUserInfo.id = '1'
    hoisted.authUserInfo.email = 'user@test.com'
    hoisted.authUserInfo.userEmail = 'user@test.com'
    hoisted.authUserInfo.name = 'User'
    hoisted.authUserInfo.userName = 'User'
    hoisted.authUserInfo.default_language = 'en-US'
    hoisted.authUserInfo.additional_info = '{}'
    hoisted.authUserInfo.avatar_url = ''
    hoisted.fetchUserInfo.mockResolvedValue({
      error: null,
      data: {
        id: '1',
        name: 'User',
        email: 'user@test.com',
        default_language: 'en-US',
        phone_number: '+8613800000000',
        additional_info: '{}',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
    hoisted.changeInformation.mockResolvedValue({ error: null })
    hoisted.passwordModification.mockResolvedValue({ error: null })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads the account profile form and password form defaults on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchUserInfo).toHaveBeenCalledTimes(1)
    expect(state.profileForm).toMatchObject({
      name: 'User',
      default_language: 'en-US'
    })
    expect(state.userInfoSnapshot).toMatchObject({
      id: '1',
      email: 'user@test.com',
      phone_number: '+8613800000000'
    })
    expect(state.passwordForm).toEqual({
      old_password: '',
      password: '',
      passwords: ''
    })
    expect(state.loading).toBe(false)
  })

  it('calls loadUserInfo on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchUserInfo).toHaveBeenCalledTimes(1)
  })

  it('languageOptions maps appStore localeOptions', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.languageOptions).toHaveLength(2)
    expect(state.languageOptions[0]).toEqual({ label: 'Chinese', value: 'zh-CN' })
    expect(state.languageOptions[1]).toEqual({ label: 'English', value: 'en-US' })
  })

  it('normalizeLocale maps locale strings correctly', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.normalizeLocale('zh-cn')).toBe('zh-CN')
    expect(state.normalizeLocale('en-us')).toBe('en-US')
    expect(state.normalizeLocale('fr-fr')).toBe('fr-FR')
    expect(state.normalizeLocale('es-es')).toBe('es-ES')
  })

  it('normalizeLocale returns default locale for unknown', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.normalizeLocale('unknown')).toBe('en-US')
    expect(state.normalizeLocale('')).toBe('en-US')
  })

  it('getPhoneNumber returns phone_number or phone_num', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.getPhoneNumber({ phone_number: '123' })).toBe('123')
    expect(state.getPhoneNumber({ phone_num: '456' })).toBe('456')
    expect(state.getPhoneNumber({})).toBe('')
  })

  it('syncProfile populates profileForm and userInfoSnapshot', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.syncProfile({ name: 'New Name', default_language: 'zh-CN', phone_number: '123' })
    expect(state.profileForm.name).toBe('New Name')
    expect(state.profileForm.default_language).toBe('zh-CN')
    expect(state.userInfoSnapshot.phone_number).toBe('123')
  })

  it('syncProfile handles missing address', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.syncProfile({ name: 'Test', default_language: 'en-US' })
    expect(state.userInfoSnapshot.address).toEqual({ province: '', city: '', district: '', detailed_address: '' })
  })

  it('loadUserInfo fetches and syncs profile', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.loading).toBe(false)
    expect(state.profileForm.name).toBe('User')
    expect(hoisted.authUserInfo.name).toBe('User')
    expect(hoisted.localStgSet).toHaveBeenCalled()
  })

  it('loadUserInfo syncs fetched email and avatar into cached auth state', async () => {
    hoisted.fetchUserInfo.mockResolvedValue({
      error: null,
      data: {
        id: '1',
        name: 'Fetched User',
        email: 'fresh@test.com',
        default_language: 'fr-fr',
        phone_number: '+8613800000000',
        additional_info: '{"user_icon":"/uploads/header.png"}',
        address: { province: '', city: '', district: '', detailed_address: '' }
      }
    })
    mountComponent()
    await flushPromises()
    expect(hoisted.authUserInfo.name).toBe('Fetched User')
    expect(hoisted.authUserInfo.userName).toBe('Fetched User')
    expect(hoisted.authUserInfo.email).toBe('fresh@test.com')
    expect(hoisted.authUserInfo.userEmail).toBe('fresh@test.com')
    expect(hoisted.authUserInfo.default_language).toBe('fr-FR')
    expect(hoisted.authUserInfo.avatar_url).toBe('/uploads/header.png')
  })

  it('saveProfile validates and calls changeInformation on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.changeInformation.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    state.profileFormRef = { validate: vi.fn().mockResolvedValue(undefined) }
    state.profileForm.name = 'Updated Name'
    await state.saveProfile()
    await flushPromises()
    expect(hoisted.changeInformation).toHaveBeenCalledTimes(1)
    expect(hoisted.changeInformation).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Updated Name',
      default_language: 'en-US'
    }))
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.accountProfile.profileSaved')
  })

  it('saveProfile changes locale when different', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.changeInformation.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    state.profileFormRef = { validate: vi.fn().mockResolvedValue(undefined) }
    state.profileForm.default_language = 'zh-CN'
    await state.saveProfile()
    await flushPromises()
    expect(hoisted.changeLocale).toHaveBeenCalledWith('zh-CN', { persistRemote: false })
  })

  it('resetPasswordForm clears password fields', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.passwordForm.old_password = 'old'
    state.passwordForm.password = 'new'
    state.passwordForm.passwords = 'new'
    state.passwordFormRef = { restoreValidation: vi.fn() }
    state.resetPasswordForm()
    expect(state.passwordForm.old_password).toBe('')
    expect(state.passwordForm.password).toBe('')
    expect(state.passwordForm.passwords).toBe('')
  })

  it('shouldEncryptPassword returns false when localStorage has invalid data', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.shouldEncryptPassword()).toBe(false)
  })

  it('savePassword validates and calls passwordModification on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.passwordModification.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    state.passwordFormRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.passwordForm.old_password = 'oldpass'
    state.passwordForm.password = 'ValidPass123'
    state.passwordForm.passwords = 'ValidPass123'
    await state.savePassword()
    await flushPromises()
    expect(hoisted.passwordModification).toHaveBeenCalledTimes(1)
    expect(hoisted.passwordModification).toHaveBeenCalledWith({
      old_password: 'oldpass',
      password: 'ValidPass123',
      salt: null
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.accountProfile.passwordSaved')
  })

  it('profileRules are defined', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.profileRules.name).toHaveLength(1)
    expect(state.profileRules.name[0]).toMatchObject({
      required: true,
      trigger: ['input', 'blur']
    })
    expect(state.profileRules.name[0].validator).toEqual(expect.any(Function))
    expect(state.profileRules.default_language).toEqual({
      required: true,
      message: 'page.manage.user.form.defaultLanguage',
      trigger: ['change', 'blur']
    })
  })

  it('passwordRules are defined', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.passwordRules.old_password).toEqual({
      required: true,
      message: 'generate.old-password',
      trigger: ['input', 'blur']
    })
    expect(state.passwordRules.password).toHaveLength(1)
    expect(state.passwordRules.password[0]).toMatchObject({
      required: true,
      message: 'form.pwd.tip',
      trigger: ['input', 'blur']
    })
    expect(state.passwordRules.password[0].validator).toEqual(expect.any(Function))
    expect(state.passwordRules.passwords).toEqual({ required: true, message: 'confirm pwd', trigger: ['input', 'blur'] })
  })
})
