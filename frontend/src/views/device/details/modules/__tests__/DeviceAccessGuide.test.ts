/**
 * 文件用途: DeviceAccessGuide 组件测试，覆盖拆分后的凭证网格、诊断面板、调试证据与测试代码分区接线。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({ $t: (key: string) => key }))
vi.mock('@/utils/clipboard', () => ({ writeClipboardText: vi.fn() }))

import DeviceAccessGuide from '../DeviceAccessGuide.vue'
import { buildDeviceAccessGuideState } from '../device-access-guide-state'

const passthrough = (tag = 'div') =>
  defineComponent({
    inheritAttrs: false,
    setup(_, { slots, attrs }) {
      return () => h(tag, attrs, [slots.default?.(), slots.header?.(), slots.footer?.()])
    }
  })

const ButtonStub = defineComponent({
  inheritAttrs: false,
  emits: ['click'],
  setup(_, { slots, emit, attrs }) {
    return () => h('button', { ...attrs, onClick: () => emit('click') }, slots.default?.())
  }
})

const stubs = {
  NAlert: passthrough(),
  NButton: ButtonStub,
  NCard: passthrough(),
  NCode: defineComponent({ props: ['code'], setup: (props) => () => h('pre', props.code) }),
  NDescriptions: passthrough(),
  NDescriptionsItem: passthrough(),
  // DeviceAccessGuide 直接从 naive-ui 导入 NModal，按组件注册名 Modal 打桩。
  Modal: defineComponent({
    props: ['show'],
    setup:
      (props, { slots }) =>
      () =>
        props.show ? h('div', { class: 'modal' }, [slots.default?.(), slots.footer?.()]) : null
  }),
  NScrollbar: passthrough(),
  ConnectionProofSteps: true,
  DeviceMqttDebugWorkbench: true
}

function mountGuide(overrides: Record<string, unknown> = {}) {
  const accessGuide = buildDeviceAccessGuideState(
    {
      server: 'mqtts://broker.example.com:8883',
      username: 'mqtt_device_001',
      report_topic: 'devices/telemetry',
      control_topic: 'devices/telemetry/control/D-001',
      remark: '{"temperature": 23}'
    },
    'D-001',
    { username: 'device-user', password: 'device-pass' }
  )
  return mount(DeviceAccessGuide, {
    props: {
      deviceId: 'dev-1',
      accessGuide,
      connectInfo: { server: 'mqtts://broker.example.com:8883' },
      debugStatus: { enabled: true, expire_at: '', remaining_seconds: 42 } as any,
      debugLogs: [],
      ...overrides
    },
    global: { stubs }
  })
}

describe('DeviceAccessGuide.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders credential grid and forwards copy events from child sections', async () => {
    const wrapper = mountGuide()
    expect(wrapper.text()).toContain('device-pass')
    expect(wrapper.text()).toContain('devices/telemetry/control/D-001')
    expect(wrapper.text()).toContain('42s')

    await wrapper.find('[data-testid="device-access-guide-endpoint"] button').trigger('click')
    expect(wrapper.emitted('copy')?.[0]?.[0]).toBe(wrapper.props('accessGuide').endpoint)
  })

  it('masks the password when credentials are masked', () => {
    const wrapper = mountGuide({ credentialsMasked: true })
    const passwordTile = wrapper
      .findAll('.access-guide-metric')
      .find((tile) => tile.text().includes('custom.device_details.accessGuidePassword'))
    expect(passwordTile?.text()).toContain('******')
    expect(passwordTile?.find('button').exists()).toBe(false)
  })

  it('opens the support summary preview from the diagnostics panel and copies it', async () => {
    const wrapper = mountGuide()
    await wrapper.find('[data-testid="device-access-guide-support-bundle"]').trigger('click')
    const modal = wrapper.find('.modal')
    expect(modal.exists()).toBe(true)
    const copyButton = modal.findAll('button').find((button) => button.text() === 'generate.copy')
    await copyButton!.trigger('click')
    const copied = wrapper.emitted('copy')?.at(-1)?.[0]
    expect(typeof copied).toBe('string')
    expect(String(copied).length).toBeGreaterThan(0)
  })

  it('forwards debug and navigation actions', async () => {
    const wrapper = mountGuide()
    await wrapper.find('[data-testid="device-access-guide-run-ready-check"]').trigger('click')
    await wrapper.find('[data-testid="device-access-guide-open-twin"]').trigger('click')
    const disable = wrapper
      .findAll('button')
      .find((button) => button.text() === 'custom.device_details.accessGuideDebugDisable')
    await disable!.trigger('click')
    expect(wrapper.emitted('openReadyCheck')).toHaveLength(1)
    expect(wrapper.emitted('openTwinEvidence')).toHaveLength(1)
    expect(wrapper.emitted('disableDebug')).toHaveLength(1)
  })

  it('downloads the access packet through the shared JSON download helper', async () => {
    const message = { success: vi.fn(), warning: vi.fn(), error: vi.fn() }
    ;(window as any).$message = message
    window.URL.createObjectURL = vi.fn(() => 'blob:x')
    window.URL.revokeObjectURL = vi.fn()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const wrapper = mountGuide()
    await wrapper.find('[data-testid="device-access-guide-download-access-packet"]').trigger('click')

    expect(click).toHaveBeenCalledTimes(1)
    expect(message.success).toHaveBeenCalledWith('custom.device_details.accessGuideDownloadSdkBundleSuccess')
    click.mockRestore()
    delete (window as any).$message
  })
})
