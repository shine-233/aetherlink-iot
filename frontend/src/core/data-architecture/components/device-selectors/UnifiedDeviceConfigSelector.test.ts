/**
 * 文件用途: Unified Device Config Selector 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import UnifiedDeviceConfigSelector from './UnifiedDeviceConfigSelector.vue'
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

const checkboxStub = {
  name: 'NCheckbox',
  props: ['checked', 'disabled'],
  emits: ['update:checked'],
  template:
    '<input type="checkbox" :checked="checked" :disabled="disabled" @change="$emit(\'update:checked\', $event.target.checked)" />'
}

const mountSelector = (props: Record<string, unknown> = {}) =>
  mount(UnifiedDeviceConfigSelector, {
    props,
    global: {
      stubs: {
        NCard: passthroughStub,
        NSpace: passthroughStub,
        NText: passthroughStub,
        NIcon: passthroughStub,
        NButton: buttonStub,
        NSelect: selectStub,
        NCheckbox: checkboxStub,
        NAlert: passthroughStub,
        NDivider: passthroughStub,
        NTag: passthroughStub,
        DeviceIcon: passthroughStub,
        MetricIcon: passthroughStub,
        LocationIcon: passthroughStub,
        StatusIcon: passthroughStub
      }
    }
  })

describe('UnifiedDeviceConfigSelector real device source', () => {
  beforeEach(() => {
    vi.mocked(getDeviceSourceList).mockReset()
    vi.mocked(getDeviceMetricList).mockReset()
  })

  it('does not generate mock parameters when the device API returns no devices', async () => {
    vi.mocked(getDeviceSourceList).mockResolvedValue({ data: [] })
    vi.mocked(getDeviceMetricList).mockResolvedValue({ data: [] })

    const wrapper = mountSelector()
    await flushPromises()

    expect(wrapper.text()).not.toContain('sensor_001')
    expect(wrapper.text()).not.toContain('temperature')

    await wrapper.findAll('button').at(-1)!.trigger('click')

    expect(wrapper.emitted('parametersGenerated')).toBeUndefined()
  })

  it('preserves existing real device and metric values when API lookup fails', async () => {
    vi.mocked(getDeviceSourceList).mockRejectedValue(new Error('device API unavailable'))
    vi.mocked(getDeviceMetricList).mockRejectedValue(new Error('metrics API unavailable'))

    const wrapper = mountSelector({
      editMode: true,
      existingParameters: [
        { key: 'deviceId', value: 'real-device-404', enabled: true, valueMode: 'manual', dataType: 'string' },
        { key: 'metric', value: 'real_metric_404', enabled: true, valueMode: 'manual', dataType: 'string' }
      ]
    })
    await flushPromises()

    expect(wrapper.text()).toContain('real-device-404')
    expect(wrapper.text()).toContain('real_metric_404')
    expect(wrapper.text()).not.toContain('sensor_001')
    expect(wrapper.text()).not.toContain('temperature')

    await wrapper.findAll('button').at(-1)!.trigger('click')

    const emitted = wrapper.emitted('parametersGenerated')?.at(-1)?.[0] as any[]
    expect(emitted).toEqual([
      expect.objectContaining({ key: 'deviceId', value: 'real-device-404' }),
      expect.objectContaining({ key: 'metric', value: 'real_metric_404' })
    ])
  })
})
