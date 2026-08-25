/**
 * 文件用途：覆盖 index 在 系统与账号设置 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, inject, provide, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const paneSpecs = [
  { name: 'data-clear', label: 'page.manage.setting.dataClearSetting.title', section: 'data-clear' },
  {
    name: 'telemetry-dead-letter',
    label: 'custom.management.telemetryDeadLetter.title',
    section: 'telemetry-dead-letter'
  },
  { name: 'operation-log', label: 'custom.management.operationLog.title', section: 'operation-log' },
  { name: 'account-profile', label: 'custom.management.accountProfile.title', section: 'account-profile' },
  { name: 'account-email', label: 'custom.personalCenter.changeAccountEmail', section: 'account-email' },
  { name: 'warning-email', label: 'custom.management.warningEmail', section: 'warning-email' },
  { name: 'branding', label: 'custom.management.branding', section: 'branding' },
  { name: 'function', label: 'custom.management.configSetting', section: 'function' }
] as const

function createSettingStub(stubName: string, section: string) {
  return defineComponent({
    name: stubName,
    setup() {
      return () => h('div', { class: `${section}-setting-stub`, 'data-setting-section': section })
    }
  })
}

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../components/function-setting.vue', () => ({
  default: createSettingStub('FunctionSettingStub', 'function')
}))

vi.mock('../components/data-clear-setting.vue', () => ({
  default: createSettingStub('DataClearSettingStub', 'data-clear')
}))

vi.mock('../components/account-email-setting.vue', () => ({
  default: createSettingStub('AccountEmailSettingStub', 'account-email')
}))

vi.mock('../components/account-profile-setting.vue', () => ({
  default: createSettingStub('AccountProfileSettingStub', 'account-profile')
}))

vi.mock('../components/branding-setting.vue', () => ({
  default: createSettingStub('BrandingSettingStub', 'branding')
}))

vi.mock('../components/warning-email-setting.vue', () => ({
  default: createSettingStub('WarningEmailSettingStub', 'warning-email')
}))

vi.mock('../components/telemetry-dead-letter-setting.vue', () => ({
  default: createSettingStub('TelemetryDeadLetterSettingStub', 'telemetry-dead-letter')
}))

vi.mock('@/views/management/operation-log/index.vue', () => ({
  default: createSettingStub('OperationLogPanelStub', 'operation-log')
}))

import SettingIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []
const tabsActiveKey = Symbol('tabs-active-key')

const NTabsStub = defineComponent({
  name: 'NTabsStub',
  setup(_props, { slots }) {
    const activeTab = ref<(typeof paneSpecs)[number]['name']>(paneSpecs[0].name)
    provide(tabsActiveKey, activeTab)

    return () =>
      h('div', { class: 'n-tabs-stub' }, [
        h(
          'div',
          { class: 'n-tabs-stub__triggers' },
          paneSpecs.map(spec =>
            h(
              'button',
              {
                type: 'button',
                'data-tab-trigger': spec.name,
                'data-active': String(activeTab.value === spec.name),
                onClick: () => {
                  activeTab.value = spec.name
                }
              },
              spec.label
            )
          )
        ),
        h('div', { class: 'n-tabs-stub__content' }, slots.default ? slots.default() : [])
      ])
  }
})

const NTabPaneStub = defineComponent({
  name: 'NTabPaneStub',
  props: {
    name: {
      type: String,
      required: true
    },
    tab: {
      type: String,
      required: true
    }
  },
  setup(props, { slots }) {
    const activeTab = inject<ReturnType<typeof ref<string>>>(tabsActiveKey)

    return () =>
      h(
        'section',
        {
          class: 'n-tab-pane-stub',
          'data-pane-name': props.name,
          'data-pane-tab': props.tab,
          'data-visible': String(activeTab?.value === props.name)
        },
        activeTab?.value === props.name && slots.default ? slots.default() : []
      )
  }
})

const mountComponent = () => {
  const wrapper = mount(SettingIndex, {
    global: {
      renderStubDefaultSlot: true,
      stubs: {
        NCard: true,
        NTabs: NTabsStub,
        NTabPane: NTabPaneStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('management/setting/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders the setting page shell with the default data-clear pane active', () => {
    const wrapper = mountComponent()

    expect(wrapper.classes()).toContain('overflow-hidden')
    expect(wrapper.findAll('.n-tab-pane-stub').map(pane => pane.attributes('data-visible'))).toEqual([
      'true',
      'false',
      'false',
      'false',
      'false',
      'false',
      'false',
      'false'
    ])
    expect(wrapper.get('[data-visible="true"] [data-setting-section]').attributes('data-setting-section')).toBe(
      'data-clear'
    )
  })

  it('renders all setting panes in the expected order', () => {
    const wrapper = mountComponent()
    const paneNames = wrapper.findAll('.n-tab-pane-stub').map(pane => pane.attributes('data-pane-name'))
    expect(paneNames).toEqual(paneSpecs.map(spec => spec.name))
  })

  it('renders translated labels for each setting pane', () => {
    const wrapper = mountComponent()
    const labels = wrapper.findAll('.n-tab-pane-stub').map(pane => pane.attributes('data-pane-tab'))
    expect(labels).toEqual(paneSpecs.map(spec => spec.label))
  })

  it('renders each setting section inside its matching pane', () => {
    const wrapper = mountComponent()
    expect(wrapper.get('[data-visible="true"] [data-setting-section]').attributes('data-setting-section')).toBe('data-clear')
  })

  it('switches visible content when a different tab is selected', async () => {
    const wrapper = mountComponent()

    const accountProfileTrigger = wrapper.get('[data-tab-trigger="account-profile"]')
    await accountProfileTrigger.trigger('click')
    expect(wrapper.get('[data-visible="true"] [data-setting-section]').attributes('data-setting-section')).toBe(
      'account-profile'
    )

    const brandingTrigger = wrapper.get('[data-tab-trigger="branding"]')
    await brandingTrigger.trigger('click')
    expect(wrapper.get('[data-visible="true"] [data-setting-section]').attributes('data-setting-section')).toBe('branding')

    const deadLetterTrigger = wrapper.get('[data-tab-trigger="telemetry-dead-letter"]')
    await deadLetterTrigger.trigger('click')
    expect(wrapper.get('[data-visible="true"] [data-setting-section]').attributes('data-setting-section')).toBe(
      'telemetry-dead-letter'
    )

    const functionTrigger = wrapper.get('[data-tab-trigger="function"]')
    await functionTrigger.trigger('click')
    expect(wrapper.get('[data-visible="true"] [data-setting-section]').attributes('data-setting-section')).toBe('function')
  })
})
