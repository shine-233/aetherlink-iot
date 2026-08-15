/**
 * 文件用途：覆盖 warning-message 在 告警消息管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {}
  })
}))

vi.mock('../components/alarm-configuration.vue', () => ({
  default: defineComponent({
    name: 'AlarmConfiguration',
    setup() {
      return () => h('div', { 'data-test': 'alarm-configuration-stub' })
    }
  })
}))

vi.mock('../components/new-information.vue', () => ({
  default: defineComponent({
    name: 'NewInformation',
    setup() {
      return () => h('div', { 'data-test': 'new-information-stub' })
    }
  })
}))

const NCardStub = defineComponent({
  name: 'NCard',
  props: {
    title: {
      type: String,
      default: ''
    }
  },
  setup(props, { slots }) {
    return () => h('section', { 'data-test': 'n-card', 'data-title': props.title }, slots.default?.())
  }
})

const NTabsStub = defineComponent({
  name: 'NTabs',
  props: {
    type: {
      type: String,
      default: ''
    },
    size: {
      type: String,
      default: ''
    }
  },
  setup(props, { slots }) {
    return () =>
      h(
        'div',
        {
          'data-test': 'n-tabs',
          'data-type': props.type,
          'data-size': props.size
        },
        slots.default?.()
      )
  }
})

const NTabPaneStub = defineComponent({
  name: 'NTabPane',
  props: {
    name: {
      type: String,
      default: ''
    },
    tag: {
      type: String,
      default: ''
    }
  },
  setup(props, { slots }) {
    return () =>
      h(
        'article',
        {
          'data-test': 'n-tab-pane',
          'data-name': props.name,
          'data-tag': props.tag
        },
        slots.default?.()
      )
  }
})

import WarningMessage from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountWarningMessage = () => {
  const wrapper = mount(WarningMessage, {
    global: {
      stubs: {
        NCard: NCardStub,
        'n-card': NCardStub,
        NTabs: NTabsStub,
        'n-tabs': NTabsStub,
        NTabPane: NTabPaneStub,
        'n-tab-pane': NTabPaneStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('warning-message/index.vue', () => {
  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('mounts with the expected root class and alarm center title', () => {
    const wrapper = mountWarningMessage()

    expect(wrapper.classes()).toContain('table-box')
    expect(wrapper.get('[data-test="n-card"]').attributes('data-title')).toBe('generate.alarm-center')
  })

  it('configures line tabs with large size and two translated tab panes', () => {
    const wrapper = mountWarningMessage()

    const tabs = wrapper.get('[data-test="n-tabs"]')
    const panes = wrapper.findAll('[data-test="n-tab-pane"]')

    expect(tabs.attributes('data-type')).toBe('line')
    expect(tabs.attributes('data-size')).toBe('large')
    expect(panes).toHaveLength(2)
    expect(panes.map(pane => pane.attributes('data-name'))).toEqual([
      'generate.alarmInfo',
      'generate.alarmConfig'
    ])
    expect(panes.map(pane => pane.attributes('data-tag'))).toEqual([
      'generate.alarmInfo',
      'generate.alarmConfig'
    ])
  })

  it('wires the alarm child components into the matching tab panes', () => {
    const wrapper = mountWarningMessage()

    const panes = wrapper.findAll('[data-test="n-tab-pane"]')

    expect(panes).toHaveLength(2)
    expect(panes[0].attributes('data-name')).toBe('generate.alarmInfo')
    expect(panes[0].html()).toContain('data-test="alarm-configuration-stub"')
    expect(panes[0].html()).not.toContain('data-test="new-information-stub"')
    expect(panes[1].attributes('data-name')).toBe('generate.alarmConfig')
    expect(panes[1].html()).toContain('data-test="new-information-stub"')
    expect(panes[1].html()).not.toContain('data-test="alarm-configuration-stub"')
  })
})
