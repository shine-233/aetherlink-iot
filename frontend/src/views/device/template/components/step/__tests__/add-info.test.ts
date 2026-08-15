/**
 * 文件用途: 测试物模型基础信息步骤。
 * 核心逻辑: 模拟表单输入、图片上传和物模型保存接口，验证基础信息提交。
 * 关键注意事项: 上传和附加信息字段容易产生格式漂移，测试要覆盖序列化结果。
 * 重构建议: 抽出物模型基础信息 fixture，复用到新增和编辑测试。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addTemplat: vi.fn(),
  getTemplat: vi.fn(),
  putTemplat: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  addTemplat: hoisted.addTemplat,
  getTemplat: hoisted.getTemplat,
  putTemplat: hoisted.putTemplat
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/storage', () => ({
  localStg: { get: vi.fn(() => 'token') }
}))

vi.mock('@/utils/common/tool', () => ({
  getPlatformApiBaseUrl: () => 'http://localhost/api/v1'
}))

import Component from '../add-info.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { stepCurrent: 1, modalVisible: false, deviceTemplateId: '', ...props },
    global: {
      stubs: {
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NUpload: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NIcon: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } }),
        NUploadDragger: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        SvgIcon: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/template/components/step/add-info.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.addTemplat.mockResolvedValue({ data: { id: 'tpl-1' }, error: null })
    hoisted.getTemplat.mockResolvedValue({ data: { name: 'test' }, error: null })
    hoisted.putTemplat.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes basic template form, upload target and required name rule', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.deviceTemplateId).toBe('')
    expect(state.addFrom).toEqual({
      name: '',
      templateTage: [],
      version: '',
      author: '',
      description: '',
      path: '',
      label: '',
      brand: '',
      model_number: '',
      id: ''
    })
    expect(state.fromRules).toEqual({
      name: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.enterTemplateName'
      }
    })
    expect(state.platformApiBaseUrl).toBe('http://localhost/api/v1')
    expect(state.platformAssetBaseUrl).toBe('http://localhost/api/v1')
    expect(state.pngPath).toBe('')
  })

  it('addTags sets tageFlag to true', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.addTags()
    expect(state.tageFlag).toBe(true)
  })

  it('tagBlur pushes tag text when not empty', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.addTageText = 'newtag'
    state.tagBlur()
    expect(state.addFrom.templateTage).toContain('newtag')
    expect(state.addTageText).toBe('')
  })
})
