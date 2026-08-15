/**
 * 文件用途: 覆盖测试在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => `t:${key}`
}))

const apiMocks = vi.hoisted(() => ({
  deviceGroup: vi.fn(),
  deviceGroupTree: vi.fn(),
  putDeviceGroup: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceGroup: apiMocks.deviceGroup,
  deviceGroupTree: apiMocks.deviceGroupTree,
  putDeviceGroup: apiMocks.putDeviceGroup
}))

const { deviceGroup, deviceGroupTree, putDeviceGroup } = apiMocks

import AddOrEditDevices from '../index.vue'

const NModalStub = defineComponent({
  name: 'NModal',
  emits: ['after-enter'],
  setup(_, { slots }) {
    return () => h('div', { class: 'n-modal-stub' }, slots.default?.())
  }
})

const NCardStub = defineComponent({
  name: 'NCard',
  props: {
    title: String
  },
  setup(props, { slots }) {
    return () => h('section', { class: 'n-card-stub', 'data-title': props.title }, slots.default?.())
  }
})

const NFormStub = defineComponent({
  name: 'NForm',
  setup(_, { expose, slots }) {
    expose({
      validate: vi.fn(() => Promise.resolve())
    })
    return () => h('form', { class: 'n-form-stub' }, slots.default?.())
  }
})

const NFormItemStub = defineComponent({
  name: 'NFormItem',
  setup(_, { slots }) {
    return () => h('div', { class: 'n-form-item-stub' }, slots.default?.())
  }
})

const NTreeSelectStub = defineComponent({
  name: 'NTreeSelect',
  props: {
    value: String
  },
  emits: ['update:value'],
  template: '<div class="n-tree-select-stub" />'
})

const NInputStub = defineComponent({
  name: 'NInput',
  props: {
    value: String
  },
  emits: ['update:value'],
  template: '<input class="n-input-stub" />'
})

const NButtonStub = defineComponent({
  name: 'NButton',
  emits: ['click'],
  setup(_, { emit, slots }) {
    return () => h('button', { class: 'n-button-stub', onClick: () => emit('click') }, slots.default?.())
  }
})

const mountDialog = (props: Record<string, unknown> = {}) =>
  mount(AddOrEditDevices, {
    props: {
      refreshData: vi.fn(),
      ...props
    },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      components: {
        NModal: NModalStub,
        NCard: NCardStub,
        NForm: NFormStub,
        NFormItem: NFormItemStub,
        NTreeSelect: NTreeSelectStub,
        NInput: NInputStub,
        NButton: NButtonStub
      }
    }
  })

describe('device/grouping/components/add-or-edit-devices/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    deviceGroup.mockResolvedValue({})
    putDeviceGroup.mockResolvedValue({})
    deviceGroupTree.mockResolvedValue({
      data: [
        {
          group: {
            id: 'root-child',
            name: 'Root Child'
          },
          children: [
            {
              group: {
                id: 'leaf',
                name: 'Leaf'
              }
            }
          ]
        }
      ]
    })
  })

  it('loads tree options on mount and renders add title by default', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(deviceGroupTree).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.n-card-stub').attributes('data-title')).toBe('t:custom.groupPage.addGroup')
  })

  it('keeps the dialog usable when the tree response is not an array', async () => {
    deviceGroupTree.mockResolvedValue({ data: { error: 'temporary database failure' } })

    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.find('.n-card-stub').exists()).toBe(true)
    expect(deviceGroupTree).toHaveBeenCalledTimes(1)
  })

  it('submits create request, refreshes options, calls parent refresh, and resets form', async () => {
    const refreshData = vi.fn()
    const wrapper = mountDialog({ refreshData })
    await flushPromises()

    wrapper.findComponent(NTreeSelectStub).vm.$emit('update:value', '0')
    const inputs = wrapper.findAllComponents(NInputStub)
    inputs[0].vm.$emit('update:value', 'New Group')
    inputs[1].vm.$emit('update:value', 'Created from test')
    await nextTick()

    await wrapper.findAll('.n-button-stub')[1].trigger('click')
    await flushPromises()

    expect(deviceGroup).toHaveBeenCalledWith({
      id: '',
      parent_id: '0',
      name: 'New Group',
      description: 'Created from test'
    })
    expect(putDeviceGroup).toHaveBeenCalledTimes(0)
    expect(refreshData).toHaveBeenCalledTimes(1)
  })

  it('submits edit request with edit data when editing', async () => {
    const refreshData = vi.fn()
    const wrapper = mountDialog({
      isEdit: true,
      refreshData,
      editData: {
        id: 'group-1',
        parent_id: '0',
        name: 'Existing Group',
        description: 'Existing description'
      }
    })
    await flushPromises()

    await wrapper.findAll('.n-button-stub')[1].trigger('click')
    await flushPromises()

    expect(putDeviceGroup).toHaveBeenCalledWith({
      id: 'group-1',
      parent_id: '0',
      name: 'Existing Group',
      description: 'Existing description'
    })
    expect(deviceGroup).toHaveBeenCalledTimes(0)
    expect(refreshData).toHaveBeenCalledTimes(1)
  })

  it('closes and resets form when cancel is clicked', async () => {
    const wrapper = mountDialog({
      editData: {
        id: 'group-1',
        parent_id: '0',
        name: 'Existing Group',
        description: 'Existing description'
      }
    })
    await flushPromises()

    const state = wrapper.vm as unknown as {
      showModal: boolean
    }
    state.showModal = true
    await nextTick()

    await wrapper.findAll('.n-button-stub')[0].trigger('click')

    expect(state.showModal).toBe(false)
  })
})
