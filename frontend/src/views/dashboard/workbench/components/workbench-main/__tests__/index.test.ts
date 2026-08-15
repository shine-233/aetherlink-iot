import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import CapabilityCard from '../components/capability-card.vue'
import ShortcutsCard from '../components/shortcuts-card.vue'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

import WorkbenchMain from '../index.vue'

describe('workbench-main/index.vue', () => {
  const mountedWrappers: Array<ReturnType<typeof mount>> = []
  const WorkbenchCardStub = defineComponent({
    props: ['title'],
    setup(props, { slots }) {
      return () =>
        h('section', { 'data-test': 'workbench-card', 'data-title': props.title }, [
          props.title,
          slots['header-extra']?.(),
          slots.default?.()
        ])
    }
  })

  const mountComponent = () => {
    const wrapper = mount(WorkbenchMain, {
      global: {
        stubs: {
          NCard: WorkbenchCardStub,
          'n-card': WorkbenchCardStub
        }
      }
    })
    mountedWrappers.push(wrapper)
    return wrapper
  }

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders the dashboard workbench shell with four IoT operation sections', () => {
    const wrapper = mountComponent()

    expect(wrapper.vm.$options.name).toBe('DashboardWorkbenchMain')
    expect(wrapper.findAll('[data-test="workbench-card"]')).toHaveLength(4)
    expect(wrapper.text()).toContain('custom.dashboardWorkbench.capabilityTitle')
    expect(wrapper.text()).toContain('custom.dashboardWorkbench.activityTitle')
    expect(wrapper.text()).toContain('custom.dashboardWorkbench.quickOperationTitle')
    expect(wrapper.text()).toContain('custom.dashboardWorkbench.readinessTitle')
  })

  it('renders device, twin, command, OTA, and alarm capability lanes', () => {
    const wrapper = mountComponent()
    const capabilityCards = wrapper.findAllComponents(CapabilityCard)

    expect(capabilityCards).toHaveLength(6)
    expect(capabilityCards.map(card => card.props('name'))).toEqual([
      'custom.dashboardWorkbench.capabilityDeviceOnboarding',
      'custom.dashboardWorkbench.capabilityReadyCheck',
      'custom.dashboardWorkbench.capabilityTwin',
      'custom.dashboardWorkbench.capabilityCommandJobs',
      'custom.dashboardWorkbench.capabilityOta',
      'custom.dashboardWorkbench.capabilityAlarmClosure'
    ])
    expect(capabilityCards.map(card => card.props('route'))).toEqual([
      '/first-device',
      '/device/manage',
      '/device/manage',
      '/device/command-center',
      '/product/update-ota',
      '/alarm/warning-message'
    ])
  })

  it('renders customer operation activity items and evidence descriptions', () => {
    const wrapper = mountComponent()
    const things = wrapper.findAll('nthing')

    expect(things).toHaveLength(5)
    expect(things[0].attributes('title')).toBe('custom.dashboardWorkbench.activityFirstDevice')
    expect(things[0].attributes('description')).toBe('custom.dashboardWorkbench.activityFirstDeviceDesc')
    expect(things[4].attributes('title')).toBe('custom.dashboardWorkbench.activityVisualization')
    expect(things[4].attributes('description')).toBe('custom.dashboardWorkbench.activityVisualizationDesc')
  })

  it('renders quick-operation shortcuts for live product routes', () => {
    const wrapper = mountComponent()
    const shortcutCards = wrapper.findAllComponents(ShortcutsCard)

    expect(shortcutCards).toHaveLength(6)
    expect(shortcutCards.map(card => card.props('label'))).toEqual([
      'custom.dashboardWorkbench.shortcutFirstDevice',
      'custom.dashboardWorkbench.shortcutFleet',
      'custom.dashboardWorkbench.shortcutAutomation',
      'custom.dashboardWorkbench.shortcutDashboard',
      'custom.dashboardWorkbench.shortcutOta',
      'custom.dashboardWorkbench.shortcutAlarm'
    ])
    expect(shortcutCards.map(card => card.props('route'))).toEqual([
      '/first-device',
      '/device/manage',
      '/automation/linkage-edit',
      '/visualization/thingsvis-dashboards',
      '/product/update-ota',
      '/alarm/warning-message'
    ])
  })

  it('renders readiness reminders that state the proof boundary', () => {
    const wrapper = mountComponent()

    expect(wrapper.text()).toContain('custom.dashboardWorkbench.readinessAccess')
    expect(wrapper.text()).toContain('custom.dashboardWorkbench.readinessEvidence')
    expect(wrapper.text()).toContain('custom.dashboardWorkbench.readinessSupport')
  })
})
