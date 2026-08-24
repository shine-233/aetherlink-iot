/**
 * 文件用途：验证 ThingsVis 嵌入桥接组件 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { nextTick } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  clientInstances: [] as MockThingsVisClientInstance[],
  getThingsVisToken: vi.fn(),
  attributeDataPub: vi.fn(),
  commandDataPub: vi.fn(),
  deviceAlarmStatus: vi.fn(),
  telemetryDataHistoryList: vi.fn(),
  telemetryDataPub: vi.fn(),
  localStgGet: vi.fn()
}))

type MockClientHandler = (payload?: unknown) => void

interface MockThingsVisClientOptions {
  mode?: string
  url?: string
  style?: Record<string, unknown>
  [key: string]: unknown
}

interface MockThingsVisClientInstance {
  ready: boolean
  trustedSource: { postMessage: ReturnType<typeof vi.fn> }
  trustedOrigin: string
  handlers: Map<string, MockClientHandler>
  on: ReturnType<typeof vi.fn>
  postMessageToGuest: ReturnType<typeof vi.fn>
  loadWidgetConfig: ReturnType<typeof vi.fn>
  updateSchema: ReturnType<typeof vi.fn>
  pushPlatformFieldData: ReturnType<typeof vi.fn>
  pushPlatformFieldHistory: ReturnType<typeof vi.fn>
  requestSave: ReturnType<typeof vi.fn>
  destroy: ReturnType<typeof vi.fn>
  options: MockThingsVisClientOptions
}

vi.mock('@/utils/thingsvis/sdk/client', () => ({
  ThingsVisClient: class MockThingsVisClient implements MockThingsVisClientInstance {
    ready = false
    trustedSource = { postMessage: vi.fn() }
    trustedOrigin = 'https://studio.test'
    handlers = new Map<string, MockClientHandler>()
    on = vi.fn((event: string, handler: MockClientHandler) => {
      this.handlers.set(event, handler)
    })
    isTrustedMessageEvent = vi.fn(
      (event: MessageEvent) => event.source === this.trustedSource && event.origin === this.trustedOrigin
    )
    postMessageToGuest = vi.fn()
    loadWidgetConfig = vi.fn()
    updateSchema = vi.fn()
    pushPlatformFieldData = vi.fn()
    pushPlatformFieldHistory = vi.fn()
    requestSave = vi.fn()
    destroy = vi.fn()

    constructor(public options: MockThingsVisClientOptions) {
      hoisted.clientInstances.push(this)
    }
  }
}))

vi.mock('@/utils/thingsvis', () => ({
  getThingsVisToken: hoisted.getThingsVisToken
}))

vi.mock('@/service/api/device', () => ({
  attributeDataPub: hoisted.attributeDataPub,
  commandDataPub: hoisted.commandDataPub,
  deviceAlarmStatus: hoisted.deviceAlarmStatus,
  telemetryDataHistoryList: hoisted.telemetryDataHistoryList,
  telemetryDataPub: hoisted.telemetryDataPub
}))

vi.mock('@/utils/thingsvis/constants', async importOriginal => {
  const actual = await importOriginal<typeof import('@/utils/thingsvis/constants')>()
  return {
    ...actual,
    THINGSVIS_COMPAT_PROVIDER: actual.THINGSVIS_COMPAT_ALIAS,
    getPlatformApiBase: () => 'https://platform.test/api',
    getThingsVisApiBase: () => 'https://thingsvis.test/api',
    getThingsVisStudioBaseUrl: () => 'https://studio.test/thingsvis/'
  }
})

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: hoisted.localStgGet
  }
}))

import ThingsVisWidget from './ThingsVisWidget.vue'
import { THINGSVIS_COMPAT_ALIAS } from '@/utils/thingsvis/constants'

// 归一化后节点在断言里只用到这些字段；用一个受控结构类型替代散落的显式 any。
interface NormalizedWidgetNode {
  id: string
  props: Record<string, unknown>
  data?: Array<{ targetProp: string }>
  events?: Array<Record<string, unknown>>
}

const findNormalizedNode = (config: { nodes: unknown }, id: string): NormalizedWidgetNode | undefined =>
  (config.nodes as NormalizedWidgetNode[]).find(node => node.id === id)

const mountedWrappers: VueWrapper[] = []

const flushAsync = async () => {
  for (let index = 0; index < 10; index += 1) {
    await Promise.resolve()
  }
  if (typeof window.requestAnimationFrame === 'function') {
    await new Promise<void>(resolve => window.requestAnimationFrame(() => resolve()))
  }
  await nextTick()
}

const platformFields = () => [
  { id: 'temp', name: 'Temperature', dataType: 'telemetry', type: 'number' },
  { id: 'setpoint', name: 'Setpoint', dataType: 'attribute', type: 'boolean' },
  { id: 'reboot', name: 'Reboot', dataType: 'command', type: 'json' },
  { id: 'device_alarm_count', name: 'Alarm count', dataType: 'telemetry', type: 'number' }
]

const widgetConfig = () => ({
  id: 'widget-dashboard',
  canvas: { mode: 'infinite', width: 100, height: 100 },
  nodes: [
    {
      id: 'switch-1',
      type: 'interaction/basic-switch',
      x: -20,
      y: 10,
      width: 100,
      height: 50,
      props: { value: '{{ ds.ds-1.data.temp }}' },
      data: [{ targetProp: 'value', expression: '{{ ds.ds-1.data.temp }}' }],
      events: [{ event: 'change', actions: [{ type: 'manualAction' }] }]
    },
    {
      id: 'history-chart',
      type: 'chart/line',
      x: 120,
      y: 50,
      width: 180,
      height: 100,
      props: {
        timeRangePreset: '7d',
        value: '{{ ds.ds-1.data.temp__history }}'
      },
      data: [
        {
          targetProp: 'series',
          expression: '{{ ds.ds-1.data.temp__history }}',
          historyConfig: { timeRange: '1h' }
        }
      ]
    },
    {
      id: 'camera-1',
      type: 'media/ezuikit-player',
      x: 20,
      y: 200,
      width: 200,
      height: 100,
      props: {
        ezopenUrl: 'ezopen://old',
        playbackParamsUrl: '/old',
        streamSuffix: 'old'
      },
      data: [{ targetProp: 'ezopenUrl', expression: '{{ ds.ds-1.data.old_url }}' }],
      events: [{ event: 'playbackRequest', actions: [] }]
    },
    {
      id: 'model-3d-1',
      type: 'media/3d-model',
      x: 260,
      y: 200,
      width: 240,
      height: 160,
      props: {
        modelUrl: '/uploads/rdi-panel.glb',
        autoRotate: true
      }
    }
  ],
  dataSources: [
    {
      id: 'ds-1',
      type: 'PLATFORM_FIELD',
      config: { requestedFields: ['temp'], bufferSize: 20 }
    }
  ]
})

const mountWidget = async (props: Record<string, unknown> = {}) => {
  const wrapper = mount(ThingsVisWidget, {
    props: {
      config: widgetConfig(),
      data: { temp: 25, setpoint: false },
      platformFields: platformFields(),
      platformDevices: [{ deviceId: 'dev-1', deviceName: 'Pump 1', fields: platformFields() }],
      height: '460px',
      mode: 'viewer',
      bufferSize: 30,
      ...props
    },
    attachTo: document.body
  })
  mountedWrappers.push(wrapper)
  await flushAsync()
  return wrapper
}

const latestClient = () => hoisted.clientInstances[hoisted.clientInstances.length - 1]

const markClientReady = async (client = latestClient()) => {
  client.ready = true
  client.handlers.get('ready')?.()
  await flushAsync()
}

const dispatchWindowMessage = async (
  data: Record<string, unknown>,
  source: { postMessage: (...args: unknown[]) => unknown } | null = latestClient()?.trustedSource ?? null,
  origin = 'https://studio.test'
) => {
  // MessageEventInit.source 只收 DOM 自带的来源类型；测试里的 mock iframe 来源
  // 结构兼容，这里做一次受控断言，避免给参数标 any。
  window.dispatchEvent(
    new MessageEvent('message', { data, source: source as unknown as MessageEventSource, origin })
  )
  await flushAsync()
  return latestClient()?.postMessageToGuest as ReturnType<typeof vi.fn>
}

describe('ThingsVisWidget.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
    hoisted.clientInstances = []
    hoisted.getThingsVisToken.mockResolvedValue('widget-token')
    hoisted.localStgGet.mockImplementation((key: string) => (key === 'token' ? 'platform-token' : undefined))
    hoisted.attributeDataPub.mockResolvedValue({ data: { accepted: true, channel: 'attribute' } })
    hoisted.commandDataPub.mockResolvedValue({ data: { accepted: true, channel: 'command' } })
    hoisted.telemetryDataPub.mockResolvedValue({ data: { accepted: true, channel: 'telemetry' } })
    hoisted.deviceAlarmStatus.mockResolvedValue({
      data: {
        total: 2,
        list: [
          { alarm_status: 'active', alarm_level: '2', alarm_name: 'Pressure warning', last_trigger_time: '10:30' },
          { alarm_status: 'inactive', alarm_level: '3', alarm_name: 'Old alarm', last_trigger_time: '09:00' }
        ]
      }
    })
    hoisted.telemetryDataHistoryList.mockResolvedValue({
      data: [
        { timestamp: '2026-06-27T01:00:00.000Z', value: '10' },
        { time: 1782514800000, y: 12 },
        { timestamp: 'bad', value: 'ignored' }
      ]
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    vi.unstubAllEnvs()
    document.body.innerHTML = ''
  })

  it('creates the embedded viewer client and loads normalized config before emitting ready', async () => {
    hoisted.getThingsVisToken.mockResolvedValue('widget-token&scope=a/b=1')
    const wrapper = await mountWidget()
    const client = latestClient()

    expect(client.options).toMatchObject({
      mode: 'widget',
      style: { height: '460px', minHeight: '400px' }
    })
    expect(client.options.url).toContain('https://studio.test/thingsvis/#/embed?mode=embedded')
    expect(client.options.url).toContain(`provider=${THINGSVIS_COMPAT_ALIAS}`)
    expect(client.options.url).toContain(`&token=${encodeURIComponent('widget-token&scope=a/b=1')}`)
    expect(client.options.url).toContain('&context=current-device')
    expect(client.options.url).toContain(`&thingsvisApiBaseUrl=${encodeURIComponent('https://thingsvis.test/api')}`)
    expect(client.options.url).toContain(`&platformApiBaseUrl=${encodeURIComponent('https://platform.test/api')}`)

    await markClientReady(client)

    expect(client.loadWidgetConfig).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('ready')).toHaveLength(1)
    const [normalizedConfig, schema, options] = client.loadWidgetConfig.mock.calls[0]
    expect(schema).toEqual(platformFields())
    expect(options).toMatchObject({
      platformBufferSize: 30,
      deviceId: 'dev-1',
      platformToken: 'platform-token',
      thingsvisApiBaseUrl: 'https://thingsvis.test/api',
      platformApiBaseUrl: 'https://platform.test/api',
      runtimeCapabilities: {
        version: 1,
        chartFontSizes: {
          supported: true,
          propsKey: 'fontSizes'
        },
        model3d: {
          supported: false,
          hostContractSupported: true,
          runtimeRenderingVerified: false,
          requiresExternalRuntime: true,
          acceptedExtensions: ['.glb', '.gltf', '.obj', '.fbx', '.stl'],
          maxUploadSizeMb: 1000
        }
      }
    })
    expect(options.platformDevices).toEqual([{ deviceId: 'dev-1', deviceName: 'Pump 1', fields: platformFields() }])
    expect(normalizedConfig.dataSources[0].config.deviceId).toBe('dev-1')
    expect(normalizedConfig.canvas).toMatchObject({
      scaleMode: 'fit-min',
      width: 616,
      height: 446
    })
    expect(findNormalizedNode(normalizedConfig, 'history-chart')?.props.fontSizes).toMatchObject({
      title: 16,
      legend: 12,
      axisLabel: 12,
      axisName: 12,
      seriesLabel: 12,
      tooltip: 12
    })
    expect(findNormalizedNode(normalizedConfig, 'model-3d-1')?.props).toMatchObject({
      modelUrl: '/uploads/rdi-panel.glb',
      acceptedExtensions: ['.glb', '.gltf', '.obj', '.fbx', '.stl'],
      maxUploadSizeMb: 1000,
      cameraControls: true,
      autoRotate: true,
      backgroundColor: 'transparent'
    })
    expect(findNormalizedNode(normalizedConfig, 'switch-1')?.events?.[0]?.actions).toEqual(
      expect.arrayContaining([
        { type: 'manualAction' },
        expect.objectContaining({
          type: 'callWrite',
          dataSourceId: 'ds-1',
          __thingsvisAutoWrite: 'field-binding',
          __thingsvisAutoWriteValueType: 'number'
        })
      ])
    )
    const camera = findNormalizedNode(normalizedConfig, 'camera-1')
    expect(camera?.props.spaceId).toBeUndefined()
    expect(camera?.props.busType).toBeUndefined()
    expect(camera?.props.ezopenUrl).toBeUndefined()
    expect(camera?.data?.map(item => item.targetProp)).toEqual(['accessToken', 'deviceSerial', 'channelNo'])
    expect(camera.events).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ event: 'playbackRequest' }),
        expect.objectContaining({ event: 'liveRequest' })
      ])
    )
    expect(client.updateSchema).toHaveBeenCalledWith(platformFields())
    expect(client.pushPlatformFieldData).toHaveBeenCalledWith({ temp: 25, setpoint: false }, 'dev-1')
  })

  it('keeps existing chart font size overrides while filling missing defaults', async () => {
    const config = widgetConfig()
    const chart = findNormalizedNode(config, 'history-chart')
    if (!chart) throw new Error('history-chart node missing from fixture')
    chart.props.fontSizes = { legend: 18, tooltip: 14 }

    await mountWidget({ config })
    const client = latestClient()
    await markClientReady(client)

    const [normalizedConfig] = client.loadWidgetConfig.mock.calls[0]
    expect(findNormalizedNode(normalizedConfig, 'history-chart')?.props.fontSizes).toMatchObject({
      title: 16,
      legend: 18,
      axisLabel: 12,
      axisName: 12,
      seriesLabel: 12,
      tooltip: 14
    })
  })

  it('keeps saved 3D model options while filling upload and viewer defaults', async () => {
    const config = widgetConfig()
    const model = findNormalizedNode(config, 'model-3d-1')
    if (!model) throw new Error('model-3d-1 node missing from fixture')
    model.props.acceptedExtensions = ['.glb']
    model.props.maxUploadSizeMb = 200
    model.props.cameraControls = false
    model.props.backgroundColor = '#101010'

    await mountWidget({ config })
    const client = latestClient()
    await markClientReady(client)

    const [normalizedConfig] = client.loadWidgetConfig.mock.calls[0]
    expect(findNormalizedNode(normalizedConfig, 'model-3d-1')?.props).toMatchObject({
      modelUrl: '/uploads/rdi-panel.glb',
      acceptedExtensions: ['.glb'],
      maxUploadSizeMb: 200,
      cameraControls: false,
      autoRotate: true,
      backgroundColor: '#101010'
    })
  })

  it('keeps the host-save embed URL contract when the URL token is unavailable', async () => {
    hoisted.getThingsVisToken.mockResolvedValue('')

    await mountWidget({ deviceId: '', platformDevices: [], data: undefined })
    const client = latestClient()

    expect(client.options.url).toContain('https://studio.test/thingsvis/#/embed?mode=embedded')
    expect(client.options.url).toContain(`provider=${THINGSVIS_COMPAT_ALIAS}`)
    expect(client.options.url).toContain('&saveTarget=host')
    expect(client.options.url).not.toContain('&token=')
    expect(client.options.url).toContain('&context=dashboard')
    expect(client.options.url).toContain(`&thingsvisApiBaseUrl=${encodeURIComponent('https://thingsvis.test/api')}`)
    expect(client.options.url).toContain(`&platformApiBaseUrl=${encodeURIComponent('https://platform.test/api')}`)
  })

  it('does not create a client after unmounting while the URL token is still loading', async () => {
    let resolveToken: (token: string) => void = () => {}
    hoisted.getThingsVisToken.mockReturnValue(
      new Promise<string>(resolve => {
        resolveToken = resolve
      })
    )

    const wrapper = await mountWidget()
    wrapper.unmount()
    resolveToken('late-widget-token')
    await flushAsync()

    expect(hoisted.clientInstances).toHaveLength(0)
  })

  it('uses explicit EZUIKit playback defaults from deployment env', async () => {
    vi.stubEnv('VITE_EZUIKIT_DEFAULT_SPACE_ID', 'tenant-space')
    vi.stubEnv('VITE_EZUIKIT_DEFAULT_BUS_TYPE', 'tenant-bus')
    await mountWidget()
    const client = latestClient()

    await markClientReady(client)

    const [normalizedConfig] = client.loadWidgetConfig.mock.calls[0]
    const camera = findNormalizedNode(normalizedConfig, 'camera-1')
    expect(camera?.props).toMatchObject({ spaceId: 'tenant-space', busType: 'tenant-bus' })
  })

  it('keeps compatibility EZUIKit playback defaults behind an explicit env switch', async () => {
    vi.stubEnv('VITE_ENABLE_EZUIKIT_COMPAT_DEFAULTS', 'Y')
    await mountWidget()
    const client = latestClient()

    await markClientReady(client)

    const [normalizedConfig] = client.loadWidgetConfig.mock.calls[0]
    const camera = findNormalizedNode(normalizedConfig, 'camera-1')
    expect(camera?.props).toMatchObject({ spaceId: '361254', busType: '7' })
  })

  it('does not overwrite saved EZUIKit playback props', async () => {
    vi.stubEnv('VITE_EZUIKIT_DEFAULT_SPACE_ID', 'tenant-space')
    vi.stubEnv('VITE_EZUIKIT_DEFAULT_BUS_TYPE', 'tenant-bus')
    const config = widgetConfig()
    const camera = findNormalizedNode(config, 'camera-1')
    if (!camera) throw new Error('camera-1 node missing from fixture')
    camera.props.spaceId = 'saved-space'
    camera.props.busType = 'saved-bus'
    await mountWidget({ config })
    const client = latestClient()

    await markClientReady(client)

    const [normalizedConfig] = client.loadWidgetConfig.mock.calls[0]
    const normalizedCamera = findNormalizedNode(normalizedConfig, 'camera-1')
    expect(normalizedCamera?.props).toMatchObject({ spaceId: 'saved-space', busType: 'saved-bus' })
  })

  it('keeps configured platform data source device ids when viewer context has a preview device', async () => {
    await mountWidget({
      config: {
        ...widgetConfig(),
        dataSources: [
          {
            id: 'ds-1',
            type: 'PLATFORM_FIELD',
            config: { requestedFields: ['temp'], bufferSize: 20, deviceId: 'original-device' }
          }
        ]
      }
    })
    const client = latestClient()

    await markClientReady(client)

    const [normalizedConfig] = client.loadWidgetConfig.mock.calls[0]
    expect(normalizedConfig.dataSources[0].config.deviceId).toBe('original-device')
  })

  it('uses device-template context and synthetic platform devices when editing a thing model', async () => {
    await mountWidget({
      mode: 'editor',
      deviceId: '__template__',
      platformDevices: []
    })
    const client = latestClient()

    expect(client.options.url).toContain('#/editor?mode=embedded')
    expect(client.options.url).toContain('&context=device-template')

    await markClientReady(client)

    expect(client.loadWidgetConfig.mock.calls[0][2].platformDevices).toEqual([
      expect.objectContaining({
        deviceId: '__template__',
        deviceName: 'thing-model-fields',
        groupId: '__template__',
        groupName: 'thing-model-fields',
        fields: platformFields()
      })
    ])
  })

  it('routes platform writes to telemetry, attributes, and commands with normalized values and result callbacks', async () => {
    await mountWidget({ deviceId: 'dev-1' })
    const client = latestClient()

    await dispatchWindowMessage({
      type: 'tv:platform-write',
      requestId: 'write-telemetry',
      payload: { dataSourceId: 'ds-1', data: '1' }
    })
    await dispatchWindowMessage({
      type: 'tv:platform-write',
      requestId: 'write-attribute',
      payload: { dataSourceId: 'ds-1', data: { setpoint: 'true' } }
    })
    await dispatchWindowMessage({
      type: 'tv:platform-write',
      requestId: 'write-command',
      payload: { dataSourceId: 'ds-1', data: { reboot: { delay: 5 } } }
    })

    expect(hoisted.telemetryDataPub).toHaveBeenCalledWith({ device_id: 'dev-1', value: JSON.stringify({ temp: 1 }) })
    expect(hoisted.attributeDataPub).toHaveBeenCalledWith({
      device_id: 'dev-1',
      value: JSON.stringify({ setpoint: true })
    })
    expect(hoisted.commandDataPub).toHaveBeenCalledWith({
      device_id: 'dev-1',
      identify: 'reboot',
      value: JSON.stringify({ delay: 5 })
    })
    expect(client.postMessageToGuest).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'tv:platform-write-result', requestId: 'write-telemetry', success: true })
    )
    expect(client.postMessageToGuest).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'tv:platform-write-result', requestId: 'write-attribute', success: true })
    )
    expect(client.postMessageToGuest).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'tv:platform-write-result', requestId: 'write-command', success: true })
    )
  })

  it('reports write validation and API failures back to the embedded source window', async () => {
    hoisted.telemetryDataPub.mockRejectedValueOnce(new Error('publish failed'))
    await mountWidget({ deviceId: '', platformDevices: [] })
    const client = latestClient()

    await dispatchWindowMessage({
      type: 'tv:platform-write',
      requestId: 'missing-data',
      payload: { data: { temp: 1 } }
    })
    await dispatchWindowMessage({
      type: 'tv:platform-write',
      requestId: 'missing-device',
      payload: { dataSourceId: 'ds-1', data: { temp: 1 } }
    })
    expect(client.postMessageToGuest).toHaveBeenCalledWith(
      expect.objectContaining({ requestId: 'missing-data', success: false, error: 'Missing dataSourceId or data' })
    )
    expect(client.postMessageToGuest).toHaveBeenCalledWith(
      expect.objectContaining({ requestId: 'missing-device', success: false, error: 'Missing deviceId' })
    )

    await mountWidget({ deviceId: 'dev-1' })
    await dispatchWindowMessage({
      type: 'tv:platform-write',
      requestId: 'wrong-device',
      payload: { dataSourceId: 'ds-1', deviceId: 'other-device', data: { temp: 2 } }
    })
    await dispatchWindowMessage({
      type: 'tv:platform-write',
      requestId: 'failed-publish',
      payload: { dataSourceId: 'ds-1', deviceId: 'dev-1', data: { temp: 2 } }
    })

    const secondClient = latestClient()
    expect(secondClient.postMessageToGuest).toHaveBeenCalledWith(
      expect.objectContaining({ requestId: 'wrong-device', success: false, error: 'Device mismatch' })
    )
    expect(secondClient.postMessageToGuest).toHaveBeenCalledWith(
      expect.objectContaining({ requestId: 'failed-publish', success: false, error: 'publish failed' })
    )
    expect(hoisted.telemetryDataPub).toHaveBeenCalledTimes(1)
    expect(hoisted.telemetryDataPub).toHaveBeenCalledWith({ device_id: 'dev-1', value: JSON.stringify({ temp: 2 }) })
  })

  it('ignores platform write and field-data messages from wrong iframe source or origin', async () => {
    await mountWidget({ deviceId: 'dev-1' })
    const client = latestClient()
    const wrongSource = { postMessage: vi.fn() }

    client.postMessageToGuest.mockClear()
    client.pushPlatformFieldData.mockClear()
    hoisted.telemetryDataPub.mockClear()
    hoisted.attributeDataPub.mockClear()
    hoisted.commandDataPub.mockClear()
    hoisted.deviceAlarmStatus.mockClear()
    hoisted.telemetryDataHistoryList.mockClear()

    await dispatchWindowMessage(
      {
        type: 'tv:platform-write',
        requestId: 'wrong-origin-write',
        payload: { dataSourceId: 'ds-1', data: { temp: 30 } }
      },
      client.trustedSource,
      'https://evil.test'
    )
    await dispatchWindowMessage(
      {
        type: 'tv:platform-write',
        requestId: 'wrong-source-write',
        payload: { dataSourceId: 'ds-1', data: { temp: 31 } }
      },
      wrongSource
    )
    await dispatchWindowMessage(
      {
        type: 'thingsvis:requestFieldData',
        payload: { dataSourceId: 'ds-1', deviceId: 'dev-1', fieldIds: ['temp', 'device_alarm_count'] }
      },
      client.trustedSource,
      'https://evil.test'
    )
    await dispatchWindowMessage(
      {
        type: 'thingsvis:requestFieldData',
        payload: { dataSourceId: 'ds-1', deviceId: 'dev-1', fieldIds: ['temp'] }
      },
      wrongSource
    )

    expect(hoisted.telemetryDataPub).toHaveBeenCalledTimes(0)
    expect(hoisted.attributeDataPub).toHaveBeenCalledTimes(0)
    expect(hoisted.commandDataPub).toHaveBeenCalledTimes(0)
    expect(hoisted.deviceAlarmStatus).toHaveBeenCalledTimes(0)
    expect(hoisted.telemetryDataHistoryList).toHaveBeenCalledTimes(0)
    expect(client.pushPlatformFieldData).toHaveBeenCalledTimes(0)
    expect(client.postMessageToGuest).toHaveBeenCalledTimes(0)
  })

  it('answers field-data requests with current data, alarm fields, explicit history, and buffered history prefill', async () => {
    await mountWidget({
      data: { temp: 25, flow: 8 },
      deviceId: 'dev-1'
    })
    const client = latestClient()
    await markClientReady(client)

    await dispatchWindowMessage({
      type: 'thingsvis:requestFieldData',
      payload: {
        dataSourceId: 'ds-1',
        deviceId: 'dev-1',
        fieldIds: ['temp', 'temp__history', 'device_alarm_count', 'device_alarm_highest_level'],
        historyConfig: { aggFunction: 'AVG', aggWindow: '1m' }
      }
    })

    expect(hoisted.deviceAlarmStatus).toHaveBeenCalledWith({ device_id: 'dev-1', page: 1, page_size: 20 })
    expect(hoisted.telemetryDataHistoryList).toHaveBeenCalledWith(
      {
        device_id: 'dev-1',
        key: 'temp',
        time_range: 'last_7d',
        aggregate_window: '1m',
        aggregate_function: 'AVG'
      },
      { silentError: true }
    )
    expect(client.pushPlatformFieldHistory).toHaveBeenCalledWith(
      'temp',
      [
        { timestamp: '2026-06-27T01:00:00.000Z', ts: Date.parse('2026-06-27T01:00:00.000Z'), value: 10 },
        { timestamp: 1782514800000, ts: 1782514800000, value: 12 }
      ],
      'dev-1'
    )
    expect(client.pushPlatformFieldData).toHaveBeenLastCalledWith(
      {
        temp: 25,
        temp__history: [
          { timestamp: '2026-06-27T01:00:00.000Z', ts: Date.parse('2026-06-27T01:00:00.000Z'), value: 10 },
          { timestamp: 1782514800000, ts: 1782514800000, value: 12 }
        ],
        device_alarm_count: 2,
        device_alarm_highest_level: 'warning'
      },
      'dev-1'
    )
  })

  it('selects highest active alarm severity instead of the first active alarm row', async () => {
    hoisted.deviceAlarmStatus.mockResolvedValueOnce({
      data: {
        total: 2,
        list: [
          { alarm_status: 'active', alarm_level: 'warning', alarm_name: 'Recent warning', last_trigger_time: '10:30' },
          {
            alarm_status: 'active',
            alarm_level: 'critical',
            alarm_name: 'Critical pressure',
            last_trigger_time: '10:00'
          }
        ]
      }
    })
    await mountWidget({
      data: {},
      deviceId: 'dev-1'
    })
    const client = latestClient()
    await markClientReady(client)

    await dispatchWindowMessage({
      type: 'thingsvis:requestFieldData',
      payload: {
        dataSourceId: 'ds-1',
        deviceId: 'dev-1',
        fieldIds: ['device_alarm_highest_level', 'latest_device_alarm_level']
      }
    })

    expect(client.pushPlatformFieldData).toHaveBeenLastCalledWith(
      {
        device_alarm_highest_level: 'critical',
        latest_device_alarm_level: 'warning'
      },
      'dev-1'
    )
  })

  it('ignores field-data requests for a different preview device and skips template-device history/alarm API calls', async () => {
    await mountWidget({ deviceId: '__template__', platformDevices: [] })
    const client = latestClient()
    await markClientReady(client)

    await dispatchWindowMessage({
      type: 'thingsvis:requestFieldData',
      payload: { dataSourceId: 'ds-1', deviceId: 'other-device', fieldIds: ['temp'] }
    })
    await dispatchWindowMessage({
      type: 'thingsvis:requestFieldData',
      payload: { dataSourceId: 'ds-1', deviceId: '__template__', fieldIds: ['temp__history', 'device_alarm_count'] }
    })

    expect(hoisted.deviceAlarmStatus).toHaveBeenCalledTimes(0)
    expect(hoisted.telemetryDataHistoryList).toHaveBeenCalledTimes(0)
    expect(client.pushPlatformFieldData).not.toHaveBeenLastCalledWith(
      expect.objectContaining({ device_alarm_count: 2 }),
      '__template__'
    )
  })

  it('reacts to config, data, and schema prop changes after the client is ready', async () => {
    const wrapper = await mountWidget()
    const client = latestClient()
    await markClientReady(client)
    client.loadWidgetConfig.mockClear()
    client.pushPlatformFieldData.mockClear()
    client.updateSchema.mockClear()

    await wrapper.setProps({
      config: {
        ...widgetConfig(),
        nodes: [{ id: 'simple-node', type: 'text', props: { text: 'changed' } }]
      }
    })
    await wrapper.setProps({ data: { temp: 30 } })
    await wrapper.setProps({ platformFields: [{ id: 'flow', dataType: 'telemetry', type: 'number' }] })
    await flushAsync()

    expect(client.loadWidgetConfig).toHaveBeenCalledTimes(1)
    expect(client.pushPlatformFieldData).toHaveBeenCalledWith({ temp: 30 }, 'dev-1')
    expect(client.updateSchema).toHaveBeenCalledWith([{ id: 'flow', dataType: 'telemetry', type: 'number' }])
  })

  it('emits save/change from SDK save events, exposes triggerSave and pushPlatformData, and destroys the client on unmount', async () => {
    const wrapper = await mountWidget()
    const client = latestClient()
    await markClientReady(client)

    client.handlers.get('tv:save-config')?.({ config: { id: 'saved-config', nodes: [] } })
    await flushAsync()
    expect(wrapper.emitted('save')).toEqual([[{ id: 'saved-config', nodes: [] }]])
    expect(wrapper.emitted('change')).toEqual([[{ id: 'saved-config', nodes: [] }]])

    wrapper.vm.triggerSave()
    wrapper.vm.pushPlatformData({ temp: 31 }, 'dev-1')

    expect(client.requestSave).toHaveBeenCalledTimes(1)
    expect(client.pushPlatformFieldData).toHaveBeenLastCalledWith({ temp: 31 }, 'dev-1')

    wrapper.unmount()
    expect(client.destroy).toHaveBeenCalledTimes(1)
  })
})
