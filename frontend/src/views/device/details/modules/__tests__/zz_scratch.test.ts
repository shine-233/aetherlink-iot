import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getDeviceHealthDetail: vi.fn(),
  evaluateDeviceHealth: vi.fn()
}))

vi.mock('@/service/api/device-health', () => ({
  getDeviceHealthDetail: hoisted.getDeviceHealthDetail,
  evaluateDeviceHealth: hoisted.evaluateDeviceHealth
}))

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return { ...actual, useMessage: () => ({ success: vi.fn(), error: vi.fn() }) }
})

import Component from '../health-assessment/index.vue'

describe('scratch', () => {
  it('prints html', async () => {
    hoisted.getDeviceHealthDetail.mockResolvedValue({
      data: {
        device_id: 'd1', device_name: 'n1', device_number: 's1', score: 76,
        health_status: 'SUB_HEALTHY', alarm_penalty: 0, offline_penalty: 0, anomaly_penalty: 24,
        is_online: true, offline_duration_seconds: 0, active_alarm_count: 0, active_alarms: [],
        suggestions: ['s1'], evaluated_at: '2026-09-26T00:00:00Z',
        mset: { applied: false, degraded: true, degrade_reason: 'singular_matrix', deviation_score: 0, penalty: 0 }
      }
    })
    const wrapper = shallowMount(Component, {
      props: { deviceId: 'd1' },
      global: {
        stubs: {
          NCard: defineComponent({
            name: 'NCard',
            props: ['title', 'size', 'embedded'],
            setup(_, { slots }) {
              return () => h('div', { class: 'card-stub' }, [slots.header?.(), slots.default?.()])
            }
          }),
          NGrid: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }),
          NGridItem: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }),
          NProgress: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }),
          NStatistic: defineComponent({ props: ['label', 'value'], setup: (p) => () => h('div', [String(p.label), String(p.value)]) }),
          NTag: defineComponent({ setup: (_, { slots }) => () => h('span', slots.default?.()) }),
          NAlert: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }),
          NSpace: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }),
          NButton: defineComponent({ setup: (_, { slots }) => () => h('button', slots.default?.()) }),
          NSpin: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) })
        }
      }
    })
    await flushPromises()
    console.log(wrapper.html())
    expect(true).toBe(true)
  })
})
