/**
 * 文件用途: 覆盖ServiceModal在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  createServiceDrop: vi.fn(),
  getServiceAccessForm: vi.fn(),
  putServiceDrop: vi.fn(),
  getServiceListDrop: vi.fn()
}))

vi.mock('@/service/api/plugin', () => ({
  createServiceDrop: hoisted.createServiceDrop,
  getServiceAccessForm: hoisted.getServiceAccessForm,
  putServiceDrop: hoisted.putServiceDrop,
  getServiceListDrop: hoisted.getServiceListDrop
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../form.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))

import Component from '../serviceModal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NModal: defineComponent({
          props: { show: Boolean },
          emits: ['update:show'],
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
        NInput: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NSelect: defineComponent({
          props: { value: { default: null } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
          }
        }),
        NSteps: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NStep: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        FormInput: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/service-details/components/serviceModal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getServiceAccessForm.mockResolvedValue({ data: [] })
    hoisted.createServiceDrop.mockResolvedValue({ error: null })
    hoisted.putServiceDrop.mockResolvedValue({ error: null })
    hoisted.getServiceListDrop.mockResolvedValue({ data: [] })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('starts closed on manual mode with required access point rules', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    expect(state.serviceModals).toBe(false)
    expect(state.currentStep).toBe(1)
    expect(state.isEdit).toBe(false)
    expect(state.rules.name).toEqual({
      required: true,
      trigger: ['blur', 'input'],
      message: 'custom.serviceAccess.accessPointNameRequired'
    })
    expect(state.rules.auth_type).toEqual({
      required: true,
      trigger: ['change'],
      message: 'custom.serviceAccess.modeRequired'
    })
  })

  it('initializes with default form', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.form.name).toBe('')
    expect(state.form.auth_type).toBe('manual')
  })

  it('openModal sets service_plugin_id and fetches form', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    await state.openModal('svc-1')
    expect(state.service_plugin_id).toBe('svc-1')
    expect(hoisted.getServiceAccessForm).toHaveBeenCalledWith({ service_plugin_id: 'svc-1' })
  })

  it('openModal in edit mode sets isEdit', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    await state.openModal('svc-1', { name: 'Test', voucher: '{}' })
    expect(state.isEdit).toBe(true)
  })

  it('openModal in edit mode restores voucher values and auth type', async () => {
    hoisted.getServiceAccessForm.mockResolvedValue({ data: [{ type: 'input', dataKey: 'host' }] })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    await state.openModal('svc-1', {
      id: 'acc-1',
      name: 'Access 1',
      voucher: '{"host":"127.0.0.1","auth_type":"auto"}'
    })

    expect(state.serviceModals).toBe(true)
    expect(state.form.service_plugin_id).toBe('svc-1')
    expect(state.form.vouchers).toMatchObject({ host: '127.0.0.1', auth_type: 'auto' })
    expect(state.form.auth_type).toBe('auto')
  })

  it('openModal in edit mode tolerates malformed voucher JSON', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    await state.openModal('svc-1', {
      id: 'acc-1',
      name: 'Access 1',
      voucher: '{bad-json'
    })

    expect(state.serviceModals).toBe(true)
    expect(state.isEdit).toBe(true)
    expect(state.form.vouchers).toEqual({})
    expect(state.form.auth_type).toBe('manual')
  })

  it('does not open modal when form schema is missing', async () => {
    hoisted.getServiceAccessForm.mockResolvedValue({ data: null })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    await state.openModal('svc-1')

    expect(state.serviceModals).toBe(false)
  })

  it('close resets modal, form and step state', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.serviceModals = true
    state.isEdit = true
    state.currentStep = 2
    state.form.name = 'Access 1'
    state.form.vouchers = { host: '127.0.0.1' }

    state.close()

    expect(state.serviceModals).toBe(false)
    expect(state.isEdit).toBe(false)
    expect(state.currentStep).toBe(1)
    expect(state.form.name).toBe('')
    expect(state.form.vouchers).toEqual({})
  })

  it('creates manual access point and emits config step with created id', async () => {
    hoisted.createServiceDrop.mockResolvedValue({ data: { id: 'acc-1' }, error: null })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    state.form.name = 'Access 1'
    state.form.service_plugin_id = 'svc-1'
    state.form.auth_type = 'manual'
    state.form.vouchers = { username: 'user' }

    await state.submitSevice()
    await flushPromises()

    expect(hoisted.createServiceDrop).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Access 1',
        service_plugin_id: 'svc-1',
        voucher: '{"username":"user","auth_type":"manual"}'
      })
    )
    expect(wrapper.emitted('isEdit')?.[0]).toEqual(['{"username":"user","auth_type":"manual"}', 'acc-1', false])
    expect(state.serviceModals).toBe(false)
  })

  it('updates manual access point and emits config step with existing id', async () => {
    hoisted.putServiceDrop.mockResolvedValue({ data: { id: 'unused' }, error: null })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    state.isEdit = true
    state.form.id = 'acc-1'
    state.form.name = 'Access 1'
    state.form.auth_type = 'manual'
    state.form.vouchers = { username: 'user' }

    await state.submitSevice()
    await flushPromises()

    expect(hoisted.putServiceDrop).toHaveBeenCalledTimes(1)
    expect(hoisted.putServiceDrop).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'acc-1',
        name: 'Access 1',
        voucher: '{"username":"user","auth_type":"manual"}'
      })
    )
    expect(wrapper.emitted('isEdit')?.[0]).toEqual(['{"username":"user","auth_type":"manual"}', 'acc-1', true])
  })

  it('creates automatic access point, probes device list and emits automatic config row', async () => {
    hoisted.createServiceDrop.mockResolvedValue({ data: { id: 'acc-auto' }, error: null })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    state.form.name = 'Auto Access'
    state.form.auth_type = 'auto'
    state.form.vouchers = { token: 'secret' }

    await state.submitSevice()
    await flushPromises()

    expect(hoisted.getServiceListDrop).toHaveBeenCalledWith({
      voucher: '{"token":"secret","auth_type":"auto"}',
      service_type: '',
      page: 1,
      page_size: 10
    })
    expect(wrapper.emitted('isEdit')?.[0]).toEqual([
      '{"token":"secret","auth_type":"auto"}',
      { id: 'acc-auto', auth_type: 'auto', name: 'Auto Access' },
      true
    ])
  })

  it('does not persist automatic access when adapter probe returns an error', async () => {
    hoisted.getServiceListDrop.mockResolvedValue({ error: new Error('adapter unavailable') })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    state.serviceModals = true
    state.form.name = 'Auto Access'
    state.form.auth_type = 'auto'
    state.form.vouchers = { token: 'secret' }

    await state.submitSevice()
    await flushPromises()

    expect(hoisted.createServiceDrop).not.toHaveBeenCalled()
    expect(hoisted.putServiceDrop).not.toHaveBeenCalled()
    expect(wrapper.emitted('isEdit')).toBeUndefined()
    expect(state.serviceModals).toBe(true)
    expect(state.form.name).toBe('Auto Access')
  })

  it('does not persist automatic access when adapter probe rejects', async () => {
    hoisted.getServiceListDrop.mockRejectedValue(new Error('connection refused'))
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    state.serviceModals = true
    state.form.name = 'Auto Access'
    state.form.auth_type = 'auto'
    state.form.vouchers = { token: 'secret' }

    await state.submitSevice()
    await flushPromises()

    expect(hoisted.createServiceDrop).not.toHaveBeenCalled()
    expect(hoisted.putServiceDrop).not.toHaveBeenCalled()
    expect(wrapper.emitted('isEdit')).toBeUndefined()
    expect(state.serviceModals).toBe(true)
  })

  it('keeps the modal open when creating the access point fails', async () => {
    hoisted.createServiceDrop.mockResolvedValue({ data: null, error: new Error('save failed') })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    state.serviceModals = true
    state.form.name = 'Manual Access'
    state.form.auth_type = 'manual'
    state.form.vouchers = { token: 'secret' }

    await state.submitSevice()
    await flushPromises()

    expect(wrapper.emitted('isEdit')).toBeUndefined()
    expect(state.serviceModals).toBe(true)
    expect(state.form.name).toBe('Manual Access')
  })

  it('accepts a successful edit response without a returned id', async () => {
    hoisted.putServiceDrop.mockResolvedValue({ data: null, error: null })
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: (cb: (errors?: unknown) => void) => cb(false) }
    state.serviceModals = true
    state.isEdit = true
    state.form.id = 'acc-existing'
    state.form.name = 'Manual Access'
    state.form.auth_type = 'manual'
    state.form.vouchers = { token: 'secret' }

    await state.submitSevice()
    await flushPromises()

    expect(wrapper.emitted('isEdit')?.[0]).toEqual([
      '{"token":"secret","auth_type":"manual"}',
      'acc-existing',
      true
    ])
    expect(state.serviceModals).toBe(false)
  })

  it('does not submit when validation fails', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: (cb: (errors?: unknown) => void) => cb(true) }

    await state.submitSevice()
    await flushPromises()

    expect(hoisted.createServiceDrop).toHaveBeenCalledTimes(0)
    expect(hoisted.putServiceDrop).toHaveBeenCalledTimes(0)
  })
})
