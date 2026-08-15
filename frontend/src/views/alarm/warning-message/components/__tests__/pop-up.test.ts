/**
 * 文件用途：覆盖 pop-up 在 告警消息管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addWarningMessage: vi.fn(),
  editInfo: vi.fn(),
  getNotificationGroupList: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  loggerInfo: vi.fn(),
  validateMock: vi.fn()
}))

vi.mock('@/service/api/alarm', () => ({
  addWarningMessage: hoisted.addWarningMessage,
  editInfo: hoisted.editInfo
}))

vi.mock('@/service/api/notification', () => ({
  getNotificationGroupList: hoisted.getNotificationGroupList
}))

vi.mock('@/hooks/common/form', () => ({
  useNaiveForm: () => ({
    formRef: ref(null)
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    info: hoisted.loggerInfo
  })
}))

vi.mock('naive-ui', async importOriginal => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      success: hoisted.messageSuccess,
      error: hoisted.messageError
    })
  }
})

import PopUp from '../pop-up.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const baseEditData = {
  id: 'alarm-1',
  name: 'Alarm 1',
  alarm_level: 'H',
  alarm_repeat_time: 2,
  alarm_keep_time: 5,
  notification_group_id: 'group-1',
  enabled: 'Y',
  description: 'desc'
}

const mountComponent = (props: Record<string, any> = {}) => {
  const wrapper = shallowMount(PopUp, {
    props: {
      visible: true,
      type: 'add',
      editData: null,
      ...props
    },
    global: {
      stubs: {
        NModal: defineComponent({
          props: { show: Boolean },
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ setup() { return () => h('input') } }),
        NSelect: defineComponent({ setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
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

describe('pop-up.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getNotificationGroupList
      .mockResolvedValueOnce({
        data: {
          list: [
            { id: 'group-1', name: 'Open Group', status: 'OPEN' }
          ],
          total: 60
        }
      })
      .mockResolvedValue({
        data: {
          list: [
            { id: 'group-2', name: 'Closed Group', status: 'CLOSE' }
          ],
          total: 60
        }
      })
    hoisted.addWarningMessage.mockResolvedValue({ ok: true })
    hoisted.editInfo.mockResolvedValue({ data: true })
    hoisted.validateMock.mockImplementation(callback => callback(null))
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads notification groups on mount and appends the next page on scroll', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.state.generalOptions[0]).toMatchObject({
      id: 'group-1',
      name: 'Open Group',
      disabled: false
    })

    await setupState.notificationGroupHandleScroll({
      target: { scrollTop: 80, clientHeight: 20, scrollHeight: 100 }
    })
    await flushPromises()

    expect(hoisted.getNotificationGroupList).toHaveBeenCalledTimes(2)
    expect(setupState.state.generalOptions).toHaveLength(2)
    expect(setupState.state.generalOptions[1]).toMatchObject({
      id: 'group-2',
      name: 'Closed Group',
      disabled: true
    })
  })

  it('computes add and edit titles', async () => {
    const wrapper = mountComponent({ type: 'add' })
    await flushPromises()
    expect(getSetupState(wrapper).title).toBe('generate.addAlarm')

    await wrapper.setProps({ type: 'edit', editData: { ...baseEditData } })
    await flushPromises()
    expect(getSetupState(wrapper).title).toBe('generate.editAlarm')
  })

  it('fills edit data and stringifies numeric time fields', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    await wrapper.setProps({ type: 'edit', editData: { ...baseEditData } })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.formData.alarm_keep_time).toBe('5')
    expect(setupState.formData.alarm_repeat_time).toBe('2')
    expect(setupState.formData.name).toBe('Alarm 1')
  })

  it('resets form data when switching back to add mode', async () => {
    const wrapper = mountComponent({ type: 'edit', editData: { ...baseEditData } })
    await flushPromises()

    await wrapper.setProps({ type: 'add', editData: null })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.formData.name).toBe('')
    expect(setupState.formData.alarm_level).toBe('')
    expect(setupState.formData.notification_group_id).toBe('')
  })

  it('closeModal emits visibility update and refresh callback', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    getSetupState(wrapper).closeModal()

    expect(wrapper.emitted('update:visible')).toEqual([[false]])
    expect(wrapper.emitted('newEdit')).toEqual([[]])
    expect(wrapper.emitted('saved')).toBeUndefined()
  })

  it('submits add payload with numeric fields and reports success', async () => {
    hoisted.addWarningMessage.mockResolvedValueOnce({
      data: {
        id: 'alarm-new',
        name: 'New alarm'
      }
    })
    const wrapper = mountComponent({ type: 'add' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.formRef = {
      validate: (callback: (errors: unknown) => void) => hoisted.validateMock(callback)
    }
    setupState.formData.name = 'New alarm'
    setupState.formData.alarm_level = 'M'
    setupState.formData.alarm_repeat_time = '3'
    setupState.formData.alarm_keep_time = '6'
    setupState.formData.notification_group_id = 'group-1'
    setupState.formData.description = 'note'

    setupState.handleReset({ preventDefault: vi.fn() })
    await flushPromises()

    expect(hoisted.addWarningMessage).toHaveBeenCalledWith({
      name: 'New alarm',
      alarm_level: 'M',
      alarm_repeat_time: 3,
      alarm_keep_time: 6,
      notification_group_id: 'group-1',
      enabled: 'Y',
      description: 'note'
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.addSuccess')
    expect(wrapper.emitted('saved')).toEqual([
      [
        expect.objectContaining({
          id: 'alarm-new',
          name: 'New alarm',
          alarm_level: 'M',
          alarm_repeat_time: 3,
          alarm_keep_time: 6
        })
      ]
    ])
    expect(wrapper.emitted('newEdit')).toEqual([[]])
  })

  it('reports add failure when API returns a falsy result', async () => {
    hoisted.addWarningMessage.mockResolvedValueOnce(null)
    const wrapper = mountComponent({ type: 'add' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.formRef = {
      validate: (callback: (errors: unknown) => void) => hoisted.validateMock(callback)
    }
    setupState.formData.name = 'New alarm'
    setupState.formData.alarm_level = 'M'
    setupState.formData.alarm_repeat_time = '1'
    setupState.formData.alarm_keep_time = '2'

    setupState.handleReset({ preventDefault: vi.fn() })
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith('common.addFail')
  })

  it('submits edit payload and reports success', async () => {
    const wrapper = mountComponent({ type: 'edit', editData: { ...baseEditData } })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.formRef = {
      validate: (callback: (errors: unknown) => void) => hoisted.validateMock(callback)
    }
    setupState.formData.id = 'alarm-1'
    setupState.formData.name = 'Updated alarm'
    setupState.formData.alarm_level = 'L'
    setupState.formData.alarm_repeat_time = '4'
    setupState.formData.alarm_keep_time = '8'
    setupState.formData.notification_group_id = 'group-2'
    setupState.formData.description = 'updated'

    setupState.handleReset({ preventDefault: vi.fn() })
    await flushPromises()

    expect(hoisted.editInfo).toHaveBeenCalledWith({
      id: 'alarm-1',
      name: 'Updated alarm',
      alarm_level: 'L',
      alarm_repeat_time: 4,
      alarm_keep_time: 8,
      notification_group_id: 'group-2',
      enabled: 'Y',
      description: 'updated'
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.editSuccess')
  })

  it('reports edit failure when API response has no data', async () => {
    hoisted.editInfo.mockResolvedValueOnce({ data: null })
    const wrapper = mountComponent({ type: 'edit', editData: { ...baseEditData } })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.formRef = {
      validate: (callback: (errors: unknown) => void) => hoisted.validateMock(callback)
    }

    setupState.handleReset({ preventDefault: vi.fn() })
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith('common.editFail')
  })
})
