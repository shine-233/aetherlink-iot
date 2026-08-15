/**
 * 文件用途: 覆盖Config Modal在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfigAdd: vi.fn(),
  deviceConfigEdit: vi.fn(),
  deviceTemplate: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigAdd: hoisted.deviceConfigAdd,
  deviceConfigEdit: hoisted.deviceConfigEdit,
  deviceTemplate: hoisted.deviceTemplate
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import Component from '../config-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      modalVisible: false,
      modalType: 'add',
      ...props
    },
    global: {
      stubs: {
        NCard: defineComponent({ props: ['title'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'placeholder'], emits: ['update:value'], setup() { return () => h('input') } }),
        NSelect: defineComponent({ props: ['value', 'options'], emits: ['update:value'], setup() { return () => h('div') } }),
        NRadioGroup: defineComponent({ props: ['value'], emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NRadio: defineComponent({ props: ['value'], setup(_, { slots }) { return () => h('label', slots.default?.()) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const deferred = <T>() => {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, reject, resolve }
}

const bottomScrollEvent = {
  currentTarget: { offsetHeight: 20, scrollHeight: 100, scrollTop: 80 }
} as unknown as Event

describe('device/config/modules/config-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceTemplate.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.deviceConfigAdd.mockResolvedValue({ error: null })
    hoisted.deviceConfigEdit.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes add dialog form model and required device config rules', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.modalTitle).toBe('generate.add')
    expect(state.configForm).toEqual({
      additional_info: null,
      description: null,
      device_conn_type: null,
      device_template_id: null,
      device_type: null,
      name: null,
      protocol_config: null,
      protocol_type: null,
      remark: null,
      voucher_type: null
    })
    expect(state.configFormRules).toMatchObject({
      name: {
        required: true,
        message: 'common.deviceConfigName',
        trigger: 'blur'
      },
      device_type: {
        required: true,
        message: 'common.deviceAccessType',
        trigger: 'change'
      },
      device_conn_type: {
        required: true,
        message: 'common.deviceConnectionMethod',
        trigger: 'change'
      }
    })
  })

  it('initializes configForm with default values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.configForm.name).toBeNull()
    expect(state.configForm.device_type).toBeNull()
    expect(state.configForm.device_conn_type).toBeNull()
  })

  it('sets modalTitle to add when modalType is add', async () => {
    const wrapper = mountComponent({ modalType: 'add' })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.modalTitle).toBe('generate.add')
  })

  it('sets modalTitle to edit when modalType is edit', async () => {
    const wrapper = mountComponent({ modalType: 'edit', modalVisible: false })
    await flushPromises()
    await wrapper.setProps({ modalVisible: true })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.modalTitle).toBe('common.edit')
  })

  it('loads the first template page when the dialog opens', async () => {
    hoisted.deviceTemplate.mockResolvedValue({ data: { list: [{ id: '1', name: 'Template1' }], total: 1 } })
    const wrapper = mountComponent({ modalVisible: false })
    await wrapper.setProps({ modalVisible: true })
    await flushPromises()

    expect(hoisted.deviceTemplate).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(getSetupState(wrapper).deviceTemplateOptions).toEqual([{ id: '1', name: 'Template1' }])
  })

  it('appends template pages and removes duplicate IDs', async () => {
    hoisted.deviceTemplate
      .mockResolvedValueOnce({ data: { list: [{ id: '1', name: 'One' }, { id: '2', name: 'Old two' }], total: 3 } })
      .mockResolvedValueOnce({ data: { list: [{ id: '2', name: 'New two' }, { id: '3', name: 'Three' }], total: 3 } })
    const wrapper = mountComponent({ modalVisible: false })
    await wrapper.setProps({ modalVisible: true })
    await flushPromises()

    getSetupState(wrapper).deviceTemplateScroll(bottomScrollEvent)
    await flushPromises()

    expect(hoisted.deviceTemplate).toHaveBeenNthCalledWith(2, { page: 2, page_size: 20 })
    expect(getSetupState(wrapper).deviceTemplateOptions).toEqual([
      { id: '1', name: 'One' },
      { id: '2', name: 'New two' },
      { id: '3', name: 'Three' }
    ])
  })

  it('does not start duplicate template requests while a page is loading', async () => {
    const nextPage = deferred<{ data: { list: Array<{ id: string; name: string }>; total: number } }>()
    hoisted.deviceTemplate
      .mockResolvedValueOnce({ data: { list: [{ id: '1', name: 'One' }], total: 2 } })
      .mockReturnValueOnce(nextPage.promise)
    const wrapper = mountComponent({ modalVisible: false })
    await wrapper.setProps({ modalVisible: true })
    await flushPromises()
    const state = getSetupState(wrapper)

    state.deviceTemplateScroll(bottomScrollEvent)
    state.deviceTemplateScroll(bottomScrollEvent)
    expect(hoisted.deviceTemplate).toHaveBeenCalledTimes(2)

    nextPage.resolve({ data: { list: [{ id: '2', name: 'Two' }], total: 2 } })
    await flushPromises()
  })

  it('does not request another template page after reaching total', async () => {
    hoisted.deviceTemplate.mockResolvedValue({ data: { list: [{ id: '1', name: 'One' }], total: 1 } })
    const wrapper = mountComponent({ modalVisible: false })
    await wrapper.setProps({ modalVisible: true })
    await flushPromises()

    getSetupState(wrapper).deviceTemplateScroll(bottomScrollEvent)
    await flushPromises()
    expect(hoisted.deviceTemplate).toHaveBeenCalledTimes(1)
  })

  it('keeps the loaded template state when a later request rejects', async () => {
    hoisted.deviceTemplate
      .mockResolvedValueOnce({ data: { list: [{ id: '1', name: 'One' }], total: 2 } })
      .mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountComponent({ modalVisible: false })
    await wrapper.setProps({ modalVisible: true })
    await flushPromises()
    const state = getSetupState(wrapper)

    state.deviceTemplateScroll(bottomScrollEvent)
    await flushPromises()
    expect(state.deviceTemplateOptions).toEqual([{ id: '1', name: 'One' }])
    expect(state.templateLoading).toBe(false)
  })

  it('modalClose emits modalClose event', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.modalClose()
    expect(wrapper.emitted('modalClose')).toEqual([[]])
  })

  it('handleClose resets form and closes modal', async () => {
    const wrapper = mountComponent({ modalVisible: true })
    await flushPromises()
    const state = getSetupState(wrapper)
    state.configForm.name = 'test'
    state.visible = true
    state.handleClose()
    expect(state.configForm.name).toBeNull()
    expect(state.visible).toBe(false)
    expect(wrapper.emitted('modalClose')).toEqual([[]])
  })

  it('submits add mode and closes only after successful persistence', async () => {
    const wrapper = mountComponent({ modalType: 'add' })
    const state = getSetupState(wrapper)
    state.configFormRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.configForm.name = 'Test Config'
    state.configForm.device_type = '1'
    state.configForm.device_conn_type = 'A'
    state.visible = true

    await state.handleSubmit()

    expect(hoisted.deviceConfigAdd).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Test Config',
      device_type: '1',
      device_conn_type: 'A'
    }))
    expect(wrapper.emitted('submitted')).toEqual([[]])
    expect(wrapper.emitted('modalClose')).toEqual([[]])
    expect(state.visible).toBe(false)
  })

  it('submits edit mode through deviceConfigEdit', async () => {
    const wrapper = mountComponent({ modalType: 'edit' })
    const state = getSetupState(wrapper)
    state.configFormRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.configForm.name = 'Test Config'
    await state.handleSubmit()
    expect(hoisted.deviceConfigEdit).toHaveBeenCalledWith(expect.objectContaining({ name: 'Test Config' }))
  })

  it.each([
    ['add response error', 'add', 'response'],
    ['edit response error', 'edit', 'response'],
    ['add rejected request', 'add', 'reject']
  ])('keeps the dialog and input on %s', async (_name, modalType, failure) => {
    const api = modalType === 'add' ? hoisted.deviceConfigAdd : hoisted.deviceConfigEdit
    if (failure === 'reject') api.mockRejectedValueOnce(new Error('offline'))
    else api.mockResolvedValueOnce({ error: { message: 'save failed' } })
    const wrapper = mountComponent({ modalType })
    const state = getSetupState(wrapper)
    state.configFormRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.configForm.name = 'Keep me'
    state.visible = true

    await state.handleSubmit()

    expect(state.visible).toBe(true)
    expect(state.configForm.name).toBe('Keep me')
    expect(wrapper.emitted('submitted')).toBeUndefined()
    expect(wrapper.emitted('modalClose')).toBeUndefined()
  })
})
