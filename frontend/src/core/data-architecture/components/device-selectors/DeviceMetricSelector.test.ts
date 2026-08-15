/**
 * 文件用途: Device Metric Selector 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DeviceMetricSelector from './DeviceMetricSelector.vue'
import { getDeviceMetricList, getDeviceSourceList } from '@/service/api'

vi.mock('@/service/api', () => ({
  getDeviceMetricList: vi.fn(),
  getDeviceSourceList: vi.fn()
}))

const passthroughStub = {
  template: '<div><slot /></div>'
}

const selectStub = {
  name: 'NSelect',
  props: ['value', 'options', 'loading', 'disabled'],
  emits: ['update:value'],
  template: `
    <select data-test="n-select" :value="value" :disabled="disabled" @change="$emit('update:value', $event.target.value)">
      <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
    </select>
  `
}

const buttonStub = {
  name: 'NButton',
  props: ['disabled', 'type'],
  emits: ['click'],
  template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>'
}

const mountSelector = (props: Record<string, unknown> = {}) =>
  mount(DeviceMetricSelector, {
    props,
    global: {
      stubs: {
        NSelect: selectStub,
        NSpace: passthroughStub,
        NText: passthroughStub,
        NIcon: passthroughStub,
        NButton: buttonStub,
        NAlert: passthroughStub,
        NDivider: passthroughStub,
        DeviceIcon: passthroughStub,
        MetricIcon: passthroughStub
      }
    }
  })

describe('DeviceMetricSelector real device metrics source', () => {
  beforeEach(() => {
    vi.mocked(getDeviceSourceList).mockReset()
    vi.mocked(getDeviceMetricList).mockReset()
  })

  it('keeps metric options empty and does not emit mock metrics when the metrics API is empty', async () => {
    vi.mocked(getDeviceSourceList).mockResolvedValue({
      data: [{ id: 'real-device-1', name: 'Real Device 1', device_type: 'meter' }]
    })
    vi.mocked(getDeviceMetricList).mockResolvedValue({ data: [] })

    const wrapper = mountSelector({
      preSelectedDevice: {
        deviceId: 'real-device-1',
        deviceName: 'Real Device 1',
        deviceType: 'meter'
      }
    })
    await flushPromises()

    expect(getDeviceMetricList).toHaveBeenCalledWith('real-device-1')
    expect(wrapper.text()).not.toContain('temperature')
    expect(wrapper.text()).not.toContain('power_001')

    await wrapper.findAll('button').at(-1)!.trigger('click')

    expect(wrapper.emitted('selectionCompleted')).toBeUndefined()
  })
})
