/**
 * 文件用途: rdi-device-operations-view 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { labels } from '../rdi/constants/rdi-labels'

const mockLoadConfig = vi.fn()
const mockLoadRealtimeState = vi.fn()
const mockLoadEnergyStatistics = vi.fn()
const mockLoadOtaPackages = vi.fn()
const mockStartTelemetryRefresh = vi.fn()
const mockResetShareState = vi.fn()
const mockSaveConfig = vi.fn()
const mockSetFieldValue = vi.fn()
const mockSetSystemExtraField = vi.fn()
const mockExportHistoryData = vi.fn()
const mockSetDryContact = vi.fn()
const mockTestDryContact = vi.fn()
const mockSendFieldSetting = vi.fn()
const mockSendOtaUpgrade = vi.fn()
const mockSendUnbindDevice = vi.fn()
const mockSendFactoryReset = vi.fn()
const mockApplyLatestFirmwarePackage = vi.fn()
const mockCheckLatestFirmware = vi.fn()
const mockCreateShare = vi.fn()
const mockCopyShare = vi.fn()
const mockShareLink = ref('')
const mockShareExpiresAt = ref('')
const mockCommandTrackingSummary = ref('')
const mockTelemetryRows = ref([{ label: 'T1', value: '1.00', unit: 'C' }])
const mockTelemetry = {
  temperature_1: 1,
  temperature_2: 2,
  switch_1: 'closed',
  switch_2: 'open',
  dry_contact_output: 'high'
}

const mockLiveOnlineStatus = ref<number | null>(2)

function createMockConfig() {
  return {
    alarm_sensor_1_enabled: true,
    alarm_sensor_2_enabled: true,
    sensor_1_lower: -10,
    sensor_1_upper: 80,
    sensor_1_duration: 30,
    sensor_2_lower: -10,
    sensor_2_upper: 80,
    sensor_2_duration: 30,
    switch_1_alarm_mode: 'disabled',
    switch_1_alarm_duration: 30,
    switch_2_alarm_mode: 'disabled',
    switch_2_alarm_duration: 30,
    dry_contact_alarm_level: 'high',
    dry_contact_normal_level: 'low',
    dry_contact_alarm_delay: 0,
    dry_contact_normal_delay: 0,
    notification_enabled: false,
    notification_temperature_alarm: true,
    notification_switch_alarm: true,
    notification_warranty_alarm: true,
    data_collection_interval: 60,
    sensor_alarm_emails: '',
    switch_alarm_emails: '',
    warranty_alarm_emails: '',
    sensor_1_alarm_emails: '',
    sensor_2_alarm_emails: '',
    switch_1_alarm_emails: '',
    switch_2_alarm_emails: ''
  }
}

const mockConfig = createMockConfig()

vi.mock('../rdi/composables/useRdiConfig', () => ({
  useRdiConfig: () => ({
    loading: ref(false),
    applyToDevice: ref(false),
    config: mockConfig,
    systemInfo: {
      installation_location: '',
      address: '',
      installation_date: '',
      installer_company: '',
      installer_contact: '',
      installer_name: '',
      installer_phone: '',
      installer_email: '',
      controller_serial_number: '',
      maintenance_technician: '',
      customer_name: '',
      contact_email: '',
      contact_phone: '',
      warranty_status: ''
    },
    sensor1Range: ref([-10, 80]),
    sensor2Range: ref([-10, 80]),
    fieldEntries: [],
    t: (key: string) => labels['en-US'][key as keyof (typeof labels)['en-US']] ?? key,
    systemExtraFieldLabel: (key: string) => key,
    systemExtraFieldDefinitions: [{ key: 'site_name' }],
    setFieldValue: mockSetFieldValue,
    getFieldValue: vi.fn().mockReturnValue(''),
    setSystemExtraField: mockSetSystemExtraField,
    getSystemExtraField: vi.fn().mockReturnValue(''),
    loadConfig: mockLoadConfig,
    saveConfig: mockSaveConfig
  })
}))

vi.mock('../rdi/composables/useRdiTelemetry', () => ({
  useRdiTelemetry: () => ({
    telemetry: mockTelemetry,
    liveOnlineStatus: mockLiveOnlineStatus,
    temperatureUnit: ref<'C' | 'F'>('C'),
    telemetryRows: mockTelemetryRows,
    deviceOnlineText: ref('online'),
    deviceDescriptionText: ref('desc'),
    toAxisValue: vi.fn(value => value),
    formatSwitch: vi.fn(value => String(value ?? '--')),
    loadRealtimeState: mockLoadRealtimeState,
    startTelemetryRefresh: mockStartTelemetryRefresh
  })
}))

vi.mock('../rdi/composables/useRdiHistory', () => ({
  useRdiHistory: () => ({
    RDI_DURATION_MAX_SECONDS: 86400,
    energyLoading: ref(false),
    historyExportLoading: ref(false),
    energyRange: ref('last1Day'),
    energyCustomRange: ref(null),
    historyExportKey: ref('electricity_consumption'),
    historyExportFormat: ref('csv'),
    energyStats: {
      latest: 0,
      delta: 0,
      min: 0,
      max: 0,
      sample_count: 0
    },
    historyChartOptions: ref({}),
    energyRangeOptions: ref([]),
    historyChartSeriesKeys: ref([]),
    historyChartSeriesOptions: ref([]),
    historyExportKeyOptions: ref([]),
    historyExportFormatOptions: ref([]),
    formatDurationLabel: vi.fn((value: number) => `${value}s`),
    formatEnergyValue: vi.fn((value: number) => String(value)),
    loadEnergyStatistics: mockLoadEnergyStatistics,
    exportHistoryData: mockExportHistoryData
  })
}))

vi.mock('../rdi/composables/useRdiCommands', () => ({
  useRdiCommands: () => ({
    commandLoading: ref(false),
    dryCommandDelay: ref(0),
    dryTestDuration: ref(1),
    commandTrackingSummary: mockCommandTrackingSummary,
    otaPackageLoading: ref(false),
    otaPackageId: ref(''),
    latestFirmwareLoading: ref(false),
    latestFirmwarePackage: ref(null),
    lastCommandTracking: ref(null),
    otaCommand: {
      firmware_url: '',
      version: '',
      size: 1,
      md5: ''
    },
    otaPackageOptions: ref([]),
    otaMissingFieldLabels: ref([]),
    canSendOtaUpgrade: ref(true),
    setDryContact: mockSetDryContact,
    testDryContact: mockTestDryContact,
    sendFieldSetting: mockSendFieldSetting,
    sendOtaUpgrade: mockSendOtaUpgrade,
    sendUnbindDevice: mockSendUnbindDevice,
    sendFactoryReset: mockSendFactoryReset,
    loadOtaPackages: mockLoadOtaPackages,
    applyLatestFirmwarePackage: mockApplyLatestFirmwarePackage,
    checkLatestFirmware: mockCheckLatestFirmware
  })
}))

vi.mock('../rdi/composables/useRdiShare', () => ({
  useRdiShare: () => ({
    shareLoading: ref(false),
    shareExpiresIn: ref(604800),
    shareLink: mockShareLink,
    shareExpiryOptions: ref([]),
    shareExpiresAt: mockShareExpiresAt,
    shareActions: {
      create: mockCreateShare,
      copy: mockCopyShare
    },
    resetShareState: mockResetShareState
  })
}))

import RdiDeviceOperationsView from '../RdiDeviceOperationsView.vue'

const naiveStubs = {
  NSpin: true,
  NAlert: true,
  NButton: true,
  NFormItem: true,
  NSelect: true,
  NDatePicker: true,
  NInput: true,
  NInputNumber: true,
  NSwitch: true,
  NSlider: true,
  NTabs: true,
  NTabPane: true,
  NTag: true,
  NPopconfirm: true,
  NCheckbox: true,
  NEmpty: true,
  ChartComponent: true,
  RdiTemperatureAlarmAxis: true
}

const SlotStub = defineComponent({
  name: 'SlotStub',
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const FormItemStub = defineComponent({
  name: 'NFormItem',
  props: ['label'],
  setup(props, { slots }) {
    return () => h('div', [props.label ? h('span', String(props.label)) : null, slots.default?.()])
  }
})

const ButtonStub = defineComponent({
  name: 'NButton',
  props: ['disabled'],
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          onClick: () => {
            if (!props.disabled) emit('click')
          }
        },
        slots.default?.()
      )
  }
})

const InputStub = defineComponent({
  name: 'NInput',
  props: ['value'],
  emits: ['update:value'],
  setup() {
    return () => h('input')
  }
})

const NumberInputStub = defineComponent({
  name: 'NInputNumber',
  props: ['value', 'min', 'max'],
  emits: ['update:value'],
  setup() {
    return () => h('input', { type: 'number' })
  }
})

const SelectStub = defineComponent({
  name: 'NSelect',
  props: ['value', 'options'],
  emits: ['update:value'],
  setup() {
    return () => h('div')
  }
})

const SwitchStub = defineComponent({
  name: 'NSwitch',
  props: ['value'],
  emits: ['update:value'],
  setup() {
    return () => h('button')
  }
})

const SliderStub = defineComponent({
  name: 'NSlider',
  props: ['value'],
  emits: ['update:value'],
  setup() {
    return () => h('div')
  }
})

const CheckboxStub = defineComponent({
  name: 'NCheckbox',
  props: ['checked'],
  emits: ['update:checked'],
  setup(_, { slots }) {
    return () => h('label', slots.default?.())
  }
})

const PopconfirmStub = defineComponent({
  name: 'NPopconfirm',
  emits: ['positive-click'],
  setup(_, { emit, slots }) {
    return () =>
      h('div', [
        slots.trigger?.(),
        h('button', { class: 'confirm-action', onClick: () => emit('positive-click') }, 'confirm')
      ])
  }
})

const TelemetrySummaryStub = defineComponent({
  name: 'RdiTelemetrySummary',
  props: ['rows', 'temperatureUnit'],
  emits: ['update:temperatureUnit'],
  setup(props, { emit }) {
    return () =>
      h('div', [
        h(SelectStub, {
          value: props.temperatureUnit,
          'onUpdate:value': (value: string) => emit('update:temperatureUnit', value)
        }),
        ...(props.rows || []).map((row: { label: string; value: string; unit?: string }) =>
          h('span', `${row.label}${row.value}${row.unit || ''}`)
        )
      ])
  }
})

const RdiOperationsViewStub = defineComponent({
  name: 'RdiOperationsView',
  props: ['shareLink', 'shareExpiresAt', 'commandTrackingSummary'],
  emits: [
    'apply-latest-firmware',
    'check-latest-firmware',
    'copy-share',
    'create-share',
    'load-ota-packages',
    'send-factory-reset',
    'send-ota-upgrade',
    'send-unbind-device',
    'set-dry-contact',
    'test-dry-contact'
  ],
  setup(props, { emit }) {
    return () =>
      h('div', [
        props.shareExpiresAt ? h('span', `expires: ${props.shareExpiresAt}`) : null,
        props.commandTrackingSummary ? h('span', props.commandTrackingSummary) : null,
        h(ButtonStub, { disabled: !props.shareLink, onClick: () => emit('copy-share') }, () => 'copy'),
        h(ButtonStub, { onClick: () => emit('create-share') }, () => 'createShare'),
        h(ButtonStub, { onClick: () => emit('set-dry-contact', 'high') }, () => 'high'),
        h(ButtonStub, { onClick: () => emit('set-dry-contact', 'low') }, () => 'low'),
        h(ButtonStub, { onClick: () => emit('test-dry-contact') }, () => 'testDry'),
        h(ButtonStub, { onClick: () => emit('load-ota-packages') }, () => 'loadOta'),
        h(ButtonStub, { onClick: () => emit('check-latest-firmware') }, () => 'checkFirmware'),
        h(ButtonStub, { onClick: () => emit('apply-latest-firmware') }, () => 'applyFirmware'),
        h(ButtonStub, { onClick: () => emit('send-ota-upgrade') }, () => 'sendOta'),
        h(ButtonStub, { onClick: () => emit('send-unbind-device') }, () => 'unbind'),
        h(ButtonStub, { onClick: () => emit('send-factory-reset') }, () => 'factoryReset')
      ])
  }
})

const AxisStub = defineComponent({
  name: 'RdiTemperatureAlarmAxis',
  emits: ['update:lower', 'update:upper'],
  setup() {
    return () => h('div', { class: 'axis-stub' })
  }
})

const interactiveStubs = {
  NSpin: SlotStub,
  NAlert: SlotStub,
  NButton: ButtonStub,
  NFormItem: FormItemStub,
  NSelect: SelectStub,
  NDatePicker: defineComponent({ name: 'NDatePicker', props: ['value'], emits: ['update:value'], setup: () => () => h('div') }),
  NInput: InputStub,
  NInputNumber: NumberInputStub,
  NSwitch: SwitchStub,
  NSlider: SliderStub,
  NTabs: SlotStub,
  NTabPane: SlotStub,
  NTag: SlotStub,
  NPopconfirm: PopconfirmStub,
  NCheckbox: CheckboxStub,
  NEmpty: SlotStub,
  ChartComponent: true,
  RdiTelemetrySummary: TelemetrySummaryStub,
  RdiOperationsView: RdiOperationsViewStub,
  RdiTemperatureAlarmAxis: AxisStub
}

describe('RdiDeviceOperationsView.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockLiveOnlineStatus.value = 2
    mockShareLink.value = ''
    mockShareExpiresAt.value = ''
    mockCommandTrackingSummary.value = ''
    mockTelemetryRows.value = [{ label: 'T1', value: '1.00', unit: 'C' }]
    Object.assign(mockTelemetry, {
      temperature_1: 1,
      temperature_2: 2,
      switch_1: 'closed',
      switch_2: 'open',
      dry_contact_output: 'high'
    })
    Object.assign(mockConfig, createMockConfig())
    mockLoadConfig.mockResolvedValue(undefined)
    mockLoadRealtimeState.mockResolvedValue(undefined)
    mockLoadEnergyStatistics.mockResolvedValue(undefined)
    mockLoadOtaPackages.mockResolvedValue(undefined)
    mockExportHistoryData.mockResolvedValue(undefined)
  })

  it('loads config, telemetry, energy stats, packages, and refresh on mount', async () => {
    shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: naiveStubs
      }
    })

    await flushPromises()

    expect(mockLoadConfig).toHaveBeenCalledTimes(1)
    expect(mockLoadRealtimeState).toHaveBeenCalledTimes(1)
    expect(mockLoadEnergyStatistics).toHaveBeenCalledTimes(1)
    expect(mockLoadOtaPackages).not.toHaveBeenCalled()
    expect(mockStartTelemetryRefresh).toHaveBeenCalledTimes(1)
  })

  it('loads history stats on first mount before any manual load click', async () => {
    shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-history-1',
        online: 1,
        deviceData: { device_number: 'RDI-001' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    expect(mockLoadEnergyStatistics).toHaveBeenCalledTimes(1)
    expect(mockExportHistoryData).not.toHaveBeenCalled()
  })

  it('renders fallback device metadata, telemetry values without units, and disabled share copy state', async () => {
    mockTelemetryRows.value = [{ label: 'Door', value: 'closed', unit: '' }]

    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1'
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('PID: --')
    expect(wrapper.text()).toContain('Door')
    expect(wrapper.text()).toContain('closed')
    expect(wrapper.findAllComponents(ButtonStub).some(button => button.attributes('disabled') !== undefined)).toBe(true)
  })

  it('renders the screenshot-aligned basic info section from device detail fields', async () => {
    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 0,
        onlineUpdatedAt: '2026-06-29 16:17:56',
        deviceData: {
          name: 'ming Desk Lamp',
          device_number: 'rvd165fhgt_0000005',
          current_version: '',
          description: '',
          created_at: '2026-06-26T09:27:33Z',
          update_at: '2026-06-30T08:00:00Z'
        }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    const basicInfoColumns = wrapper.findAll('.rdi-basic-info-column')
    expect(basicInfoColumns).toHaveLength(2)
    expect(basicInfoColumns[0].findAll('.rdi-basic-info-row')).toHaveLength(4)
    expect(basicInfoColumns[1].findAll('.rdi-basic-info-row')).toHaveLength(3)
    expect(wrapper.find('.rdi-basic-info-chip').text()).toBe('rvd165fhgt_0000005')
    expect(wrapper.find('.rdi-basic-info-status-dot').classes()).not.toContain('rdi-basic-info-status-dot--online')

    const text = wrapper.text()
    expect(text).toContain('Basic Info')
    expect(text).toContain('Status')
    expect(text).toContain('Device Name')
    expect(text).toContain('ming Desk Lamp')
    expect(text).toContain('Device ID')
    expect(text).toContain('rvd165fhgt_0000005')
    expect(text).toContain('Added At')
    expect(text).toContain('2026-06-26 09:27:33')
    expect(text).toContain('Last Heartbeat')
    expect(text).toContain('2026-06-29 16:17:56')
    expect(text).not.toContain('2026-06-30 08:00:00')
  })

  it('renders the system information maintenance fields required by the manual', async () => {
    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('installation_date')
    expect(text).toContain('installer_company')
    expect(text).toContain('installer_contact')
    expect(text).toContain('installer_name')
    expect(text).toContain('installer_phone')
    expect(text).toContain('installer_email')
    expect(text).toContain('controller_serial_number')
    expect(text).toContain('Technician')
    expect(text).toContain('Customer')
    expect(text).toContain('Email')
    expect(text).toContain('Phone')
    expect(text).toContain('Warranty')
  })

  it('renders current switch and dry-contact status values in the detail sections', async () => {
    Object.assign(mockTelemetry, {
      switch_1: 'alarm-open',
      switch_2: 'alarm-closed',
      dry_contact_output: 'output-high'
    })

    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('alarm-open')
    expect(text).toContain('alarm-closed')
    expect(text).toContain('output-high')
  })

  it('explains the alarm email fallback to global warning-email recipients', async () => {
    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('global warning-email recipients from system settings')
  })

  it('shows the generated share expiration timestamp when a share link exists', async () => {
    mockShareLink.value = 'https://share.test/rdi/dev-1'
    mockShareExpiresAt.value = '2026-07-01 10:00:00'

    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('expires: 2026-07-01 10:00:00')
    const copyButton = wrapper.findAllComponents(ButtonStub).find(button => button.text() === 'copy')
    expect(copyButton?.attributes('disabled')).toBeUndefined()
  })

  it('passes command tracking summary into the operations view', async () => {
    mockCommandTrackingSummary.value = 'ota_upgrade message_id=mid-7 status=queued (logged)'

    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('message_id=mid-7')
  })

  it('wires RDI form changes and command buttons to the composables', async () => {
    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1', current_version: '1.0.0', protocol: 'MQTT' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()
    vi.clearAllMocks()
    mockShareLink.value = 'https://share.test/rdi/dev-1'
    await wrapper.vm.$nextTick()

    const setupState = wrapper.vm.$.setupState as Record<string, any>

    const selects = wrapper.findAllComponents(SelectStub)
    await selects[0].vm.$emit('update:value', 'F')
    await selects[1].vm.$emit('update:value', 'custom')
    await selects[3].vm.$emit('update:value', 'temperature_1')
    await selects[4].vm.$emit('update:value', 'excel')

    const inputs = wrapper.findAllComponents(InputStub)
    await inputs[0].vm.$emit('update:value', 'n00-value')
    await inputs[33].vm.$emit('update:value', 'cold-room')

    const numberInputs = wrapper.findAllComponents(NumberInputStub)
    await numberInputs[0].vm.$emit('update:value', -5)

    const switches = wrapper.findAllComponents(SwitchStub)
    await switches[0].vm.$emit('update:value', false)

    const sliders = wrapper.findAllComponents(SliderStub)
    await sliders[0].vm.$emit('update:value', [-5, 75])

    const axes = wrapper.findAllComponents(AxisStub)
    await axes[0].vm.$emit('update:lower', -6)
    await axes[0].vm.$emit('update:upper', 76)

    expect(setupState.temperatureUnit).toBe('F')
    expect(setupState.energyRange).toBe('custom')
    expect(setupState.historyExportKey).toBe('temperature_1')
    expect(setupState.historyExportFormat).toBe('excel')
    expect(mockSetFieldValue).toHaveBeenCalledWith('n00', 'n00-value')
    expect(mockSetSystemExtraField).toHaveBeenCalledWith('site_name', 'cold-room')
    expect(setupState.config.sensor_1_lower).toBe(-5)
    expect(setupState.config.alarm_sensor_1_enabled).toBe(false)

    const buttons = wrapper.findAllComponents(ButtonStub)
    for (const button of buttons) {
      await button.trigger('click')
    }
    for (const confirm of wrapper.findAll('.confirm-action')) {
      await confirm.trigger('click')
    }

    expect(mockLoadConfig).toHaveBeenCalled()
    expect(mockLoadEnergyStatistics).toHaveBeenCalled()
    expect(mockExportHistoryData).toHaveBeenCalled()
    expect(mockSendFieldSetting).toHaveBeenCalled()
    expect(mockSetDryContact).toHaveBeenCalledWith('high')
    expect(mockSetDryContact).toHaveBeenCalledWith('low')
    expect(mockTestDryContact).toHaveBeenCalled()
    expect(mockLoadOtaPackages).toHaveBeenCalled()
    expect(mockCheckLatestFirmware).toHaveBeenCalled()
    expect(mockSendOtaUpgrade).toHaveBeenCalled()
    expect(mockSendUnbindDevice).toHaveBeenCalled()
    expect(mockSendFactoryReset).toHaveBeenCalled()
    expect(mockCreateShare).toHaveBeenCalled()
    expect(mockCopyShare).toHaveBeenCalled()
    expect(mockSaveConfig).toHaveBeenCalled()
  })

  it('limits the RDI data collection interval input to 45-60 seconds', async () => {
    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    const intervalInput = wrapper
      .findAllComponents(NumberInputStub)
      .find(input => input.props('value') === 60 && input.props('min') === 45 && input.props('max') === 60)

    expect(intervalInput?.props()).toMatchObject({
      value: 60,
      min: 45,
      max: 60
    })
  })

  it('uses screenshot-aligned labels for the input node trigger settings', async () => {
    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text.match(/Alarm Trigger Level/g)?.length ?? 0).toBeGreaterThanOrEqual(2)
    expect(text.match(/Trigger Effective Time \(s\)/g)?.length ?? 0).toBeGreaterThanOrEqual(2)
  })

  it('collapses dry contact delay controls into one trigger effective time field when both delays match', async () => {
    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text.match(/Trigger Effective Time \(s\)/g)?.length ?? 0).toBeGreaterThanOrEqual(3)
    expect(text).not.toContain('Recovery Delay (s)')
  })

  it('shows distinct dry contact alarm and recovery delay fields when the values differ', async () => {
    mockConfig.dry_contact_alarm_delay = 120
    mockConfig.dry_contact_normal_delay = 240

    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: interactiveStubs
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Alarm Delay (s)')
    expect(text).toContain('Recovery Delay (s)')
    expect(text.match(/Trigger Effective Time \(s\)/g)?.length ?? 0).toBe(2)
  })

  it('resets share state and refreshes data when the device id changes', async () => {
    const wrapper = shallowMount(RdiDeviceOperationsView, {
      props: {
        id: 'dev-1',
        online: 1,
        deviceData: { device_number: 'D-1' }
      },
      global: {
        stubs: naiveStubs
      }
    })

    await flushPromises()
    vi.clearAllMocks()
    mockLiveOnlineStatus.value = 5

    await wrapper.setProps({ id: 'dev-2' })
    await flushPromises()

    expect(mockResetShareState).toHaveBeenCalledTimes(1)
    expect(mockLiveOnlineStatus.value).toBeNull()
    expect(mockLoadConfig).toHaveBeenCalledTimes(1)
    expect(mockLoadRealtimeState).toHaveBeenCalledTimes(1)
    expect(mockLoadEnergyStatistics).toHaveBeenCalledTimes(1)
    expect(mockStartTelemetryRefresh).toHaveBeenCalledTimes(1)
  })
})
