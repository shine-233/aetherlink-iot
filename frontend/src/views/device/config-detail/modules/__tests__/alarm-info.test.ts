/**
 * 文件用途: alarm-info 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  routerPushByKey: vi.fn()
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

vi.mock('@/views/automation/scene-linkage/modules/dataList.vue', () => ({
  default: defineComponent({
    name: 'alarmDataList',
    props: ['isAlarm', 'deviceConfigId', 'backType'],
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  })
}))

import Component from '../alarm-info.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { configId: 'cfg-1', ...props },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/alarm-info.vue', () => {
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() } })

  it('renders config-scoped alarm rule list entry and add action', () => {
    const wrapper = mountComponent()
    const alarmList = wrapper.getComponent({ name: 'alarmDataList' })
    expect(alarmList.props()).toMatchObject({
      isAlarm: true,
      deviceConfigId: 'cfg-1',
      backType: 'config'
    })
    expect(wrapper.text()).toContain('generate.addAlarmRule')
  })

  it('alarmAdd navigates to automation linkage edit', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.alarmAdd()
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('automation_linkage-edit', {
      query: { device_config_id: 'cfg-1', backType: 'config' }
    })
  })
})
