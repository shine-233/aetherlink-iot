/**
 * 文件用途: join 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConnectForm: vi.fn(),
  getDeviceConnectionGuide: vi.fn(),
  getDeviceConnectionDiagnostics: vi.fn(),
  getDeviceConnectInfo: vi.fn(),
  getPlugininfoByService: vi.fn(),
  updateDeviceVoucher: vi.fn(),
  fetchData: vi.fn(),
  deviceData: {
    device_config: { protocol_type: 'MQTT' },
    voucher: '{}',
    access_way: 'A',
    device_number: 'D-001'
  } as any
}))

vi.mock('@/service/api/device', () => ({
  deviceConnectForm: hoisted.deviceConnectForm,
  getDeviceConnectionGuide: hoisted.getDeviceConnectionGuide,
  getDeviceConnectionDiagnostics: hoisted.getDeviceConnectionDiagnostics,
  getDeviceConnectInfo: hoisted.getDeviceConnectInfo,
  getPlugininfoByService: hoisted.getPlugininfoByService,
  updateDeviceVoucher: hoisted.updateDeviceVoucher
}))

vi.mock('@/store/modules/device', () => ({
  useDeviceDataStore: () => ({
    fetchData: hoisted.fetchData,
    deviceData: hoisted.deviceData
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push: vi.fn() })
}))

import Component from '../join.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { id: 'device-1', ...props },
    global: {
      stubs: {
        NCard: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NForm: defineComponent({
          props: ['rules', 'model'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NFormItem: defineComponent({
          props: ['label', 'path'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NInput: defineComponent({
          props: ['value', 'placeholder'],
          emits: ['update:value'],
          setup() {
            return () => h('input')
          }
        }),
        NSelect: defineComponent({
          props: ['value', 'options'],
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NAlert: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NCode: defineComponent({
          props: ['code', 'language'],
          setup(props) {
            return () => h('pre', String(props.code || ''))
          }
        }),
        NDescriptions: defineComponent({
          props: ['column'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NDescriptionsItem: defineComponent({
          props: ['label'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NButton: defineComponent({
          props: ['type'],
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NScrollbar: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/details/modules/join.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceConnectForm.mockResolvedValue({ data: [] })
    hoisted.getDeviceConnectionGuide.mockResolvedValue({ data: null })
    hoisted.getDeviceConnectionDiagnostics.mockResolvedValue({ data: {} })
    hoisted.getDeviceConnectInfo.mockResolvedValue({ data: {} })
    hoisted.getPlugininfoByService.mockResolvedValue({ error: null, data: {} })
    hoisted.fetchData.mockResolvedValue(undefined)
    hoisted.updateDeviceVoucher.mockResolvedValue({ error: null })
    hoisted.deviceData = {
      device_config: { protocol_type: 'MQTT' },
      voucher: '{}',
      access_way: 'A',
      device_number: 'D-001'
    }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('builds dynamic voucher form rules from fetched form schema', async () => {
    hoisted.deviceConnectForm.mockResolvedValue({
      data: [
        {
          type: 'input',
          dataKey: 'username',
          label: 'Username',
          placeholder: 'Input username',
          validate: { required: true, message: 'Username required' }
        },
        {
          type: 'table',
          dataKey: 'params',
          label: 'Params',
          array: [
            {
              type: 'input',
              dataKey: 'clientId',
              label: 'Client ID',
              validate: { required: true, message: 'Client ID required' }
            }
          ]
        }
      ]
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.deviceConnectForm).toHaveBeenCalledWith({ device_id: 'device-1' })
    expect(state.formElements.map((item: any) => item.dataKey)).toEqual(['username', 'params'])
    expect(state.formRules).toMatchObject({
      username: { required: true, message: 'Username required' },
      clientId: { required: true, message: 'Client ID required' }
    })
    expect(state.formData).toMatchObject({
      username: '',
      clientId: ''
    })
  })

  it('fetches form json on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceConnectForm).toHaveBeenCalledWith({ device_id: 'device-1' })
  })

  it('fetches connect info on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getDeviceConnectInfo).toHaveBeenCalledWith({ device_id: 'device-1' })
    expect(hoisted.getDeviceConnectionGuide).toHaveBeenCalledWith('device-1', {
      debug_log_limit: 5,
      command_log_limit: 3
    })
  })

  it('uses stable connection guide profile before localized connect-info labels', async () => {
    hoisted.getDeviceConnectInfo.mockResolvedValue({
      data: {
        '接入地址': 'localized-broker.example.com:1883',
        '上报 Topic': 'localized/topic'
      }
    })
    hoisted.getDeviceConnectionGuide.mockResolvedValue({
      data: {
        access: {
          protocol: 'MQTT',
          credential_mode: 'BASIC',
          connection_profile: {
            protocol: 'MQTT',
            endpoint: 'mqtts://stable-broker.example.com:8883',
            host: 'stable-broker.example.com',
            port: '8883',
            client_id: 'mqtt_device_001',
            username: 'mqtt_device_001',
            telemetry_topic: 'devices/telemetry',
            command_topic: 'devices/telemetry/control/D-001',
            sample_payload: '{"temperature":26}',
            tls_enabled: true
          },
          tls: { enabled: true }
        },
        readiness: {
          online: true,
          ready: true,
          level: 'ok',
          code: 'ready',
          summary: 'ready'
        }
      }
    })

    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(state.accessGuide.endpoint).toBe('mqtts://stable-broker.example.com:8883')
    expect(state.accessGuide.reportTopic).toBe('devices/telemetry')
    expect(state.accessGuide.commands[0].code).toContain('stable-broker.example.com')
  })

  it('uses connection diagnostics debug errors in the access guide', async () => {
    hoisted.getDeviceConnectionDiagnostics.mockResolvedValue({
      data: {
        online: {
          is_online: false
        },
        debug: {
          enabled: true,
          recent_logs: [
            {
              direction: 'uplink',
              action: 'publish',
              error: 'invalid payload'
            }
          ]
        },
        diagnostics: {
          recent_failures: [
            {
              stage: 'collector',
              direction: 'uplink',
              error: 'older failure'
            }
          ]
        }
      }
    })

    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.getDeviceConnectionDiagnostics).toHaveBeenCalledWith('device-1', { debug_log_limit: 5 })
    expect(state.accessGuide.lastError).toBe('[publish] uplink: invalid payload')
    expect(state.accessGuide.diagnostics).toContainEqual(
      expect.objectContaining({
        labelKey: 'custom.device_details.accessGuideDiagnosticDebug',
        valueKey: 'custom.device_details.accessGuideDiagnosticDebugOn',
        tone: 'success'
      })
    )
    expect(state.accessGuide.diagnostics).toContainEqual(
      expect.objectContaining({
        labelKey: 'custom.device_details.accessGuideDiagnosticOnline',
        valueKey: 'custom.device_details.offline',
        tone: 'warning'
      })
    )
  })

  it('calls deviceDataStore.fetchData on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchData).toHaveBeenCalledWith('device-1')
  })

  it('handleSubmit calls updateDeviceVoucher', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    await wrapper.vm.handleSubmit()
    expect(hoisted.updateDeviceVoucher).toHaveBeenCalledWith({
      device_id: 'device-1',
      voucher: expect.any(String)
    })
  })

  it('handleSubmit shows success message', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    const state = getSetupState(wrapper)
    state.formRef = { validate: mockValidate }
    await wrapper.vm.handleSubmit()
    expect(mockValidate).toHaveBeenCalledTimes(1)
    expect(hoisted.updateDeviceVoucher).toHaveBeenCalledWith({
      device_id: 'device-1',
      voucher: expect.any(String)
    })
    expect(window.$message?.success).toHaveBeenCalledTimes(1)
    expect(window.$message?.success).toHaveBeenCalledWith('common.updateSuccess')
  })

  it('keeps the connection guide usable when stored voucher JSON is invalid', async () => {
    hoisted.deviceData = {
      device_config: { protocol_type: 'MQTT' },
      voucher: '{bad-json',
      access_way: 'A',
      device_number: 'D-001'
    }
    hoisted.deviceConnectForm.mockResolvedValue({
      data: [
        {
          type: 'input',
          dataKey: 'username',
          label: 'Username',
          validate: { required: true }
        }
      ]
    })

    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(state.formData).toMatchObject({ username: '' })
    expect(state.accessGuide.username).toBe('<mqtt-username>')
  })

  it('getPlugininfoByService is called with protocol_type', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getPlugininfoByService).toHaveBeenCalledWith({ service_identifier: 'MQTT' })
  })

  it('defaults to MQTT when no protocol_type', async () => {
    hoisted.deviceData = {
      device_config: { protocol_type: null },
      voucher: '{}',
      access_way: 'A',
      device_number: 'D-001'
    }
    mountComponent()
    await flushPromises()
    expect(hoisted.getPlugininfoByService).toHaveBeenCalledTimes(1)
    expect(hoisted.getPlugininfoByService).toHaveBeenCalledWith({ service_identifier: 'MQTT' })
  })
})
