/**
 * 文件用途: 新增/编辑弹窗组件（package-form-modal）的单元测试。
 * 核心逻辑: 覆盖事件上抛（save/upload/select-file/search-device-configs/update:show）、
 *   文件拖拽选择、保存按钮文案随回跳流程切换、以及重新打开时清空原生 input 与拖拽态。
 * 关键注意事项: naive 组件用替身渲染，事件经由替身实例 $emit 触发父级监听。
 * 重构建议: 表单校验入组件后，补充校验态渲染断言。
 */
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import PackageFormModal from '../package-form-modal.vue'
import type { OtaPackageFormModel } from '../use-ota-package-form'

const NModalStub = defineComponent({
  props: { show: Boolean },
  emits: ['update:show'],
  setup(_, { slots }) {
    return () => h('div', { class: 'modal-stub' }, [slots.default?.(), slots.footer?.()])
  }
})

const NInputStub = defineComponent({
  props: { value: { default: '' }, type: { default: 'text' } },
  emits: ['update:value'],
  setup() {
    return () => h('div')
  }
})

const NSelectStub = defineComponent({
  props: ['value', 'options', 'loading', 'filterable', 'remote', 'placeholder'],
  emits: ['update:value', 'search'],
  setup() {
    return () => h('div')
  }
})

const NButtonStub = defineComponent({
  props: ['loading', 'disabled', 'type', 'size', 'secondary'],
  emits: ['click'],
  setup(props, { slots, emit }) {
    return () => h('button', { disabled: props.disabled, onClick: () => emit('click') }, slots.default?.())
  }
})

const NSpaceStub = defineComponent({
  props: ['vertical', 'align', 'justify'],
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const NFormStub = defineComponent({
  props: ['labelPlacement'],
  setup(_, { slots }) {
    return () => h('form', slots.default?.())
  }
})

const NGridStub = defineComponent({
  props: ['cols', 'responsive', 'xGap'],
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const NFormItemStub = defineComponent({
  props: ['label', 'required', 'path'],
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const globalStubs = {
  stubs: {
    NModal: NModalStub,
    NForm: NFormStub,
    NGrid: NGridStub,
    NFormItemGi: NFormItemStub,
    NFormItem: NFormItemStub,
    NInput: NInputStub,
    NSelect: NSelectStub,
    NButton: NButtonStub,
    NSpace: NSpaceStub
  }
}

function createFormModel(overrides: Partial<OtaPackageFormModel> = {}): OtaPackageFormModel {
  return {
    id: '',
    name: '',
    version: '',
    target_version: '',
    device_config_id: null,
    module: '',
    package_type: 2,
    signature_type: 'MD5',
    package_url: '',
    additional_info: '{}',
    description: '',
    remark: '',
    ...overrides
  }
}

function mountModal(props: Record<string, unknown> = {}) {
  return mount(PackageFormModal, {
    props: {
      show: true,
      isEditing: false,
      saving: false,
      uploading: false,
      form: createFormModel(),
      selectedFile: null,
      packageTypeOptions: [
        { label: 'page.product.update-package.diff', value: 1 },
        { label: 'page.product.update-package.full', value: 2 }
      ],
      signatureOptions: [{ label: 'MD5', value: 'MD5' }],
      deviceConfigLoading: false,
      deviceConfigOptions: [],
      isReturnToOtaTaskFlow: false,
      ...props
    },
    global: globalStubs
  })
}

describe('package-form-modal', () => {
  it('shows the add title and switches the save label for the OTA return flow', async () => {
    const wrapper = mountModal()
    const buttons = wrapper.findAll('button')
    const saveButton = buttons[buttons.length - 1]
    expect(saveButton.text()).toBe('common.save')

    await wrapper.setProps({ isReturnToOtaTaskFlow: true })
    expect(saveButton.text()).toBe('page.product.update-package.returnToOtaSaveAction')
  })

  it('emits save and upload events from the footer actions', async () => {
    // 传入已选文件，否则上传按钮处于 disabled 态，点击不会触发 upload 事件
    const wrapper = mountModal({ selectedFile: new File(['abc'], 'pkg.bin') })
    const buttons = wrapper.findAll('button')
    const saveButton = buttons[buttons.length - 1]
    // 渲染顺序为 [上传(表单区), 取消(footer), 保存(footer)]，上传按钮固定在首位
    const uploadButton = buttons[0]

    await uploadButton.trigger('click')
    expect(wrapper.emitted('upload')).toHaveLength(1)

    await saveButton.trigger('click')
    expect(wrapper.emitted('save')).toHaveLength(1)
  })

  it('disables the upload button while no file is selected', () => {
    const wrapper = mountModal()
    const uploadButton = wrapper.findAll('button')[0]
    expect(uploadButton.attributes('disabled')).toBeDefined()

    const wrapperWithFile = mountModal({ selectedFile: new File(['abc'], 'pkg.bin') })
    expect(wrapperWithFile.findAll('button')[0].attributes('disabled')).toBeUndefined()
  })

  it('emits select-file when a file is picked via the native input', async () => {
    const wrapper = mountModal()
    const input = wrapper.find('input[type="file"]')
    const file = new File(['abc'], 'pkg.bin')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })

    await input.trigger('change')

    expect(wrapper.emitted('select-file')?.[0]).toEqual([file])
  })

  it('emits select-file on file drop and clears the dragging state', async () => {
    const wrapper = mountModal()
    const dropZone = wrapper.find('.file-drop-zone')
    const file = new File(['abc'], 'dropped.bin')

    await dropZone.trigger('dragenter')
    await dropZone.trigger('drop', { dataTransfer: { files: [file] } })

    expect(wrapper.emitted('select-file')?.[0]).toEqual([file])
    expect(dropZone.classes()).not.toContain('is-dragging')
  })

  it('clears the native input and drag state each time the modal opens', async () => {
    const wrapper = mountModal({ show: false })
    const input = wrapper.find('input[type="file"]')
    // happy-dom 只允许把 file input 的 value 写成空串，这里拦截 value 写入以验证组件的清空赋值
    let recordedValue = 'C:\\fakepath\\pkg.bin'
    Object.defineProperty(input.element, 'value', {
      configurable: true,
      get: () => recordedValue,
      set: (next: string) => {
        recordedValue = next
      }
    })
    await wrapper.find('.file-drop-zone').trigger('dragenter')
    expect(wrapper.find('.file-drop-zone').classes()).toContain('is-dragging')

    await wrapper.setProps({ show: true })
    await nextTick()

    expect(recordedValue).toBe('')
    expect(input.element.value).toBe('')
    expect(wrapper.find('.file-drop-zone').classes()).not.toContain('is-dragging')
  })

  it('forwards device config search and show updates', async () => {
    const wrapper = mountModal()
    const deviceConfigSelect = wrapper.findComponent(NSelectStub)

    await deviceConfigSelect.vm.$emit('search', 'cfg')
    expect(wrapper.emitted('search-device-configs')?.[0]).toEqual(['cfg'])

    const modal = wrapper.findComponent(NModalStub)
    await modal.vm.$emit('update:show', false)
    expect(wrapper.emitted('update:show')?.[0]).toEqual([false])
  })
})
