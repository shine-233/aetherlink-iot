/**
 * 文件用途: 覆盖测试在自动化场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  sceneAutomationsGet: vi.fn(),
  sceneAutomationsSwitch: vi.fn(),
  sceneAutomationsDel: vi.fn(),
  sceneAutomationsLog: vi.fn(),
  deviceAlarmList: vi.fn(),
  routeQuery: {} as Record<string, unknown>
}))

vi.mock('@/service/api/automation', () => ({
  sceneAutomationsGet: hoisted.sceneAutomationsGet,
  sceneAutomationsSwitch: hoisted.sceneAutomationsSwitch,
  sceneAutomationsDel: hoisted.sceneAutomationsDel,
  sceneAutomationsLog: hoisted.sceneAutomationsLog
}))

vi.mock('@/service/api', () => ({
  deviceAlarmList: hoisted.deviceAlarmList
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual('vue-router')
  return {
    ...actual,
    useRoute: () => ({ query: hoisted.routeQuery }),
    useRouter: () => ({ push: vi.fn(), back: vi.fn() })
  }
})

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: vi.fn() })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual('naive-ui')
  return {
    ...actual,
    useDialog: () => ({ warning: vi.fn() }),
    useMessage: () => ({ success: vi.fn(), error: vi.fn() })
  }
})

vi.mock('dayjs', () => {
  const fn = (v?: any) => ({
    subtract: () => ({ valueOf: () => 1000 }),
    format: () => '2024-01-01T00:00:00',
    valueOf: () => v || 1000
  })
  return { default: fn }
})

vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return {
    ...actual,
    getCurrentInstance: () => ({ proxy: { getPlatform: () => false } })
  }
})

import SceneLinkage from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(SceneLinkage, {
    props,
    global: {
      stubs: {
        DataList: defineComponent({
          name: 'DataList',
          props: {
            deviceId: { type: String, default: '' },
            deviceConfigId: { type: String, default: '' },
            backType: { type: String, default: '' },
            onboarding: { type: String, default: '' },
            starter: { type: String, default: '' },
            firstDeviceName: { type: String, default: '' },
            firstDeviceNumber: { type: String, default: '' },
            telemetryKey: { type: String, default: '' },
            telemetryValue: { type: String, default: '' },
            telemetryAt: { type: String, default: '' }
          },
          setup(_, { slots }) {
            return () => h('div', { 'data-testid': 'data-list' }, slots.default?.())
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('SceneLinkage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.routeQuery = {}
    hoisted.sceneAutomationsGet.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.deviceAlarmList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
  })

  afterEach(() => {
    mountedWrappers.forEach((w) => w.unmount())
    mountedWrappers.length = 0
  })

  it('should expose the scene automation list as the only business child', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const dataLists = wrapper.findAll('[data-testid="data-list"]')
    expect(wrapper.element.tagName).toBe('DIV')
    expect(wrapper.classes()).toContain('w-full')
    expect(dataLists).toHaveLength(1)
    expect(dataLists[0].element.parentElement).toBe(wrapper.element)
  })

  it('normalizes repeated route values and keeps automation as the default return context', async () => {
    hoisted.routeQuery = {
      backType: [],
      device_id: ['device-primary', 'device-ignored'],
      device_config_id: ['config-primary', 'config-ignored'],
      first_device_number: ['pump-007', 'pump-ignored'],
      telemetry_at: ['2026-07-30T03:00:00Z', '2026-07-29T03:00:00Z']
    }

    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.getComponent({ name: 'DataList' }).props()).toMatchObject({
      deviceId: 'device-primary',
      deviceConfigId: 'config-primary',
      backType: 'automation',
      firstDeviceNumber: 'pump-007',
      telemetryAt: '2026-07-30T03:00:00Z'
    })
  })

  it('should pass first-device onboarding query into the scene linkage list', async () => {
    hoisted.routeQuery = {
      backType: 'automation',
      onboarding: 'first-device',
      starter: 'first-telemetry-rule',
      device_id: 'dev1',
      device_config_id: 'cfg1',
      first_device_name: 'Pump Controller',
      telemetry_key: 'temperature',
      telemetry_value: '23'
    }

    const wrapper = mountComponent()
    await flushPromises()

    const list = wrapper.findComponent({ name: 'DataList' })
    expect(list.props()).toMatchObject({
      deviceId: 'dev1',
      deviceConfigId: 'cfg1',
      backType: 'automation',
      onboarding: 'first-device',
      starter: 'first-telemetry-rule',
      firstDeviceName: 'Pump Controller',
      telemetryKey: 'temperature',
      telemetryValue: '23'
    })
  })
})
