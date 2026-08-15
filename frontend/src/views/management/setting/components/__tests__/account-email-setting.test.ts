/**
 * 文件用途：覆盖 account-email-setting 在 系统与账号设置 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchEmailCodeByEmail: vi.fn(),
  changeAccountEmail: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  localStgSet: vi.fn(),
  authUserInfo: {
    id: '1',
    email: 'old@test.com',
    userEmail: 'old@test.com'
  }
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: hoisted.authUserInfo
  })
}))

vi.mock('@/service/api/auth', () => ({
  fetchEmailCodeByEmail: hoisted.fetchEmailCodeByEmail
}))

vi.mock('@/service/api/personal-center', () => ({
  changeAccountEmail: hoisted.changeAccountEmail
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    set: hoisted.localStgSet,
    get: vi.fn(),
    remove: vi.fn()
  }
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import AccountEmailSetting from '../account-email-setting.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(AccountEmailSetting, {
    global: {
      stubs: {
        NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NAlert: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ emits: ['click'], props: { loading: Boolean }, setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NText: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/setting/components/account-email-setting.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.authUserInfo.id = '1'
    hoisted.authUserInfo.email = 'old@test.com'
    hoisted.authUserInfo.userEmail = 'old@test.com'
    hoisted.fetchEmailCodeByEmail.mockResolvedValue({ error: null })
    hoisted.changeAccountEmail.mockResolvedValue({ error: null, data: { new_email: 'new@test.com', devices_migrated: 3 } })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess, error: hoisted.messageError }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('starts with current email visible and an empty change-email form', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    expect(state.currentEmail).toBe('old@test.com')
    expect(state.form).toEqual({
      new_email: '',
      verify_code: ''
    })
    expect(state.codeLoading).toBe(false)
    expect(state.submitLoading).toBe(false)
    expect(state.migratedDeviceCount).toBeNull()
  })

  it('currentEmail returns email from authStore userInfo', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.currentEmail).toBe('old@test.com')
  })

  it('resetForm clears form fields and migratedDeviceCount', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'test@test.com'
    state.form.verify_code = '123456'
    state.migratedDeviceCount = 5
    state.resetForm()
    expect(state.form.new_email).toBe('')
    expect(state.form.verify_code).toBe('')
    expect(state.migratedDeviceCount).toBeNull()
  })

  it('sendCode shows error when new_email is empty', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = ''
    await state.sendCode()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.accountEmail.newEmailRequired')
    expect(hoisted.fetchEmailCodeByEmail).toHaveBeenCalledTimes(0)
  })

  it('sendCode shows error when currentEmail is empty', async () => {
    hoisted.authUserInfo.email = ''
    hoisted.authUserInfo.userEmail = ''
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    await state.sendCode()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.accountEmail.currentEmail')
  })

  it('sendCode blocks when new email matches current email', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'old@test.com'
    await state.sendCode()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.accountEmail.sameEmailNotAllowed')
    expect(hoisted.fetchEmailCodeByEmail).toHaveBeenCalledTimes(0)
  })

  it('sendCode calls fetchEmailCodeByEmail and shows success on success', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    await state.sendCode()
    await flushPromises()
    expect(hoisted.fetchEmailCodeByEmail).toHaveBeenCalledWith('old@test.com')
    expect(state.codeCounting).toBe(true)
    expect(String(state.sendCodeLabel)).toContain('60')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.accountEmail.sent')
  })

  it('sendCode sets codeLoading to false after completion', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    await state.sendCode()
    await flushPromises()
    expect(state.codeLoading).toBe(false)
  })

  it('submitChange shows error when new_email is empty', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = ''
    await state.submitChange()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.accountEmail.newEmailRequired')
    expect(hoisted.changeAccountEmail).toHaveBeenCalledTimes(0)
  })

  it('submitChange shows error when verify_code is empty', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    state.form.verify_code = ''
    await state.submitChange()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.accountEmail.verifyCodeRequired')
    expect(hoisted.changeAccountEmail).toHaveBeenCalledTimes(0)
  })

  it('submitChange blocks when new email matches current email', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'old@test.com'
    state.form.verify_code = '123456'
    await state.submitChange()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.accountEmail.sameEmailNotAllowed')
    expect(hoisted.changeAccountEmail).toHaveBeenCalledTimes(0)
  })

  it('submitChange calls changeAccountEmail and updates store on success', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    state.form.verify_code = '123456'
    await state.submitChange()
    await flushPromises()
    expect(hoisted.changeAccountEmail).toHaveBeenCalledWith({ new_email: 'new@test.com', verify_code: '123456' })
    expect(state.migratedDeviceCount).toBe(3)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.accountEmail.changedWithCount')
    expect(hoisted.localStgSet).toHaveBeenCalledTimes(1)
    expect(hoisted.localStgSet).toHaveBeenCalledWith('userInfo', expect.objectContaining({
      id: '1',
      email: 'new@test.com',
      userEmail: 'new@test.com'
    }))
  })

  it('submitChange clears verify_code after success', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    state.form.verify_code = '123456'
    await state.submitChange()
    await flushPromises()
    expect(state.form.verify_code).toBe('')
  })

  it('submitChange also clears new_email after success', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    state.form.verify_code = '123456'
    await state.submitChange()
    await flushPromises()
    expect(state.form.new_email).toBe('')
  })

  it('submitChange falls back to generic success message when migrated count is missing', async () => {
    hoisted.changeAccountEmail.mockResolvedValue({ error: null, data: { new_email: 'new@test.com' } })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    state.form.verify_code = '123456'
    await state.submitChange()
    await flushPromises()
    expect(state.migratedDeviceCount).toBeNull()
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.accountEmail.changed')
  })

  it('submitChange sets submitLoading to false after completion', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    state.form.verify_code = '123456'
    await state.submitChange()
    await flushPromises()
    expect(state.submitLoading).toBe(false)
  })

  it('submitChange does not update store when API returns error', async () => {
    hoisted.changeAccountEmail.mockResolvedValue({ error: 'fail', data: null })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.form.new_email = 'new@test.com'
    state.form.verify_code = '123456'
    await state.submitChange()
    await flushPromises()
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(0)
    expect(state.migratedDeviceCount).toBeNull()
  })

  it('t function returns translation key with prefix', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.t('title')).toBe('custom.management.accountEmail.title')
    expect(state.t('sendCode')).toBe('custom.management.accountEmail.sendCode')
    expect(state.t('devicesRetainedDetail')).toBe('custom.management.accountEmail.devicesRetainedDetail')
  })
})
