/**
 * 文件用途: health-assessment 组件 TP-21 MSET 特征维度的三态渲染测试。
 * 核心逻辑: 钉死三条口径——①后端开关默认关（响应无 mset 字段）时不渲染 MSET 区块，旧 UI 形状不变；
 * ②有效推理态展示偏差分/马氏距离/训练样本/特征键与折入异常扣分的扣分值；
 * ③降级态（singular_matrix 等）如实渲染降级原因与"中性不扣分"说明，绝不伪装成正常推理。
 * 关键注意事项: mock 响应字段须与后端 model.DeviceHealthMSETFeature 的 JSON 形状逐字段一致；
 * 组件显式 import naive-ui，stub 键必须用其内部名（Card/Tag/...，非 NCard），否则命中自动 stub、
 * 命名插槽（#header）不渲染导致断言失真；naive-ui 仅覆写 useMessage（无 provider 上下文）。
 * 重构建议: 后端若新增降级原因词表项，同步补 getMSETDegradeReasonText 映射与本文件断言。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

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
  return {
    ...actual,
    useMessage: () => ({
      success: vi.fn(),
      error: vi.fn()
    })
  }
})

import Component from '../health-assessment/index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

// naive-ui 组件在组件内显式 import，stub 匹配走其内部名（Card/Tag/...）；
// 键须渲染命名插槽（#header/#header-extra）与默认插槽，否则文本断言失真。
const slotStub = defineComponent({
  name: 'SlotStub',
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const cardStub = defineComponent({
  name: 'Card',
  props: ['title', 'size', 'embedded'],
  setup(props, { slots }) {
    // title 是 prop（如「告警扣分」维度卡），须渲染出来供文本断言；header/default 为插槽内容。
    return () => h('div', [String(props.title ?? ''), slots.header?.(), slots['header-extra']?.(), slots.default?.()])
  }
})

const statisticStub = defineComponent({
  name: 'Statistic',
  props: ['label', 'value'],
  setup(props) {
    return () => h('div', [String(props.label ?? ''), String(props.value ?? '')])
  }
})

const mountComponent = () => {
  const wrapper = shallowMount(Component, {
    props: { deviceId: 'device-mset-1' },
    global: {
      stubs: {
        Card: cardStub,
        NCard: cardStub,
        Grid: slotStub,
        GridItem: slotStub,
        Progress: slotStub,
        Statistic: statisticStub,
        Tag: slotStub,
        Alert: slotStub,
        Space: slotStub,
        Button: slotStub,
        Spin: slotStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const baseDetail = {
  device_id: 'device-mset-1',
  device_name: 'MSET 演示设备',
  device_number: 'SN-MSET-001',
  score: 76,
  health_status: 'SUB_HEALTHY',
  alarm_penalty: 0,
  offline_penalty: 0,
  anomaly_penalty: 24,
  is_online: true,
  offline_duration_seconds: 0,
  active_alarm_count: 0,
  active_alarms: [],
  suggestions: ['设备运行工况良好，各项指标均在稳定基线以内'],
  evaluated_at: '2026-09-26T00:00:00Z'
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  while (mountedWrappers.length) {
    mountedWrappers.pop()?.unmount()
  }
})

describe('health-assessment MSET dimension', () => {
  it('hides the MSET block entirely when the backend switch is off (no mset field)', async () => {
    hoisted.getDeviceHealthDetail.mockResolvedValue({ data: { ...baseDetail } })
    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.text()).not.toContain('MSET 多元状态估计')
    expect(wrapper.text()).not.toContain('已降级（未参与评分）')
    // 旧 UI 形状不变：综合得分与三维扣分仍然可见
    expect(wrapper.text()).toContain('综合得分')
    expect(wrapper.text()).toContain('告警扣分')
  })

  it('renders applied inference with deviation score, distance, samples and feature keys', async () => {
    hoisted.getDeviceHealthDetail.mockResolvedValue({
      data: {
        ...baseDetail,
        suggestions: ['MSET 多元状态估计检测到运行特征显著偏离历史基线（偏差分 85.50），建议核查多指标联合趋势'],
        mset: {
          applied: true,
          degraded: false,
          deviation_score: 85.5,
          penalty: 25.2,
          mahalanobis: 3.42,
          feature_keys: ['temperature', 'humidity'],
          train_samples: 143
        }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('MSET 多元状态估计')
    expect(text).toContain('Multivariate State Estimation')
    expect(text).toContain('有效推理')
    expect(text).toContain('偏差分 85.5')
    expect(text).toContain('马氏距离3.42')
    expect(text).toContain('训练样本数143')
    expect(text).toContain('temperature')
    expect(text).toContain('humidity')
    expect(text).toContain('-25.2 分')
    // 降级警示与降级标签不得出现在有效推理态
    expect(text).not.toContain('已降级（未参与评分）')
    expect(text).not.toContain('未包含多元偏差扣分')
  })

  it('renders degraded state with the reason and neutral-score note', async () => {
    hoisted.getDeviceHealthDetail.mockResolvedValue({
      data: {
        ...baseDetail,
        // 降级场景的 suggestions 也应是降级口径（后端在降级时会给出"未生效"建议）
        suggestions: ['MSET 多元状态估计未生效（singular_matrix），本次评分未包含多元偏差扣分'],
        mset: {
          applied: false,
          degraded: true,
          degrade_reason: 'singular_matrix',
          deviation_score: 0,
          penalty: 0
        }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('MSET 多元状态估计')
    expect(text).toContain('已降级（未参与评分）')
    expect(text).toContain('特征协方差奇异：特征共线或零方差，无法求逆')
    expect(text).toContain('本次评分未包含多元偏差扣分（中性处理）')
    // 降级态不得渲染推理指标
    expect(text).not.toContain('马氏距离')
    expect(text).not.toContain('训练样本数')
  })
})
