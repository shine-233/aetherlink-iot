/**
 * 文件用途：覆盖 warning-email-setting 在 系统与账号设置 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchWarningEmails: vi.fn(),
  updateWarningEmails: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  authUserInfo: { authority: 'TENANT_ADMIN', roles: ['TENANT_ADMIN'] as string[] }
}))

vi.mock('@/service/api/personal-center', () => ({
  fetchWarningEmails: hoisted.fetchWarningEmails,
  updateWarningEmails: hoisted.updateWarningEmails
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: hoisted.authUserInfo
  })
}))

import WarningEmailSetting from '../warning-email-setting.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(WarningEmailSetting, {
    global: {
      stubs: {
        NSpin: defineComponent({
          props: { show: Boolean },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NFlex: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NAlert: defineComponent({
          props: { type: String, showIcon: Boolean },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NForm: defineComponent({
          props: { labelPlacement: String, labelWidth: [String, Number] },
          setup(_, { slots }) {
            return () => h('form', slots.default ? slots.default() : [])
          }
        }),
        NFormItem: defineComponent({
          props: { label: String },
          setup(props, { slots }) {
            return () =>
              h('div', [props.label ? h('span', String(props.label)) : null, ...(slots.default ? slots.default() : [])])
          }
        }),
        NInput: defineComponent({
          props: {
            value: { default: '' },
            type: String,
            placeholder: String,
            clearable: Boolean,
            disabled: Boolean
          },
          emits: ['update:value'],
          setup(props, { emit }) {
            return () => h('input', { disabled: props.disabled })
          }
        }),
        NCheckbox: defineComponent({
          props: { checked: Boolean, disabled: Boolean },
          setup(_, { slots }) {
            return () => h('label', slots.default ? slots.default() : [])
          }
        }),
        NText: defineComponent({
          props: { depth: Number },
          setup(_, { slots }) {
            return () => h('span', slots.default ? slots.default() : [])
          }
        }),
        NSpace: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NButton: defineComponent({
          props: { loading: Boolean, type: String, disabled: Boolean },
          emits: ['click'],
          setup(props, { slots, emit }) {
            return () =>
              h(
                'button',
                {
                  disabled: props.disabled,
                  onClick: () => {
                    if (!props.disabled) {
                      emit('click')
                    }
                  }
                },
                slots.default ? slots.default() : []
              )
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) =>
  wrapper.vm.$.setupState as Record<string, any>

describe('management/setting/components/warning-email-setting.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.authUserInfo.authority = 'TENANT_ADMIN'
    hoisted.authUserInfo.roles = ['TENANT_ADMIN']
    hoisted.fetchWarningEmails.mockResolvedValue({
      error: null,
      data: ['admin@test.com', 'ops@test.com']
    })
    hoisted.updateWarningEmails.mockResolvedValue({
      error: null,
      data: ['admin@test.com', 'ops@test.com']
    })
    ;(globalThis as any).$message = {
      success: hoisted.messageSuccess,
      error: hoisted.messageError
    }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads warning email recipients into saved and editable state on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchWarningEmails).toHaveBeenCalledTimes(1)
    expect(state.savedEmails).toEqual(['admin@test.com', 'ops@test.com'])
    expect(state.emailText).toBe('admin@test.com, ops@test.com')
    expect(state.loading).toBe(false)
  })

  it('calls fetchWarningEmails on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchWarningEmails).toHaveBeenCalledTimes(1)
  })

  it('populates emailText and savedEmails on successful load', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.savedEmails).toEqual(['admin@test.com', 'ops@test.com'])
    expect(state.emailText).toBe('admin@test.com, ops@test.com')
    expect(state.loading).toBe(false)
  })

  it('handles fetchWarningEmails returning non-array data gracefully', async () => {
    hoisted.fetchWarningEmails.mockResolvedValue({ error: null, data: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.savedEmails).toEqual([])
    expect(state.emailText).toBe('')
  })

  it('handles fetchWarningEmails with error gracefully', async () => {
    hoisted.fetchWarningEmails.mockResolvedValue({ error: 'network error', data: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.savedEmails).toEqual([])
    expect(state.emailText).toBe('')
  })

  it('parseEmails splits by newline, comma, semicolon and deduplicates', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.emailText = 'a@b.com\nb@c.com, c@d.com; a@b.com'
    const emails = state.parseEmails()
    expect(emails).toEqual(['a@b.com', 'b@c.com', 'c@d.com'])
  })

  it('saveWarningEmails calls updateWarningEmails with valid emails', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.emailText = 'user1@test.com, user2@test.com'
    await state.saveWarningEmails()
    await flushPromises()
    expect(hoisted.updateWarningEmails).toHaveBeenCalledWith({
      emails: ['user1@test.com', 'user2@test.com']
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.warningEmail.saved')
  })

  it('saveWarningEmails shows error for invalid email', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.emailText = 'invalid-email, user@test.com'
    await state.saveWarningEmails()
    await flushPromises()
    expect(hoisted.messageError).toHaveBeenCalledTimes(1)
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.warningEmail.invalidEmail: invalid-email')
    expect(hoisted.updateWarningEmails).toHaveBeenCalledTimes(0)
  })

  it('resetWarningEmails restores emailText from savedEmails', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.emailText = 'modified@test.com'
    state.resetWarningEmails()
    expect(state.emailText).toBe('admin@test.com, ops@test.com')
  })

  it('loadWarningEmails sets loading to true then false', async () => {
    let loadingDuringCall = false
    hoisted.fetchWarningEmails.mockImplementation(async () => {
      return { error: null, data: [] }
    })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    // After mount + flush, loading should be false
    await flushPromises()
    expect(state.loading).toBe(false)
  })

  it('saveWarningEmails syncs data from API response on success', async () => {
    hoisted.updateWarningEmails.mockResolvedValue({
      error: null,
      data: ['new@test.com']
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.emailText = 'new@test.com'
    await state.saveWarningEmails()
    await flushPromises()
    expect(state.savedEmails).toEqual(['new@test.com'])
    expect(state.emailText).toBe('new@test.com')
  })

  it('saveWarningEmails falls back to local emails when API returns non-array', async () => {
    hoisted.updateWarningEmails.mockResolvedValue({
      error: null,
      data: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.emailText = 'fallback@test.com'
    await state.saveWarningEmails()
    await flushPromises()
    expect(state.savedEmails).toEqual(['fallback@test.com'])
  })

  it('saveWarningEmails does not show success message on API error', async () => {
    hoisted.updateWarningEmails.mockResolvedValue({ error: 'server error' })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.emailText = 'user@test.com'
    await state.saveWarningEmails()
    await flushPromises()
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(0)
  })

  it('renders the legacy notification scope as device alerts', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.text()).toContain('custom.management.warningEmail.scope')
    expect(wrapper.text()).toContain('custom.management.warningEmail.deviceAlerts')
  })

  it('blocks editing for tenant users', async () => {
    hoisted.authUserInfo.authority = 'TENANT_USER'
    hoisted.authUserInfo.roles = ['TENANT_USER']
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.emailText = 'tenant-user@test.com'
    await state.saveWarningEmails()
    expect(wrapper.text()).toContain('custom.management.warningEmail.readOnly')
    expect(state.canEditWarningEmails).toBe(false)
    expect(hoisted.updateWarningEmails).toHaveBeenCalledTimes(0)
  })

  it('renders warning-email controls as read-only for tenant users', async () => {
    hoisted.authUserInfo.authority = 'TENANT_USER'
    hoisted.authUserInfo.roles = ['TENANT_USER']
    const wrapper = mountComponent()
    await flushPromises()

    const buttons = wrapper.findAll('button')
    const reloadButton = buttons.find(button => button.text().includes('custom.management.warningEmail.reload'))
    const resetButton = buttons.find(button => button.text().includes('custom.management.warningEmail.reset'))
    const saveButton = buttons.find(button => button.text().includes('custom.management.warningEmail.save'))

    // 只读态下输入框必须真正带 disabled 属性（Vue 渲染为空串），而不仅仅是属性存在。
    expect(wrapper.find('input').attributes('disabled')).toBe('')
    // reload 允许只读用户点击（刷新是无副作用读操作），必须保持可用。
    expect(reloadButton!.attributes('disabled')).toBeUndefined()
    // reset/save 会写状态，只读用户必须被禁用。
    expect(resetButton!.attributes('disabled')).toBe('')
    expect(saveButton!.attributes('disabled')).toBe('')
  })
})
