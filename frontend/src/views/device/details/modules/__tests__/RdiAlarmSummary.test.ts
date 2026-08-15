import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceAlarmHistory: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deviceAlarmHistory: hoisted.deviceAlarmHistory
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import RdiAlarmSummary from '../RdiAlarmSummary.vue'

const mountComponent = () =>
  shallowMount(RdiAlarmSummary, {
    props: { deviceId: 'device-1' },
    global: {
      stubs: {
        NAlert: true,
        NButton: true,
        NCard: true,
        NEmpty: true,
        NFlex: true,
        NSpin: true,
        NTag: true
      }
    }
  })

const getSetupState = (wrapper: ReturnType<typeof mountComponent>) => wrapper.vm.$.setupState as Record<string, any>

describe('RdiAlarmSummary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const alarms = {
      recent: { id: 'recent-cleared', alarm_status: 'N', create_at: '2026-07-18T10:00:00Z' },
      ACTIVE: { id: 'active-medium', alarm_status: 'M', create_at: '2026-07-18T09:00:00Z' }
    }
    hoisted.deviceAlarmHistory.mockImplementation(async (params: { alarm_status?: 'ACTIVE' }) => ({
      data: {
        list: [params.alarm_status ? alarms[params.alarm_status] : alarms.recent],
        total: 1
      }
    }))
  })

  it('loads the latest history row and the current ACTIVE stream separately', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const setupState = getSetupState(wrapper)

    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(2)
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledWith({ device_id: 'device-1', page: 1, page_size: 1 })
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledWith({
      device_id: 'device-1',
      page: 1,
      page_size: 1,
      alarm_status: 'ACTIVE'
    })
    expect(setupState.currentAlarm.id).toBe('active-medium')
    expect(setupState.recentAlarm.id).toBe('recent-cleared')
    expect(setupState.recentAlarm.alarm_status).toBe('N')
    const cardTitles = wrapper.findAll('n-card-stub').map(card => card.attributes('title'))
    expect(cardTitles).toContain('rdi.overview.activeAlarms')
    expect(cardTitles).toContain('rdi.overview.mostRecentAlert')
  })

  it('keeps the summary empty when there is no current or historical alarm', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })
    const wrapper = mountComponent()
    await flushPromises()
    const setupState = getSetupState(wrapper)

    expect(setupState.currentAlarm).toBeNull()
    expect(setupState.recentAlarm).toBeNull()
    expect(setupState.loadFailed).toBe(false)
  })

  it('contains API failures without breaking the device alarm page', async () => {
    hoisted.deviceAlarmHistory.mockRejectedValue(new Error('offline'))
    const wrapper = mountComponent()
    await flushPromises()
    const setupState = getSetupState(wrapper)

    expect(setupState.currentAlarm).toBeNull()
    expect(setupState.recentAlarm).toBeNull()
    expect(setupState.loadFailed).toBe(true)
    expect(setupState.loading).toBe(false)
  })
})
