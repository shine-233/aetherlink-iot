import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  userInfo: { userName: 'TestUser' },
  sumData: vi.fn(),
  getAlarmCount: vi.fn(),
  listFleetCommandJobs: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string, params?: Record<string, string | number>) => {
    if (!params) return key

    const serialized = Object.entries(params)
      .map(([name, value]) => `${name}=${value}`)
      .join(',')

    return `${key}:${serialized}`
  }
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: hoisted.userInfo
  })
}))

vi.mock('@/service/api/system-data', () => ({
  sumData: hoisted.sumData,
  getAlarmCount: hoisted.getAlarmCount
}))

vi.mock('@/service/api/device', () => ({
  listFleetCommandJobs: hoisted.listFleetCommandJobs
}))

import WorkbenchHeader from '../index.vue'

describe('workbench-header/index.vue', () => {
  const mountedWrappers: Array<ReturnType<typeof mount>> = []

  const mountComponent = () => {
    const wrapper = mount(WorkbenchHeader)
    mountedWrappers.push(wrapper)
    return wrapper
  }

  afterEach(() => {
    vi.clearAllMocks()

    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders the AetherLink IoT workbench greeting with the current user', () => {
    hoisted.sumData.mockResolvedValue({ data: { device_total: 16, device_on: 9, device_offline: 7 } })
    hoisted.getAlarmCount.mockResolvedValue({ data: { active_alarm_total: 4, alarm_device_total: 2 } })
    hoisted.listFleetCommandJobs.mockResolvedValue({
      total: 6,
      attention_counts: { needs_operator_action_count: 3 }
    })

    const wrapper = mountComponent()

    expect(wrapper.vm.$options.name).toBe('DashboardWorkbenchHeader')
    expect(wrapper.text()).toContain('custom.dashboardWorkbench.title:userName=TestUser')
    expect(wrapper.text()).toContain('custom.dashboardWorkbench.description')
    expect(wrapper.findAll('iconlocalavatar')).toHaveLength(1)
  })

  it('renders real IoT summary metrics instead of fake readiness statistics', async () => {
    hoisted.sumData.mockResolvedValue({ data: { device_total: 16, device_on: 9, device_offline: 7 } })
    hoisted.getAlarmCount.mockResolvedValue({ data: { active_alarm_total: 4, alarm_device_total: 2 } })
    hoisted.listFleetCommandJobs.mockResolvedValue({
      total: 6,
      attention_counts: { needs_operator_action_count: 3 }
    })

    const wrapper = mountComponent()
    await flushPromises()
    const statistics = wrapper.findAll('.workbench-header-summary-item')

    expect(statistics).toHaveLength(3)
    expect(statistics[0].text()).toContain('custom.dashboardWorkbench.deviceSummary')
    expect(statistics[0].text()).toContain('16')
    expect(statistics[0].text()).toContain('custom.dashboardWorkbench.deviceSummaryDetail:online=9,offline=7')
    expect(statistics[1].text()).toContain('custom.dashboardWorkbench.activeAlarmSummary')
    expect(statistics[1].text()).toContain('4')
    expect(statistics[1].text()).toContain('custom.dashboardWorkbench.activeAlarmSummaryDetail:devices=2')
    expect(statistics[2].text()).toContain('custom.dashboardWorkbench.commandJobSummary')
    expect(statistics[2].text()).toContain('6')
    expect(statistics[2].text()).toContain('custom.dashboardWorkbench.commandJobSummaryDetail:attention=3')
    expect(hoisted.sumData).toHaveBeenCalledTimes(1)
    expect(hoisted.getAlarmCount).toHaveBeenCalledTimes(1)
    expect(hoisted.listFleetCommandJobs).toHaveBeenCalledWith({ page: 1, page_size: 1 })
  })

  it('falls back to an honest unavailable hint when runtime metrics cannot be read', async () => {
    hoisted.sumData.mockRejectedValue(new Error('device summary unavailable'))
    hoisted.getAlarmCount.mockResolvedValue({ data: { active_alarm_total: 4, alarm_device_total: 2 } })
    hoisted.listFleetCommandJobs.mockResolvedValue({
      total: 6,
      attention_counts: {}
    })

    const wrapper = mountComponent()
    await flushPromises()

    const statistics = wrapper.findAll('.workbench-header-summary-item')
    expect(statistics[0].text()).toContain('custom.dashboardWorkbench.metricUnavailable')
    expect(statistics[1].text()).toContain('custom.dashboardWorkbench.activeAlarmSummaryDetail:devices=2')
    expect(statistics[2].text()).toContain('custom.dashboardWorkbench.metricUnavailable')
  })
})
