/**
 * 文件用途：覆盖 index 在 RDI 告警概览 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

// Mock the dashboard RdiOverview as a stub component so we can verify it's used
vi.mock('@/views/dashboard/rdi-overview/index.vue', () => ({
  default: defineComponent({
    name: 'RdiOverview',
    props: {
      activeSystemsOnly: {
        type: Boolean,
        default: false
      }
    },
    setup() {
      return () => h('div', { class: 'rdi-overview-stub' }, 'RdiOverview Content')
    }
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import AlarmRdiOverview from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

// Mount without stubs so the mock RdiOverview lifecycle hooks fire
const mountComponent = () => {
  const wrapper = mount(AlarmRdiOverview)
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('alarm/rdi-overview/index.vue', () => {
  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('delegates the alarm route to one active-system dashboard overview', () => {
    const wrapper = mountComponent()

    const rdiComponents = wrapper.findAllComponents({ name: 'RdiOverview' })
    expect(rdiComponents).toHaveLength(1)
    expect(rdiComponents[0].props('activeSystemsOnly')).toBe(true)
    expect(rdiComponents[0].text()).toBe('RdiOverview Content')
    expect(wrapper.text()).toBe('RdiOverview Content')
    expect(Object.keys(wrapper.vm.$props)).toEqual([])
    expect(wrapper.emitted()).toEqual({})
  })
})
