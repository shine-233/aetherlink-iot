import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceUpdate: vi.fn(),
  issueDeviceClaimToken: vi.fn(),
  redeemDeviceClaim: vi.fn(),
  createRdiShareToken: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceUpdate: hoisted.deviceUpdate,
  issueDeviceClaimToken: hoisted.issueDeviceClaimToken,
  redeemDeviceClaim: hoisted.redeemDeviceClaim
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
        (props.options as Array<{ value: number; label: string }>).map((option) =>
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
        NFlex: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NInput: InputStub,
        NButton: ButtonStub,
        NSelect: SelectStub,
        NAlert: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NText: defineComponent({
          setup(_, { slots }) {
            return () => h('span', slots.default?.())
          }
        })
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
    hoisted.issueDeviceClaimToken.mockResolvedValue({
      error: null,
      data: { token_id: 'tok-1', claim_key: 'ack_' + 'a'.repeat(48), expires_at: '2026-12-31T00:00:00Z' }
    })
    hoisted.redeemDeviceClaim.mockResolvedValue({
      error: null,
      data: { device_id: 'device-claimed', previous_tenant_id: 'tenant-a' }
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

  it('issues a one-time claim key and copies it immediately', async () => {
    const wrapper = mountComponent()
    const exposed = wrapper.vm.$.exposed as {
      openIssueClaimToken: (row: Record<string, unknown>) => void
    }
    const state = wrapper.vm.$.setupState as Record<string, any>
    const clipboard = navigator.clipboard as { writeText: ReturnType<typeof vi.fn> }

    exposed.openIssueClaimToken({ id: 'device-3', name: 'Sensor B', device_number: 'sn-3' })
    await flushPromises()
    expect(state.claimIssueVisible).toBe(true)
    expect(state.claimKey).toBe('')

    await state.generateClaimKey()
    await flushPromises()

    expect(hoisted.issueDeviceClaimToken).toHaveBeenCalledWith({
      device_id: 'device-3',
      ttl_seconds: 72 * 60 * 60
    })
    expect(state.claimKey).toBe('ack_' + 'a'.repeat(48))
    // 明文只出现一次：签发即自动复制。
    expect(clipboard.writeText).toHaveBeenCalledWith('ack_' + 'a'.repeat(48))
  })

  it('redeems a claim key and refreshes the device table', async () => {
    const wrapper = mountComponent()
    const exposed = wrapper.vm.$.exposed as {
      openClaimDevice: () => void
    }
    const state = wrapper.vm.$.setupState as Record<string, any>

    exposed.openClaimDevice()
    await flushPromises()
    expect(state.claimRedeemVisible).toBe(true)

    // 空表单直接拒绝（fail fast，不打请求）。
    await state.submitClaimRedeem()
    expect(hoisted.redeemDeviceClaim).not.toHaveBeenCalled()

    state.claimRedeemForm.device_number = ' sn-3 '
    state.claimRedeemForm.claim_key = ' ack_' + 'a'.repeat(48) + ' '
    await state.submitClaimRedeem()
    await flushPromises()

    expect(hoisted.redeemDeviceClaim).toHaveBeenCalledWith({
      device_number: 'sn-3',
      claim_key: 'ack_' + 'a'.repeat(48)
    })
    expect(state.claimRedeemVisible).toBe(false)
    expect(wrapper.emitted('updated')).toEqual([[]])
  })
})
