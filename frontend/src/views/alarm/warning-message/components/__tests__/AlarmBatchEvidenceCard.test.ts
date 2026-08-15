import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AlarmBatchEvidenceCard from '../AlarmBatchEvidenceCard.vue'
import type { AlarmBatchActionEvidence } from '../alarm-configuration.helpers'

const CardStub = defineComponent({
  name: 'NCard',
  setup(_, { slots }) {
    return () => h('section', slots.default?.())
  }
})

const FlexStub = defineComponent({
  name: 'NFlex',
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const TagStub = defineComponent({
  name: 'NTag',
  setup(_, { slots }) {
    return () => h('span', slots.default?.())
  }
})

const ButtonStub = defineComponent({
  name: 'NButton',
  emits: ['click'],
  setup(_, { attrs, emit, slots }) {
    return () =>
      h(
        'button',
        {
          ...attrs,
          onClick: () => emit('click')
        },
        slots.default?.()
      )
  }
})

const mountCard = (overrides: Partial<AlarmBatchActionEvidence> = {}) => {
  const evidence: AlarmBatchActionEvidence = {
    action: 'acknowledge',
    generatedAt: '2026-07-07T10:00:00.000Z',
    expectedCount: 3,
    successCount: 3,
    failureCount: 0,
    note: 'operator confirmed',
    failedItems: [],
    summary: '3 succeeded / 0 failed',
    detail: 'All selected rows accepted the platform action.',
    copyText: 'copyable evidence packet',
    type: 'success',
    ...overrides
  }

  return shallowMount(AlarmBatchEvidenceCard, {
    props: {
      evidence,
      actionLabel: 'Acknowledge selected'
    },
    global: {
      mocks: {
        $t: (key: string) => key
      },
      stubs: {
        NCard: CardStub,
        NFlex: FlexStub,
        NTag: TagStub,
        NButton: ButtonStub
      }
    }
  })
}

describe('AlarmBatchEvidenceCard', () => {
  it('renders the latest batch evidence summary and no-failure state', async () => {
    const wrapper = mountCard()

    expect(wrapper.text()).toContain('custom.alarmPage.batchActionEvidenceTitle')
    expect(wrapper.text()).toContain('3 succeeded / 0 failed')
    expect(wrapper.text()).toContain('Acknowledge selected')
    expect(wrapper.text()).toContain('2026-07-07T10:00:00.000Z')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('operator confirmed')
    expect(wrapper.text()).toContain('All selected rows accepted the platform action.')
    expect(wrapper.text()).toContain('custom.alarmPage.batchActionNoFailedRows')

    const buttons = wrapper.findAll('button')
    await buttons[0].trigger('click')
    await wrapper.get('[data-testid="alarm-download-batch-evidence"]').trigger('click')

    expect(wrapper.emitted('copy')).toHaveLength(1)
    expect(wrapper.emitted('download')).toHaveLength(1)
  })

  it('renders failed batch rows when the platform action is partial', () => {
    const wrapper = mountCard({
      failureCount: 2,
      failedItems: ['alarm-1: permission denied', 'alarm-2: already reset'],
      summary: '1 succeeded / 2 failed',
      type: 'warning'
    })

    expect(wrapper.text()).toContain('1 succeeded / 2 failed')
    expect(wrapper.text()).toContain('alarm-1: permission denied')
    expect(wrapper.text()).toContain('alarm-2: already reset')
    expect(wrapper.text()).not.toContain('custom.alarmPage.batchActionNoFailedRows')
  })
})
