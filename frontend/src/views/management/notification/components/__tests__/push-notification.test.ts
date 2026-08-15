/**
 * 文件用途：覆盖 push-notification 在 通知服务管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchPushNotificationServices: vi.fn(),
  editPushNotificationServices: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api', () => ({
  fetchPushNotificationServices: hoisted.fetchPushNotificationServices,
  editPushNotificationServices: hoisted.editPushNotificationServices
}))

vi.mock('~/src/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@aetherlink/hooks', () => ({
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

import PushNotification from '../push-notification.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(PushNotification, {
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
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/notification/components/push-notification.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchPushNotificationServices.mockResolvedValue({
      data: { url: 'https://push.example.com' }
    })
    hoisted.editPushNotificationServices.mockResolvedValue({ error: null })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads push-notification service URL into the form on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchPushNotificationServices).toHaveBeenCalledTimes(1)
    expect(state.formModel).toEqual({
      url: 'https://push.example.com'
    })
    expect(state.loading).toBe(false)
  })

  it('calls getNotificationServices on init', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchPushNotificationServices).toHaveBeenCalledTimes(1)
  })

  it('createDefaultFormModel returns default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const model = state.createDefaultFormModel()
    expect(model.url).toBe('')
  })

  it('setTableData assigns data and sets url when not null', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.setTableData({ url: 'https://push.example.com' })
    expect(state.formModel.url).toBe('https://push.example.com')
  })

  it('setTableData clears url when API returns the null sentinel', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formModel.url = 'existing-url'
    state.setTableData({ url: 'null' })
    expect(state.formModel.url).toBe('')
  })

  it('handleSubmit validates, calls editPushNotificationServices and refreshes on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editPushNotificationServices.mockResolvedValue({ error: null })
    hoisted.fetchPushNotificationServices.mockResolvedValue({ data: { url: 'https://push.example.com' } })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.editPushNotificationServices).toHaveBeenCalledTimes(1)
    expect(hoisted.editPushNotificationServices).toHaveBeenCalledWith({
      url: 'https://push.example.com'
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.saveSuccess')
    expect(hoisted.fetchPushNotificationServices).toHaveBeenCalledTimes(1)
  })

  it('handleSubmit does not refresh when API returns error', async () => {
    hoisted.editPushNotificationServices.mockResolvedValue({ error: 'fail' })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editPushNotificationServices.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.fetchPushNotificationServices).toHaveBeenCalledTimes(0)
  })

  it('handleSubmit removes config from formData before submit', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editPushNotificationServices.mockResolvedValue({ error: null })
    hoisted.fetchPushNotificationServices.mockResolvedValue({ data: { url: 'https://push.example.com' } })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    await flushPromises()
    const callArgs = hoisted.editPushNotificationServices.mock.calls[0][0]
    expect(callArgs.config).toBeUndefined()
  })

  it('rules are defined for pushServer', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.rules.pushServer).toEqual({
      required: true,
      message: 'common.pleaseCheckValue',
      trigger: ['input', 'blur']
    })
  })

  it('loading is set to false after fetch completes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.loading).toBe(false)
  })

  it('populates formModel.url from fetched data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.formModel.url).toBe('https://push.example.com')
  })
})
