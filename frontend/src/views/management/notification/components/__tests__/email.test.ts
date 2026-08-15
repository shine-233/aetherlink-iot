/**
 * 文件用途：覆盖 email 在 通知服务管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchNotificationServicesEmail: vi.fn(),
  editNotificationServices: vi.fn(),
  sendTestEmail: vi.fn(),
  messageSuccess: vi.fn(),
  messageLoading: vi.fn(() => ({ destroy: vi.fn() })),
  messageError: vi.fn()
}))

vi.mock('@/service/api', () => ({
  fetchNotificationServicesEmail: hoisted.fetchNotificationServicesEmail,
  editNotificationServices: hoisted.editNotificationServices,
  sendTestEmail: hoisted.sendTestEmail
}))

vi.mock('~/src/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@aetherlink/hooks', () => ({
  useBoolean: (init = false) => {
    const bool = ref(init)
    return {
      bool,
      setTrue: vi.fn(() => {
        bool.value = true
      }),
      setFalse: vi.fn(() => {
        bool.value = false
      })
    }
  },
  useLoading: (init = false) => {
    const loading = ref(init)
    return {
      loading,
      startLoading: vi.fn(() => {
        loading.value = true
      }),
      endLoading: vi.fn(() => {
        loading.value = false
      })
    }
  }
}))

vi.mock('@/utils/deep-clone', () => ({
  smartDeepClone: (obj: any) => JSON.parse(JSON.stringify(obj))
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg, trigger: ['input', 'blur'] })
}))

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      loading: hoisted.messageLoading,
      success: hoisted.messageSuccess,
      error: hoisted.messageError
    })
  }
})

import Email from '../email.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(Email, {
    global: {
      stubs: {
        NSpin: defineComponent({
          props: { show: Boolean },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NForm: defineComponent({
          setup(_, { slots }) {
            return () => h('form', slots.default ? slots.default() : [])
          }
        }),
        NFormItem: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NFormItemGridItem: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NGrid: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NInputNumber: defineComponent({
          props: { value: { default: 0 } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NCheckbox: defineComponent({
          props: { checked: { default: false } },
          emits: ['update:checked'],
          setup() {
            return () => h('div')
          }
        }),
        NSwitch: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NSpace: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
          }
        }),
        NModal: defineComponent({
          props: { show: Boolean },
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/notification/components/email.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchNotificationServicesEmail.mockResolvedValue({
      data: {
        id: 'e-1',
        email_config: {
          host: 'smtp.test.com',
          port: 587,
          from_email: 'test@test.com',
          from_password: 'pass',
          ssl: false
        },
        config: '{"host":"smtp.test.com","port":587}',
        notice_type: 'EMAIL',
        status: 'OPEN',
        remark: ''
      }
    })
    hoisted.editNotificationServices.mockResolvedValue({ error: null })
    hoisted.sendTestEmail.mockResolvedValue({ error: null })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess, error: hoisted.messageError }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads email notification service config into the form on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchNotificationServicesEmail).toHaveBeenCalledTimes(1)
    expect(state.formModel).toMatchObject({
      id: 'e-1',
      notice_type: 'EMAIL',
      status: 'OPEN',
      remark: '',
      email_config: {
        host: 'smtp.test.com',
        port: 587
      }
    })
    expect(state.loading).toBe(false)
  })

  it('calls getNotificationServices on init', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchNotificationServicesEmail).toHaveBeenCalledTimes(1)
  })

  it('createDefaultFormModel returns default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const model = state.createDefaultFormModel()
    expect(model.id).toBe('')
    expect(model.email_config).toEqual({})
    expect(model.notice_type).toBe('EMAIL')
    expect(model.status).toBe('OPEN')
  })

  it('setTableData assigns data and parses config', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.setTableData({
      id: 'e-1',
      email_config: {},
      config: '{"host":"smtp.test.com","port":587}',
      notice_type: 'EMAIL',
      status: 'OPEN',
      remark: ''
    })
    expect(state.formModel.id).toBe('e-1')
    expect(state.formModel.email_config).toEqual({ host: 'smtp.test.com', port: 587 })
  })

  it('handleOpenModal resets debugData and opens modal', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.debugData.body = 'old'
    state.handleOpenModal()
    expect(state.debugData.body).toBe('')
    expect(state.debugData.email).toBe('')
    expect(state.debugData.header).toBe('')
    expect(state.visible).toBe(true)
  })

  it('handleSubmit validates, calls editNotificationServices and refreshes on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editNotificationServices.mockResolvedValue({ error: null })
    hoisted.fetchNotificationServicesEmail.mockResolvedValue({
      data: { id: 'e-1', config: 'null', email_config: {}, notice_type: 'EMAIL', status: 'OPEN', remark: '' }
    })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.editNotificationServices).toHaveBeenCalledTimes(1)
    expect(hoisted.editNotificationServices).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'e-1',
        notice_type: 'EMAIL',
        status: 'OPEN',
        remark: '',
        email_config: expect.objectContaining({
          host: 'smtp.test.com',
          port: 587
        })
      })
    )
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('success')
    expect(hoisted.fetchNotificationServicesEmail).toHaveBeenCalledTimes(1)
  })

  it('handleSubmit does not refresh when API returns error', async () => {
    hoisted.editNotificationServices.mockResolvedValue({ error: 'fail' })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editNotificationServices.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.fetchNotificationServicesEmail).toHaveBeenCalledTimes(0)
  })

  it('handleSubmit removes config from formData before submit', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editNotificationServices.mockResolvedValue({ error: null })
    hoisted.fetchNotificationServicesEmail.mockResolvedValue({
      data: { id: 'e-1', config: 'null', email_config: {}, notice_type: 'EMAIL', status: 'OPEN', remark: '' }
    })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    await flushPromises()
    const callArgs = hoisted.editNotificationServices.mock.calls[0][0]
    expect(callArgs.config).toBeUndefined()
  })

  it('handleSend validates, calls sendTestEmail and closes modal on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.sendTestEmail.mockResolvedValue({ error: null })
    hoisted.messageLoading.mockReturnValue({ destroy: vi.fn() })
    const state = getSetupState(wrapper)
    state.debugFormRef = { validate: vi.fn().mockResolvedValue(undefined) }
    state.debugData.email = 'receiver@test.com'
    state.debugData.body = 'alarm test body'
    state.debugData.header = 'alarm test header'
    await state.handleSend()
    await flushPromises()
    expect(hoisted.sendTestEmail).toHaveBeenCalledTimes(1)
    expect(hoisted.sendTestEmail).toHaveBeenCalledWith({
      email: 'receiver@test.com',
      body: 'alarm test body',
      header: 'alarm test header'
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('success')
    expect(state.visible).toBe(false)
  })

  it('rules are defined for email config fields', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const requiredRule = { required: true, message: 'common.pleaseCheckValue', trigger: ['input', 'blur'] }
    expect(state.rules['email_config.host']).toEqual(requiredRule)
    expect(state.rules['email_config.port']).toEqual(requiredRule)
    expect(state.rules['email_config.from_email']).toEqual(requiredRule)
    expect(state.rules['email_config.from_password']).toEqual(requiredRule)
    expect(state.rules.email).toEqual(requiredRule)
    expect(state.rules.body).toEqual(requiredRule)
  })

  it('loading is set to false after fetch completes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.loading).toBe(false)
  })
})
