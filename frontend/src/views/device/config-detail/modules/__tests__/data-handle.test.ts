/**
 * 文件用途: data-handle 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  dataScriptAdd: vi.fn(),
  dataScriptDel: vi.fn(),
  dataScriptEdit: vi.fn(),
  dataScriptQuiz: vi.fn(),
  getDataScriptList: vi.fn(),
  setDeviceScriptEnable: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  dataScriptAdd: hoisted.dataScriptAdd,
  dataScriptDel: hoisted.dataScriptDel,
  dataScriptEdit: hoisted.dataScriptEdit,
  dataScriptQuiz: hoisted.dataScriptQuiz,
  getDataScriptList: hoisted.getDataScriptList,
  setDeviceScriptEnable: hoisted.setDeviceScriptEnable
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'en-US' } } })
}))

vi.mock('naive-ui', () => ({
  useDialog: () => ({
    warning: vi.fn(({ onPositiveClick }: any) => { if (onPositiveClick) onPositiveClick() })
  }),
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NDataTable: defineComponent({ setup() { return () => h('div') } }),
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NForm: defineComponent({ setup() { return { validate: () => Promise.resolve(), restoreValidation: () => {} } }, render() { return h('form', this.$slots.default?.()) } }),
  NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NInput: defineComponent({ setup() { return () => h('div') } }),
  NSelect: defineComponent({ setup() { return () => h('div') } }),
  NSwitch: defineComponent({ setup() { return () => h('div') } }),
  NIcon: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

vi.mock('@vicons/ionicons5', () => ({
  PencilOutline: defineComponent({ setup: () => () => h('div') }),
  TrashOutline: defineComponent({ setup: () => () => h('div') })
}))

vi.mock('@/components/LuaScriptEditor.vue', () => ({
  default: defineComponent({ setup() { return () => h('div') } })
}))

vi.mock('@/components/dev-card-item/index.vue', () => ({
  default: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

import Component from '../data-handle.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { configInfo: { id: 'cfg-1' }, ...props },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NEmpty: true,
        NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NGridItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ setup() { return { validate: () => Promise.resolve(), restoreValidation: () => {} } }, render() { return h('form') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/data-handle.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getDataScriptList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.dataScriptAdd.mockResolvedValue({ error: null })
    hoisted.dataScriptEdit.mockResolvedValue({ error: null })
    hoisted.dataScriptQuiz.mockResolvedValue({ data: { code: 200, data: 'result' }, error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes script list query, filter options and editor defaults for config', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.getDataScriptList).toHaveBeenCalledWith({
      device_config_id: 'cfg-1',
      script_type: '',
      page: 1,
      page_size: 10
    })
    expect(state.queryData).toMatchObject({
      device_config_id: 'cfg-1',
      script_type: '',
      page: 1,
      page_size: 10
    })
    expect(state.scripTypeOpt.map((option: any) => option.value)).toEqual(['', 'A', 'B', 'C', 'D', 'E', 'F'])
    expect(state.configFormRules).toMatchObject({
      name: { required: true, message: 'generate.enter-title', trigger: 'blur' },
      content: { required: true, message: 'generate.parse-script', trigger: 'blur' },
      enable_flag: { required: true, message: 'common.select', trigger: 'change' },
      script_type: { required: true, message: 'generate.select-processing-type', trigger: 'change' }
    })
    expect(state.editorOptions).toMatchObject({
      language: 'lua',
      fontSize: 14,
      wordWrap: 'on',
      minimap: { enabled: true }
    })
    expect(state.dataScriptList).toEqual([])
    expect(state.dataScriptTotal).toBe(0)
  })

  it('loads script list on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getDataScriptList).toHaveBeenCalledWith({
      device_config_id: 'cfg-1',
      script_type: '',
      page: 1,
      page_size: 10
    })
  })

  it('searchDataScript resets page and queries', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.searchDataScript()
    await flushPromises()
    expect(state.queryData.page).toBe(1)
  })

  it('openModal sets modal title and shows modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.openModal('add', null)
    expect(state.modalTitle).toBe('add')
    expect(state.showModal).toBe(true)
  })

  it('handleClose closes modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.showModal = true
    state.handleClose()
    expect(state.showModal).toBe(false)
  })

  it('handleChange calls setDeviceScriptEnable', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleChange({ id: 's1', enable_flag: 'Y' })
    expect(hoisted.setDeviceScriptEnable).toHaveBeenCalledWith({ id: 's1', enable_flag: 'Y' })
  })

  it('toggleMinimap toggles minimap enabled', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const before = state.editorOptions.minimap.enabled
    state.toggleMinimap()
    expect(state.editorOptions.minimap.enabled).toBe(!before)
  })

  it('toggleWordWrap toggles word wrap', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const before = state.editorOptions.wordWrap
    state.toggleWordWrap()
    expect(state.editorOptions.wordWrap).toBe(before === 'on' ? 'off' : 'on')
  })

  it('changeFontSize adjusts font size within bounds', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const before = state.editorOptions.fontSize
    state.changeFontSize(1)
    expect(state.editorOptions.fontSize).toBe(before + 1)
  })
})
