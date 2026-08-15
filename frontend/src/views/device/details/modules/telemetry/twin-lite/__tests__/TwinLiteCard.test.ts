import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  expectMessageList: vi.fn(),
  getAttributeDataSet: vi.fn(),
  getDeviceTwin: vi.fn(),
  setDeviceTwinDesired: vi.fn()
}))

vi.mock('@/service/api', () => ({
  expectMessageList: hoisted.expectMessageList,
  getAttributeDataSet: hoisted.getAttributeDataSet,
  getDeviceTwin: hoisted.getDeviceTwin,
  setDeviceTwinDesired: hoisted.setDeviceTwinDesired
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import TwinLiteCard from '../TwinLiteCard.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const SlotStub = defineComponent({
  name: 'SlotStub',
  props: ['title', 'label', 'value', 'loading', 'data', 'columns', 'pagination', 'showIcon'],
  setup(props, { slots }) {
    return () =>
      h(
        'div',
        { class: 'slot-stub', 'data-title': props.title, 'data-label': props.label, 'data-value': props.value },
        slots.default?.()
      )
  }
})

const mountComponent = () => {
  const wrapper = shallowMount(TwinLiteCard, {
    props: {
      id: 'device-1',
      reportedTelemetry: [{ key: 'temperature', value: 25 }]
    },
    global: {
      stubs: {
        NCard: SlotStub,
        NButton: SlotStub,
        NAlert: SlotStub,
        NGrid: SlotStub,
        NGi: SlotStub,
        NStatistic: SlotStub,
        NDataTable: SlotStub,
        NModal: SlotStub,
        NForm: SlotStub,
        NFormItem: SlotStub,
        NSelect: SlotStub,
        NInput: SlotStub,
        NSpace: SlotStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('TwinLiteCard.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getDeviceTwin.mockResolvedValue({ data: null, error: null })
    hoisted.setDeviceTwinDesired.mockResolvedValue({ data: true, error: null })
    hoisted.expectMessageList.mockResolvedValue({
      data: {
        list: [{ send_type: 'telemetry', payload: '{"temperature":26}', status: 'pending' }]
      },
      error: null
    })
    hoisted.getAttributeDataSet.mockResolvedValue({ data: [{ key: 'device_name', value: 'Aether' }], error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads pending expected messages and attributes for the device', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.expectMessageList).toHaveBeenCalledWith({
      device_id: 'device-1',
      status: 'pending',
      page: 1,
      page_size: 100
    })
    expect(hoisted.getAttributeDataSet).toHaveBeenCalledWith({ device_id: 'device-1' })

    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.twinState.summary.desiredCount).toBe(1)
    expect(state.twinState.summary.deltaCount).toBe(1)
  })

  it('keeps rows empty when the API returns no lists', async () => {
    hoisted.expectMessageList.mockResolvedValue({ data: { list: [] }, error: null })
    hoisted.getAttributeDataSet.mockResolvedValue({ data: [], error: null })

    const wrapper = mountComponent()
    await flushPromises()

    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.previewRows).toEqual([])
    expect(state.twinState.summary.desiredCount).toBe(0)
  })

  it('prefers the backend twin aggregate when available', async () => {
    hoisted.getDeviceTwin.mockResolvedValue({
      data: {
        rows: [
          {
            key: 'temperature',
            label: 'temperature',
            source: 'telemetry',
            desired: 26,
            reported: 25,
            comparable: true,
            matched: false,
            status: 'pending'
          }
        ],
        summary: {
          desiredCount: 1,
          reportedCount: 1,
          matchedCount: 0,
          deltaCount: 1,
          unavailableCount: 0
        }
      },
      error: null
    })

    const wrapper = mountComponent()
    await flushPromises()

    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(hoisted.expectMessageList).not.toHaveBeenCalled()
    expect(state.previewRows).toHaveLength(1)
    expect(state.previewRows[0].desired).toBe(26)
  })

  it('submits desired changes through the twin endpoint', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = wrapper.vm.$.setupState as Record<string, any>
    state.openCreateDesired()
    state.desiredForm.source = 'attribute'
    state.desiredForm.key = 'fanMode'
    state.desiredForm.desiredText = '{"mode":"auto"}'

    await state.submitDesired()
    await flushPromises()

    expect(hoisted.setDeviceTwinDesired).toHaveBeenCalledWith('device-1', {
      source: 'attribute',
      key: 'fanMode',
      desired: { mode: 'auto' }
    })
  })
})
