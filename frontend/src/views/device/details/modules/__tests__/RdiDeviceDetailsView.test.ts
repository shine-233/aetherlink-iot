/**
 * 文件用途：验证 RDI 客户只读详细信息页的配置快照、实时遥测和设备切换合同。
 * 关键注意事项：该页不得出现保存/下发入口；真实设备轮询、WebSocket 和布局不由本测试证明。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  rdiDeviceConfig: vi.fn(),
  loadRealtimeState: vi.fn().mockResolvedValue(undefined),
  startTelemetryRefresh: vi.fn(),
  telemetryRows: [
    { label: 'T1', value: '5.50', unit: 'C' },
    { label: 'T2', value: '6.25', unit: 'C' },
    { label: 'Input Node 1', value: 'High', unit: '' },
    { label: 'Input Node 2', value: 'Low', unit: '' },
    { label: 'Output Node 01', value: 'High', unit: '' },
    { label: 'LED1', value: 'Solid', unit: '' },
    { label: 'kWh', value: '12.40', unit: 'kWh' }
  ],
  appStore: {
    locale: 'en-US' as App.I18n.LangType
  }
}))

vi.mock('@/service/api/rdi', () => ({
  rdiDeviceConfig: hoisted.rdiDeviceConfig
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => hoisted.appStore
}))

vi.mock('../rdi/composables/useRdiTelemetry', async () => {
  const { computed, ref } = await import('vue')
  return {
    useRdiTelemetry: (
      _deviceId: () => string,
      online: () => number | undefined
    ) => ({
      temperatureUnit: ref('C'),
      telemetryRows: computed(() => hoisted.telemetryRows),
      deviceOnlineText: computed(() => (online() === 1 ? 'Online' : 'Offline')),
      formatTemperatureValue: (value: unknown) => {
        const numeric = Number(value)
        return Number.isFinite(numeric) ? numeric.toFixed(2) : '--'
      },
      loadRealtimeState: hoisted.loadRealtimeState,
      startTelemetryRefresh: hoisted.startTelemetryRefresh
    })
  }
})

import RdiDeviceDetailsView from '../RdiDeviceDetailsView.vue'

const CardStub = defineComponent({
  props: {
    title: {
      type: String,
      default: ''
    }
  },
  setup(props, { slots }) {
    return () => h('section', [h('h2', props.title), slots.default?.()])
  }
})

const DescriptionItemStub = defineComponent({
  props: {
    label: {
      type: String,
      default: ''
    }
  },
  setup(props, { slots }) {
    return () => h('div', [h('strong', props.label), slots.default?.()])
  }
})

const SlotStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const TelemetrySummaryStub = defineComponent({
  name: 'RdiTelemetrySummary',
  props: {
    rows: {
      type: Array,
      default: () => []
    },
    temperatureUnit: {
      type: String,
      default: 'C'
    },
    temperatureUnitOptions: {
      type: Array,
      default: () => []
    },
    t: {
      type: Function,
      default: (key: string) => key
    }
  },
  emits: ['update:temperatureUnit'],
  setup(props) {
    return () =>
      h(
        'div',
        { class: 'rdi-telemetry-summary-stub' },
        (props.rows as Array<{ label: string; value: unknown; unit?: string }>).map(row =>
          h('div', `${row.label}: ${row.value}${row.unit ? ` ${row.unit}` : ''}`)
        )
      )
  }
})

function mountDetails(id = 'device-1') {
  return shallowMount(RdiDeviceDetailsView, {
    props: {
      id,
      deviceData: {
        id,
        name: 'Detail fallback name',
        description: 'Cold room controller'
      },
      online: 1,
      onlineUpdatedAt: '2026-07-18 12:34:56'
    },
    global: {
      stubs: {
        NSpin: SlotStub,
        NCard: CardStub,
        NDescriptions: SlotStub,
        NDescriptionsItem: DescriptionItemStub,
        RdiTelemetrySummary: TelemetrySummaryStub
      }
    }
  })
}

describe('RdiDeviceDetailsView', () => {
  beforeEach(() => {
    hoisted.rdiDeviceConfig.mockReset()
    hoisted.loadRealtimeState.mockClear()
    hoisted.startTelemetryRefresh.mockClear()
    hoisted.appStore.locale = 'en-US'
    hoisted.rdiDeviceConfig.mockResolvedValue({
      error: null,
      data: {
        device_id: 'device-1',
        device_name: 'Freezer Controller',
        pid_number: 'PID-1001',
        firmware_version: '3.4.5',
        online: true,
        connection_type: 'MQTT',
        config: {
          data_collection_interval: 45,
          alarm_sensor_1_enabled: true,
          alarm_sensor_2_enabled: false,
          sensor_1_upper: 12.5,
          sensor_1_lower: -5,
          sensor_2_upper: 20,
          sensor_2_lower: 2,
          sensor_1_duration: 25,
          sensor_2_duration: 30,
          switch_1_alarm_mode: 'powered_on',
          switch_2_alarm_mode: 'disabled',
          switch_1_alarm_duration: 15,
          switch_2_alarm_duration: 20,
          dry_contact_alarm_level: 'high',
          dry_contact_normal_level: 'low',
          dry_contact_alarm_delay: 5,
          dry_contact_normal_delay: 10,
          notification_enabled: true,
          notification_temperature_alarm: true,
          notification_switch_alarm: false,
          notification_warranty_alarm: true,
          sensor_alarm_emails: 'private-alerts@example.com',
          field_setting: {
            n00: ['1', '2'],
            sw1: { label: 'Enabled', value: 1 }
          }
        },
        additional_info: {},
        thing_model: {},
        system_info: {
          installation_location: 'Cold Room A',
          address: '18 Harbour Road',
          installation_date: '2026-07-01',
          installer_company: 'Northwind Controls',
          installer_contact: 'Alex Smith',
          installer_name: 'Jordan Lee',
          installer_phone: '+1 555 0100',
          installer_email: 'installer@example.com',
          controller_serial_number: 'CTRL-9001',
          customer_name: 'Alpine Foods',
          maintenance_technician: 'Morgan Chen',
          contact_phone: '+1 555 0200',
          contact_email: 'customer@example.com',
          warranty_status: 'Active'
        }
      }
    })
  })

  it('loads and renders the customer-facing RDI device and system information as read-only details', async () => {
    const wrapper = mountDetails()
    await flushPromises()

    expect(hoisted.rdiDeviceConfig).toHaveBeenCalledWith('device-1')
    expect(wrapper.text()).toContain('Basic Info')
    expect(wrapper.text()).toContain('Freezer Controller')
    expect(wrapper.text()).toContain('PID-1001')
    expect(wrapper.text()).toContain('3.4.5')
    expect(wrapper.text()).toContain('Online')
    expect(wrapper.text()).toContain('18 Harbour Road')
    expect(wrapper.text()).toContain('2026-07-01')
    expect(wrapper.text()).toContain('Northwind Controls')
    expect(wrapper.text()).toContain('Alex Smith')
    expect(wrapper.text()).toContain('Jordan Lee')
    expect(wrapper.text()).toContain('installer@example.com')
    expect(wrapper.text()).toContain('CTRL-9001')
    expect(wrapper.text()).toContain('Alpine Foods')
    expect(wrapper.text()).toContain('Morgan Chen')
    expect(wrapper.text()).toContain('Active')
    expect(wrapper.text()).toContain('2026-07-18 12:34:56')
    expect(wrapper.text()).toContain('Configured Parameter Settings')
    expect(wrapper.text()).toContain('Device auto-reporting interval (s)')
    expect(wrapper.text()).toContain('45 s')
    expect(wrapper.text()).toContain('Alarm range: -5.00 - 12.50 C')
    expect(wrapper.text()).toContain('Mode: Powered on')
    expect(wrapper.text()).toContain('Output Level During Alarm: High')
    expect(wrapper.text()).toContain('Recovery Delay: 10 s')
    expect(wrapper.text()).toContain('Temperature alarm notification: Enabled')
    expect(wrapper.text()).not.toContain('n00=1, 2')
    expect(wrapper.text()).not.toContain('private-alerts@example.com')
    expect(wrapper.text()).toContain('T1: 5.50 C')
    expect(wrapper.text()).toContain('T2: 6.25 C')
    expect(wrapper.text()).toContain('Input Node 1: High')
    expect(wrapper.text()).toContain('Input Node 2: Low')
    expect(wrapper.text()).toContain('Output Node 01: High')
    expect(wrapper.text()).toContain('LED1: Solid')
    expect(wrapper.text()).toContain('kWh: 12.40 kWh')
    expect(hoisted.loadRealtimeState).toHaveBeenCalledTimes(1)
    expect(hoisted.startTelemetryRefresh).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(wrapper.findAll('button')).toHaveLength(0)
  })

  it('shows one output trigger time when the saved alarm and recovery delays are equal', async () => {
    hoisted.rdiDeviceConfig.mockResolvedValueOnce({
      error: null,
      data: {
        device_id: 'device-1',
        config: {
          dry_contact_alarm_level: 'high',
          dry_contact_normal_level: 'low',
          dry_contact_alarm_delay: 7,
          dry_contact_normal_delay: 7
        },
        system_info: {}
      }
    })

    const wrapper = mountDetails()
    await flushPromises()

    expect(wrapper.text()).toContain('Trigger Effective Time: 7 s')
    expect(wrapper.text()).not.toContain('Alarm Delay: 7 s')
    expect(wrapper.text()).not.toContain('Recovery Delay: 7 s')
  })

  it('reads promoted installation fields from legacy extra_fields and reloads when the device changes', async () => {
    hoisted.appStore.locale = 'zh-CN'
    hoisted.rdiDeviceConfig
      .mockResolvedValueOnce({
        error: null,
        data: {
          device_id: 'device-1',
          online: false,
          system_info: {
            address: '',
            controller_serial_number: '',
            extra_fields: {
              address: '历史安装地址',
              controller_serial_number: 'OLD-CTRL-1'
            }
          }
        }
      })
      .mockResolvedValueOnce({
        error: null,
        data: {
          device_id: 'device-2',
          device_name: 'Second Controller',
          online: true,
          system_info: {}
        }
      })

    const wrapper = mountDetails()
    await flushPromises()

    expect(wrapper.text()).toContain('安装地址')
    expect(wrapper.text()).toContain('历史安装地址')
    expect(wrapper.text()).toContain('控制器序列号')
    expect(wrapper.text()).toContain('OLD-CTRL-1')
    expect(wrapper.text()).toContain('Online')

    await wrapper.setProps({ id: 'device-2' })
    await flushPromises()

    expect(hoisted.rdiDeviceConfig).toHaveBeenNthCalledWith(2, 'device-2')
    expect(hoisted.loadRealtimeState).toHaveBeenCalledTimes(2)
    expect(hoisted.startTelemetryRefresh).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('Second Controller')
  })

  it('renders custom extra_fields beyond the promoted named fields, humanizing the key', async () => {
    hoisted.rdiDeviceConfig.mockResolvedValueOnce({
      error: null,
      data: {
        device_id: 'device-1',
        online: true,
        system_info: {
          customer_name: 'Acme Cold Chain',
          extra_fields: {
            // promoted key must NOT appear again in the custom section
            customer_name: 'Acme Cold Chain',
            room_number: 'A-101',
            service_contract: 'GOLD-2026'
          }
        }
      }
    })

    const wrapper = mountDetails()
    await flushPromises()

    const text = wrapper.text()
    // humanized labels for the two genuinely-custom keys
    expect(text).toContain('Room Number')
    expect(text).toContain('A-101')
    expect(text).toContain('Service Contract')
    expect(text).toContain('GOLD-2026')
    // the promoted key stays in the named system card, not duplicated as a custom row
    const customLabelHits = text.split('Customer Name').length - 1
    expect(customLabelHits).toBe(0)
  })

  it('omits the extended-fields card when every extra_fields key is a promoted field', async () => {
    hoisted.rdiDeviceConfig.mockResolvedValueOnce({
      error: null,
      data: {
        device_id: 'device-1',
        online: true,
        system_info: {
          extra_fields: {
            address: 'legacy address only'
          }
        }
      }
    })

    const wrapper = mountDetails()
    await flushPromises()

    expect(wrapper.find('[data-testid="rdi-additional-fields"]').exists()).toBe(false)
  })
})
