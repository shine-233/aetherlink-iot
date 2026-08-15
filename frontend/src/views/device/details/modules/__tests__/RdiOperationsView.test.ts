import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import RdiOperationsView from '../RdiOperationsView.vue'
import {
  RDI_DRY_CONTACT_DELAY_MAX_SECONDS,
  RDI_DRY_CONTACT_DELAY_MIN_SECONDS,
  RDI_DRY_CONTACT_TEST_DURATION_MAX_SECONDS,
  RDI_DRY_CONTACT_TEST_DURATION_MIN_SECONDS
} from '../rdi/constants/rdi-ranges'

const SlotStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const AlertStub = defineComponent({
  name: 'NAlert',
  setup(_, { slots }) {
    return () => h('div', { class: 'alert-stub' }, slots.default?.())
  }
})

const ButtonStub = defineComponent({
  name: 'NButton',
  props: ['loading', 'disabled'],
  emits: ['click'],
  setup(_, { slots }) {
    return () => h('button', slots.default?.())
  }
})

const NumberInputStub = defineComponent({
  name: 'NInputNumber',
  props: ['value', 'min', 'max'],
  emits: ['update:value'],
  setup() {
    return () => h('input', { type: 'number' })
  }
})

function mountComponent(propOverrides: Record<string, unknown> = {}) {
  return shallowMount(RdiOperationsView, {
    props: {
      commandLoading: false,
      dryCommandDelay: 0,
      dryTestDuration: 1,
      commandTrackingSummary: '',
      otaPackageLoading: false,
      otaPackageId: '',
      latestFirmwareLoading: false,
      latestFirmwarePackage: null,
      otaCommand: {
        firmware_url: '',
        version: '',
        size: null,
        md5: ''
      },
      otaPackageOptions: [],
      otaMissingFieldLabels: [],
      canSendOtaUpgrade: false,
      shareLoading: false,
      shareExpiresIn: 604800,
      shareLink: '',
      shareExpiryOptions: [],
      shareExpiresAt: '',
      t: (key: string) => key,
      ...propOverrides
    },
    global: {
      stubs: {
        NFormItem: SlotStub,
        NInputNumber: NumberInputStub,
        NButton: ButtonStub,
        NSelect: true,
        NInput: true,
        NAlert: AlertStub,
        NPopconfirm: SlotStub
      }
    }
  })
}

describe('RdiOperationsView.vue', () => {
  it('uses centralized dry contact duration ranges for command inputs', () => {
    const wrapper = mountComponent()

    const [dryCommandDelay, dryTestDuration] = wrapper.findAllComponents(NumberInputStub)

    expect(dryCommandDelay.props('min')).toBe(RDI_DRY_CONTACT_DELAY_MIN_SECONDS)
    expect(dryCommandDelay.props('max')).toBe(RDI_DRY_CONTACT_DELAY_MAX_SECONDS)
    expect(dryTestDuration.props('min')).toBe(RDI_DRY_CONTACT_TEST_DURATION_MIN_SECONDS)
    expect(dryTestDuration.props('max')).toBe(RDI_DRY_CONTACT_TEST_DURATION_MAX_SECONDS)
  })

  it('renders command tracking summary when provided', () => {
    const wrapper = mountComponent({
      commandTrackingSummary: 'ota_upgrade message_id=mid-1 status=queued (logged)'
    })

    expect(wrapper.text()).toContain('message_id=mid-1')
  })
})
