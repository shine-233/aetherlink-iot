/**
 * 文件用途：覆盖 widget-bundle-form-modal.vue 的创建/编辑提交契约。
 * 核心逻辑：部件定义 JSON 校验失败时拦截提交不发请求；合法时按创建/更新两条路径提交
 *   并 emit saved + update:show(false)；编辑打开时按行数据回填表单。
 * 关键注意事项：本套件只覆盖前端弹窗行为，不证明后端 ValidateWidgetDefinition 链路。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  createWidgetBundle: vi.fn(),
  updateWidgetBundle: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn()
}))

vi.mock('@/service/api', () => ({
  createWidgetBundle: hoisted.createWidgetBundle,
  updateWidgetBundle: hoisted.updateWidgetBundle
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

vi.mock('naive-ui', () => {
  const stub = (name: string, tag = 'div') =>
    defineComponent({
      name,
      inheritAttrs: false,
      setup(_, { attrs, slots }) {
        return () => h(tag, attrs, slots.default?.())
      }
    })
  return {
    NButton: stub('NButton', 'button'),
    NForm: stub('NForm'),
    NFormItem: stub('NFormItem'),
    NInput: stub('NInput', 'input'),
    NModal: stub('NModal'),
    NSpace: stub('NSpace'),
    useMessage: () => ({ success: hoisted.messageSuccess, error: hoisted.messageError, info: vi.fn() })
  }
})

// shallowMount 的自动桩会吞掉 naive-ui mock 里 NForm 的 expose(validate)，
// 这里用显式 stub 覆盖，保证 formRef.value?.validate() 在测试环境可等待。
const FormStubWithValidate = defineComponent({
  name: 'NForm',
  inheritAttrs: false,
  setup(_, { attrs, slots, expose }) {
    expose({ validate: () => Promise.resolve(true) })
    return () => h('form', attrs, slots.default?.())
  }
})

import WidgetBundleFormModal from '../modules/widget-bundle-form-modal.vue'
import type { WidgetBundleItem } from '@/service/api'

const editingFixture: WidgetBundleItem = {
  id: 'wb-1',
  name: '旧部件库',
  tenant_id: 'tenant-1',
  widgets: '[{"type":"chart","version":"1"}]',
  description: '旧描述',
  version: '2.0.0',
  type_key: 'sensor'
}

const mountedWrappers: Array<VueWrapper> = []

function mountModal(editing: WidgetBundleItem | null = null) {
  const wrapper = shallowMount(WidgetBundleFormModal, {
    props: { show: true, editing },
    global: {
      stubs: { NForm: FormStubWithValidate }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function getSetupState(wrapper: VueWrapper) {
  return wrapper.vm.$.setupState as unknown as Record<string, any>
}

describe('widget-bundle-form-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.createWidgetBundle.mockResolvedValue({ error: null })
    hoisted.updateWidgetBundle.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('blocks submit and skips the create request when widgets JSON is invalid', async () => {
    const wrapper = mountModal()
    const state = getSetupState(wrapper)
    state.formModel.name = '新部件库'
    state.formModel.widgets = 'not-json'

    await state.handleSubmit()
    await flushPromises()

    expect(hoisted.createWidgetBundle).not.toHaveBeenCalled()
    expect(hoisted.messageError).toHaveBeenCalledWith('page.widgetBundle.widgetsInvalidJson')
    expect(wrapper.emitted('saved')).toBeUndefined()
  })

  it('creates a bundle with normalized payload and emits saved + close on success', async () => {
    const wrapper = mountModal()
    const state = getSetupState(wrapper)
    state.formModel.name = '新部件库'
    state.formModel.type_key = 'sensor'
    state.formModel.widgets = '[{"type":"gauge","version":"1.0.0"}]'

    await state.handleSubmit()
    await flushPromises()

    expect(hoisted.createWidgetBundle).toHaveBeenCalledWith({
      name: '新部件库',
      version: '1.0.0',
      type_key: 'sensor',
      description: undefined,
      widgets: '[{"type":"gauge","version":"1.0.0"}]'
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('page.widgetBundle.saveSuccess')
    expect(wrapper.emitted('saved')).toBeTruthy()
    expect(wrapper.emitted('update:show')?.at(-1)).toEqual([false])
  })

  it('prefills the form from the editing row and submits through the update path', async () => {
    const wrapper = mountModal(editingFixture)
    const state = getSetupState(wrapper)
    expect(state.formModel.name).toBe('旧部件库')
    expect(state.formModel.version).toBe('2.0.0')
    expect(state.formModel.widgets).toBe('[{"type":"chart","version":"1"}]')

    await state.handleSubmit()
    await flushPromises()

    expect(hoisted.updateWidgetBundle).toHaveBeenCalledWith({
      id: 'wb-1',
      name: '旧部件库',
      version: '2.0.0',
      type_key: 'sensor',
      description: '旧描述',
      widgets: '[{"type":"chart","version":"1"}]'
    })
    expect(hoisted.createWidgetBundle).not.toHaveBeenCalled()
    expect(wrapper.emitted('saved')).toBeTruthy()
  })
})
