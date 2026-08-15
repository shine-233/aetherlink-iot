/**
 * 文件用途: extend-info 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfigEdit: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigEdit: hoisted.deviceConfigEdit
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }),
  NButton: defineComponent({
    emits: ['click'],
    setup(_, { slots, emit }) {
      return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
    }
  }),
  NPopconfirm: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  NSpace: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  NSwitch: defineComponent({
    props: { value: { default: false } },
    emits: ['update:value'],
    setup() {
      return () => h('div')
    }
  }),
  NDataTable: defineComponent({
    props: { data: { type: Array, default: () => [] } },
    setup() {
      return () => h('div')
    }
  }),
  NForm: defineComponent({
    setup() {
      return { validate: () => Promise.resolve(), restoreValidation: () => {} }
    },
    render() {
      return h('form', this.$slots.default?.())
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
  NModal: defineComponent({
    props: { show: Boolean },
    emits: ['update:show'],
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  }),
  NFlex: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  })
}))

import Component from '../extend-info.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { configInfo: { id: 'cfg-1', additional_info: '[]' }, ...props },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NForm: defineComponent({
          setup() {
            return { validate: () => Promise.resolve(), restoreValidation: () => {} }
          },
          render() {
            return h('form')
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/extend-info.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceConfigEdit.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes extension info list, type options and required field rules', async () => {
    const wrapper = mountComponent({
      configInfo: {
        id: 'cfg-1',
        additional_info: '[{"name":"battery","type":"Number","default_value":"0","desc":"Battery","enable":true}]'
      }
    })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.visible).toBe(false)
    expect(state.isEdit).toBe(false)
    expect(state.editIndex).toBe(-1)
    expect(state.extendForm).toEqual({
      name: null,
      type: null,
      default_value: null,
      desc: null,
      enable: false
    })
    expect(state.extendInfoList).toEqual([
      { name: 'battery', type: 'Number', default_value: '0', desc: 'Battery', enable: true }
    ])
    expect(state.typeOptions.map((option: any) => option.value)).toEqual(['String', 'Number', 'Boolean'])
    expect(state.extendFormRules).toMatchObject({
      name: { required: true, message: 'common.enterName', trigger: 'blur' },
      type: { required: true, message: 'generate.select-type', trigger: 'change' }
    })
    expect(state.columns.map((column: any) => column.key)).toEqual([
      'name',
      'type',
      'default_value',
      'desc',
      'enable',
      'operate'
    ])
  })

  it('falls back to an empty extension list when additional_info JSON is malformed', async () => {
    const wrapper = mountComponent({
      configInfo: {
        id: 'cfg-1',
        additional_info: '{bad-json'
      }
    })
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(state.extendInfoList).toEqual([])
  })

  it('addDevice opens modal', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.addDevice()
    expect(state.visible).toBe(true)
  })

  it('handleClose resets form and closes modal', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.visible = true
    state.isEdit = true
    state.editIndex = 0
    state.handleClose()
    expect(state.visible).toBe(false)
    expect(state.isEdit).toBe(false)
    expect(state.editIndex).toBe(-1)
  })

  it('handleSubmit adds new item to list when not editing', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.extendFormRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.editIndex = -1
    state.extendForm = { name: 'test', type: 'String', default_value: 'val', desc: 'desc', enable: false }
    await state.handleSubmit()
    expect(state.extendInfoList).toEqual([
      { name: 'test', type: 'String', default_value: 'val', desc: 'desc', enable: false }
    ])
  })

  it('handleSubmit updates existing item when editing', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.extendFormRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    state.extendInfoList = [{ name: 'old', type: 'String', default_value: 'val', desc: 'desc', enable: false }]
    state.editIndex = 0
    state.extendForm = { name: 'new', type: 'String', default_value: 'val', desc: 'desc', enable: false }
    await state.handleSubmit()
    expect(state.extendInfoList[0].name).toBe('new')
  })
})
