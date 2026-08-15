import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceUpdate: vi.fn(),
  createRdiShareToken: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceUpdate: hoisted.deviceUpdate
}))

vi.mock('@/service/api/rdi', () => ({
  createRdiShareToken: hoisted.createRdiShareToken
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import DeviceManageQuickActions from '../DeviceManageQuickActions.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const ModalStub = defineComponent({
  name: 'NModal',
  props: {
    show: { type: Boolean, default: false }
  },
  setup(props, { slots }) {
    return () => (props.show ? h('div', { class: 'modal-stub' }, slots.default?.()) : null)
  }
})

const InputStub = defineComponent({
  name: 'NInput',
  props: {
    value: { type: String, default: '' }
  },
  emits: ['update:value'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        value: props.value,
        onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value)
      })
  }
})

const ButtonStub = defineComponent({
  name: 'NButton',
  props: {
    loading: { type: Boolean, default: false }
  },
  emits: ['click'],
  setup(_, { emit, slots }) {
    return () => h('button', { type: 'button', onClick: () => emit('click') }, slots.default?.())
  }
})

const SelectStub = defineComponent({
  name: 'NSelect',
  props: {
    value: { type: Number, default: null },
    options: { type: Array, default: () => [] }
  },
  emits: ['update:value'],
  setup(props, { emit }) {
    return () =>
      h(
        'select',
        {
          value: props.value ?? '',
          onChange: (event: Event) => emit('update:value', Number((event.target as HTMLSelectElement).value))
        },
        (props.options as Array<{ value: number; label: string }>).map(option =>
          h('option', { value: option.value }, option.label)
        )
      )
  }
})

const mountComponent = () => {
  const wrapper = mount(DeviceManageQuickActions, {
    global: {
      stubs: {
        NModal: ModalStub,
        NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: InputStub,
        NButton: ButtonStub,
        NSelect: SelectStub,
        NAlert: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NText: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('DeviceManageQuickActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceUpdate.mockResolvedValue({ error: null })
    hoisted.createRdiShareToken.mockResolvedValue({
      error: null,
      data: { token: 'share-token-1', share_path: '/device/share?share_token=share-token-1' }
    })
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: vi.fn().mockResolvedValue(undefined)
      }
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('edits device name and description through the exposed quick action', async () => {
    const wrapper = mountComponent()
    const exposed = wrapper.vm.$.exposed as {
      openEditDevice: (row: Record<string, unknown>) => void
    }
    const state = wrapper.vm.$.setupState as Record<string, any>

    exposed.openEditDevice({ id: 'device-1', name: 'Old Name', description: 'Old Desc' })
    await flushPromises()

    expect(state.editDeviceVisible).toBe(true)
    state.editDeviceForm.name = '  New Name  '
    state.editDeviceForm.description = '  New Desc  '
    await state.saveDeviceEdit()
    await flushPromises()

    expect(hoisted.deviceUpdate).toHaveBeenCalledWith({
      id: 'device-1',
      name: 'New Name',
      description: 'New Desc'
    })
    expect(wrapper.emitted('updated')).toEqual([[]])
    expect(state.editDeviceVisible).toBe(false)
  })

  it('generates and copies a share link through the exposed quick action', async () => {
    const wrapper = mountComponent()
    const exposed = wrapper.vm.$.exposed as {
      openShareDevice: (row: Record<string, unknown>) => void
    }
    const state = wrapper.vm.$.setupState as Record<string, any>
    const clipboard = navigator.clipboard as { writeText: ReturnType<typeof vi.fn> }

    exposed.openShareDevice({ id: 'device-2', name: 'Pump A' })
    await flushPromises()
    await state.generateShareLink()
    await flushPromises()

    expect(hoisted.createRdiShareToken).toHaveBeenCalledWith('device-2', { expires_in: 7 * 24 * 60 * 60 })
    expect(state.shareLink).toContain('/device/share?share_token=share-token-1')
    expect(clipboard.writeText).toHaveBeenCalledWith(state.shareLink)
  })
})
