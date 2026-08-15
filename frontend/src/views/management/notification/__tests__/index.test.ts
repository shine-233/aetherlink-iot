/**
 * 文件用途：覆盖 index 在 通知服务管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('~/src/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../components/email.vue', () => ({
  default: defineComponent({
    name: 'Email',
    setup() {
      return () => h('div', { 'data-test': 'email-stub' })
    }
  })
}))

vi.mock('../components/short-message.vue', () => ({
  default: defineComponent({
    name: 'ShortMessage',
    setup() {
      return () => h('div', { 'data-test': 'short-message-stub' })
    }
  })
}))

vi.mock('../components/push-notification.vue', () => ({
  default: defineComponent({
    name: 'PushNotification',
    setup() {
      return () => h('div', { 'data-test': 'push-notification-stub' })
    }
  })
}))

const NCardStub = defineComponent({
  name: 'NCard',
  props: {
    bordered: {
      type: Boolean,
      default: true
    }
  },
  setup(props, { attrs, slots }) {
    return () =>
      h(
        'section',
        {
          'data-test': 'n-card',
          'data-bordered': String(props.bordered),
          class: attrs.class
        },
        slots.default?.()
      )
  }
})

const NTabsStub = defineComponent({
  name: 'NTabs',
  props: {
    type: {
      type: String,
      default: ''
    },
    animated: {
      type: Boolean,
      default: false
    }
  },
  setup(props, { slots }) {
    return () =>
      h(
        'div',
        {
          'data-test': 'n-tabs',
          'data-type': props.type,
          'data-animated': String(props.animated)
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
    tab: {
      type: String,
      default: ''
    }
  },
  setup(props, { attrs, slots }) {
    return () =>
      h(
        'article',
        {
          'data-test': 'n-tab-pane',
          'data-name': props.name,
          'data-tab': props.tab,
          class: attrs.class
        },
        slots.default?.()
      )
  }
})

import NotificationIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountComponent = () => {
  const wrapper = mount(NotificationIndex, {
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

describe('management/notification/index.vue', () => {
  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('mounts with the expected card shell classes', () => {
    const wrapper = mountComponent()
    const card = wrapper.get('[data-test="n-card"]')

    expect(wrapper.classes()).toContain('overflow-hidden')
    expect(card.attributes('data-bordered')).toBe('false')
    expect(card.classes()).toEqual(expect.arrayContaining(['h-full', 'rounded-8px', 'shadow-sm']))
  })

  it('configures line animated tabs with the expected translated panes', () => {
    const wrapper = mountComponent()
    const tabs = wrapper.get('[data-test="n-tabs"]')
    const panes = wrapper.findAll('[data-test="n-tab-pane"]')

    expect(tabs.attributes('data-type')).toBe('line')
    expect(tabs.attributes('data-animated')).toBe('true')
    expect(panes).toHaveLength(3)
    expect(panes.map(pane => pane.attributes('data-name'))).toEqual(['1', '2', '3'])
    expect(panes.map(pane => pane.attributes('data-tab'))).toEqual([
      'page.manage.notification.email.title',
      'page.manage.notification.shortMessage.title',
      'page.manage.notification.pushNotification.title'
    ])
    expect(panes.every(pane => pane.classes().includes('pannel-content'))).toBe(true)
  })

  it('wires email, short message, and push notification components into separate panes', () => {
    const wrapper = mountComponent()
    const panes = wrapper.findAll('[data-test="n-tab-pane"]')

    expect(panes.map(pane => pane.findAll('[data-test$="-stub"]').map(stub => stub.attributes('data-test')))).toEqual([
      ['email-stub'],
      ['short-message-stub'],
      ['push-notification-stub']
    ])
  })
})
