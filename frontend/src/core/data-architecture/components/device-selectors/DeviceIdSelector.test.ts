/**
 * 文件用途: Device Id Selector 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DeviceIdSelector from './DeviceIdSelector.vue'
import { getDeviceSourceList } from '@/service/api'

vi.mock('@/service/api', () => ({
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
    <select data-test="n-select" :disabled="disabled" @change="$emit('update:value', $event.target.value)">
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

const mountSelector = () =>
  mount(DeviceIdSelector, {
    global: {
      stubs: {
        NSelect: selectStub,
        NSpace: passthroughStub,
        NText: passthroughStub,
        NIcon: passthroughStub,
        NButton: buttonStub,
        NAlert: passthroughStub,
        DeviceIcon: passthroughStub
      }
    }
  })

describe('DeviceIdSelector real device source', () => {
  beforeEach(() => {
    vi.mocked(getDeviceSourceList).mockReset()
  })

  it('does not expose or emit mock devices when the device API fails', async () => {
    vi.mocked(getDeviceSourceList).mockRejectedValue(new Error('device API unavailable'))

    const wrapper = mountSelector()
    await flushPromises()

    expect(wrapper.text()).not.toContain('sensor_001')
    expect(wrapper.text()).not.toContain('power_001')
    expect(wrapper.findAll('option')).toHaveLength(0)

    await wrapper.findAll('button').at(-1)!.trigger('click')

    expect(wrapper.emitted('deviceSelected')).toBeUndefined()
  })
})
