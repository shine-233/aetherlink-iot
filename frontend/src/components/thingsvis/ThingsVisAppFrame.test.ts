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
  routerResolve: vi.fn(() => ({ href: '/visualization/thingsvis-preview?id=dashboard-1' })),
  getThingsVisToken: vi.fn(),
  clearThingsVisToken: vi.fn(),
  getThingsVisDashboard: vi.fn(),
  updateThingsVisDashboard: vi.fn(),
  publishThingsVisDashboard: vi.fn(),
  deviceGroupTree: vi.fn(),
  deviceList: vi.fn(),
  deviceListByGroup: vi.fn(),
  deviceAlarmStatus: vi.fn(),
  deviceDictProtocolServiceFirstLevel: vi.fn(),
  getDeviceConfigList: vi.fn(),
  telemetryDataCurrent: vi.fn(),
  getAttributeDataSet: vi.fn(),
  telemetryDataPub: vi.fn(),
  attributeDataPub: vi.fn(),
  commandDataPub: vi.fn(),
  telemetryApi: vi.fn(),
  attributesApi: vi.fn(),
  commandsApi: vi.fn(),
  eventsApi: vi.fn(),
  getTemplat: vi.fn(),
  rdiDeviceConfig: vi.fn(),
  localStgGet: vi.fn(),
  extractPlatformFields: vi.fn(),
  loggerWarn: vi.fn(),
  loggerError: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    resolve: hoisted.routerResolve
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/thingsvis', () => ({
  clearThingsVisToken: hoisted.clearThingsVisToken,
  getThingsVisToken: hoisted.getThingsVisToken
}))

vi.mock('@/service/api/device', () => ({
  deviceGroupTree: hoisted.deviceGroupTree,
  deviceList: hoisted.deviceList,
  deviceListByGroup: hoisted.deviceListByGroup,
  deviceAlarmStatus: hoisted.deviceAlarmStatus,
  deviceDictProtocolServiceFirstLevel: hoisted.deviceDictProtocolServiceFirstLevel,
  getDeviceConfigList: hoisted.getDeviceConfigList,
  telemetryDataCurrent: hoisted.telemetryDataCurrent,
  getAttributeDataSet: hoisted.getAttributeDataSet,
  telemetryDataPub: hoisted.telemetryDataPub,
  attributeDataPub: hoisted.attributeDataPub,
  commandDataPub: hoisted.commandDataPub
}))

vi.mock('@/service/api', () => ({
  telemetryApi: hoisted.telemetryApi,
  attributesApi: hoisted.attributesApi,
  commandsApi: hoisted.commandsApi,
  eventsApi: hoisted.eventsApi
}))

vi.mock('@/service/api/system-data', () => ({
  getTemplat: hoisted.getTemplat
}))

vi.mock('@/service/api/thingsvis', () => ({
  getThingsVisDashboard: hoisted.getThingsVisDashboard,
  publishThingsVisDashboard: hoisted.publishThingsVisDashboard,
  updateThingsVisDashboard: hoisted.updateThingsVisDashboard
}))

vi.mock('@/service/api/rdi', () => ({
  rdiDeviceConfig: hoisted.rdiDeviceConfig
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

vi.mock('@/utils/thingsvis/platform-fields', () => ({
  extractPlatformFields: hoisted.extractPlatformFields
}))

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: hoisted.localStgGet
  }
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    warn: hoisted.loggerWarn,
    error: hoisted.loggerError
  })
}))

vi.mock('@/utils/common/tool', () => ({
  getWebsocketServerUrl: () => 'wss://platform.test'
}))

import ThingsVisAppFrame from './ThingsVisAppFrame.vue'
import { THINGSVIS_COMPAT_ALIAS } from '@/utils/thingsvis/constants'

const TARGET_ORIGIN = 'https://studio.test'

type MountedFrame = {
  wrapper: VueWrapper
  postMessage: ReturnType<typeof vi.fn>
}

class MockWebSocket {
  static OPEN = 1
  static instances: MockWebSocket[] = []

  readyState = MockWebSocket.OPEN
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null
  send = vi.fn()
  close = vi.fn(() => {
    this.readyState = 3
  })

  constructor(public url: string) {
    MockWebSocket.instances.push(this)
  }
}

const mountedWrappers: VueWrapper[] = []
let currentIframeWindow: MessageEventSource | null = null

const flushAsync = async () => {
  for (let index = 0; index < 10; index += 1) {
    await Promise.resolve()
  }
  await nextTick()
}

const defaultSchema = () => ({
  id: 'dashboard-1',
  name: 'RDI dashboard',
  thumbnail: 'thumb.png',
  canvasConfig: { width: 1920, height: 1080, background: '#fff' },
  nodes: [
    {
      id: 'node-1',
      props: {
        value: '{{ ds.__platform_dev-1__.data.temp }}'
      }
    }
  ],
  dataSources: [
    {
      id: '__platform_dev-1__',
      type: 'PLATFORM_FIELD',
      config: { deviceId: 'dev-1', requestedFields: ['temp'], bufferSize: 250 }
    },
    {
      id: '__platform_unused__',
      type: 'PLATFORM_FIELD',
      config: { deviceId: 'unused', requestedFields: ['temp'] }
    },
    {
      id: 'manual-api',
      type: 'HTTP',
      config: { url: '/api/manual' }
    }
  ],
  variables: [{ name: 'site', value: 'north' }]
})

const dashboardResponse = () => ({
  data: {
    id: 'dashboard-1',
    name: 'Fetched dashboard',
    thumbnail: 'fetched-thumb.png',
    canvasConfig: { background: '#123456' },
    nodes: [{ id: 'node-1', props: { value: '{{ ds.__platform_dev-1__.data.temp }}' } }],
    dataSources: [
      {
        id: '__platform_dev-1__',
        type: 'PLATFORM_FIELD',
        config: { deviceId: 'dev-1', requestedFields: ['temp'] }
      }
    ],
    variables: []
  },
  error: null
})

const mountFrame = async (props: Record<string, unknown> = {}): Promise<MountedFrame> => {
  const wrapper = mount(ThingsVisAppFrame, {
    props: {
      id: 'dashboard-1',
      mode: 'editor',
      ...props
    }
  })
  mountedWrappers.push(wrapper)
  await flushAsync()

  const iframe = wrapper.get('iframe').element as HTMLIFrameElement
  const postMessage = vi.fn()
  Object.defineProperty(iframe, 'contentWindow', {
    configurable: true,
    value: { postMessage }
  })
  currentIframeWindow = iframe.contentWindow

  return { wrapper, postMessage }
}

const dispatchFrameMessage = async (
  data: Record<string, unknown>,
  origin = TARGET_ORIGIN,
  source = currentIframeWindow
) => {
  window.dispatchEvent(new MessageEvent('message', { data, origin, source }))
  await flushAsync()
}

const postedPayloads = (postMessage: ReturnType<typeof vi.fn>, type: string) => {
  return postMessage.mock.calls.map(call => call[0]).filter(message => message?.type === type)
}

describe('ThingsVisAppFrame.vue', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    document.body.innerHTML = ''
    MockWebSocket.instances = []
    vi.stubGlobal('WebSocket', MockWebSocket)
    vi.spyOn(window, 'open').mockImplementation(() => null)

    hoisted.getThingsVisToken.mockResolvedValue('thingsvis-token')
    hoisted.getThingsVisDashboard.mockResolvedValue(dashboardResponse())
    hoisted.updateThingsVisDashboard.mockResolvedValue({ data: { ok: true }, error: null })
    hoisted.deviceGroupTree.mockResolvedValue({
      data: [
        {
          id: 'group-a',
          name: 'Workshop A',
          deviceCount: 1,
          children: [{ id: 'group-child', name: 'Child group', parentId: 'group-a' }]
        }
      ]
    })
    hoisted.deviceListByGroup.mockResolvedValue({
      data: {
        list: [
          {
            device_id: 'dev-1',
            device_name: 'Pump 1',
            group_id: 'group-a',
            device_template_id: 'tpl-1',
            is_online: 1
          }
        ]
      }
    })
    hoisted.deviceList.mockResolvedValue({
      data: {
        list: [
          {
            device_id: 'dev-1',
            device_name: 'Pump 1',
            group_id: 'group-a',
            device_template_id: 'tpl-1',
            is_online: 1
          }
        ],
        total: 1
      }
    })
    hoisted.deviceAlarmStatus.mockResolvedValue({
      data: {
        total: 1,
        list: [{ alarm_status: 'active', alarm_level: 'high', alarm_name: 'High pressure', last_trigger_time: '10:00' }]
      }
    })
    hoisted.deviceDictProtocolServiceFirstLevel.mockResolvedValue({
      data: {
        protocol: [{ service_identifier: 'mqtt', name: 'MQTT' }],
        service: [{ service_identifier: 'rdi', name: 'RDI' }]
      }
    })
    hoisted.getDeviceConfigList.mockResolvedValue({
      data: {
        list: [{ id: 'config-1', name: 'Config 1', device_template_id: 'tpl-1' }]
      }
    })
    hoisted.telemetryDataCurrent.mockResolvedValue({
      data: [
        { key: 'temp', value: 31 },
        { key: 'flow', value: 12 }
      ]
    })
    hoisted.getAttributeDataSet.mockResolvedValue({
      data: [{ key: 'setpoint', value: 40 }]
    })
    hoisted.rdiDeviceConfig.mockResolvedValue({
      data: { device: { firmware_version: '1.2.3', pid_number: 'PID-9', shared_status: 1 } }
    })
    hoisted.telemetryDataPub.mockResolvedValue({ data: { accepted: true, channel: 'telemetry' } })
    hoisted.attributeDataPub.mockResolvedValue({ data: { accepted: true, channel: 'attribute' } })
    hoisted.commandDataPub.mockResolvedValue({ data: { accepted: true, channel: 'command' } })
    hoisted.telemetryApi.mockResolvedValue({ data: { list: [{ key: 'temp' }] } })
    hoisted.attributesApi.mockResolvedValue({ data: { list: [{ key: 'setpoint' }] } })
    hoisted.commandsApi.mockResolvedValue({ data: { list: [{ key: 'reboot' }] } })
    hoisted.eventsApi.mockResolvedValue({ data: { list: [] } })
    hoisted.getTemplat.mockResolvedValue({
      data: {
        web_chart_config: JSON.stringify({
          nodes: [{ id: 'preset-node', type: 'value', props: { title: 'Value preset' } }]
        })
      }
    })
    hoisted.extractPlatformFields.mockReturnValue([
      { id: 'temp', name: 'Temperature', dataType: 'telemetry' },
      { id: 'setpoint', name: 'Setpoint', dataType: 'attribute' },
      { id: 'reboot', name: 'Reboot', dataType: 'command' }
    ])
    hoisted.localStgGet.mockImplementation((key: string) => {
      if (key === 'token') return 'platform-token'
      if (key === 'lang') return 'zh-CN'
      return undefined
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    currentIframeWindow = null
    vi.unstubAllGlobals()
    vi.useRealTimers()
    document.body.innerHTML = ''
  })

  it('initializes the editor iframe from a complete schema and sanitizes generated data sources', async () => {
    hoisted.getThingsVisToken.mockResolvedValue('thingsvis-token&scope=a/b=1')
    const { wrapper, postMessage } = await mountFrame({ schema: defaultSchema() })

    const iframe = wrapper.get('iframe')
    expect(iframe.attributes('src')).toContain('https://studio.test/thingsvis/#/editor?mode=embedded')
    expect(iframe.attributes('src')).toContain(`provider=${THINGSVIS_COMPAT_ALIAS}`)
    expect(iframe.attributes('src')).toContain(`&token=${encodeURIComponent('thingsvis-token&scope=a/b=1')}`)

    await dispatchFrameMessage({ type: 'tv:ready' })
    await vi.advanceTimersByTimeAsync(150)
    await flushAsync()

    expect(hoisted.getThingsVisDashboard).toHaveBeenCalledTimes(0)
    const [initMessage] = postedPayloads(postMessage, 'tv:init')
    expect(initMessage.payload.config).toMatchObject({
      mode: 'app',
      saveTarget: 'host',
      token: 'thingsvis-token&scope=a/b=1',
      platformToken: 'platform-token'
    })
    expect(initMessage.payload.platformBufferSize).toBe(250)
    expect(initMessage.payload.data.meta).toMatchObject({ id: 'dashboard-1', name: 'RDI dashboard' })
    expect(initMessage.payload.data.dataSources.map((item: any) => item.id)).toEqual([
      '__platform_dev-1__',
      'manual-api'
    ])
  })

  it('retries dashboard preload after a 401 and clears the cached ThingsVis token', async () => {
    hoisted.getThingsVisDashboard
      .mockResolvedValueOnce({ data: null, error: { status: 401, message: 'expired' } })
      .mockResolvedValueOnce(dashboardResponse())
    const { postMessage } = await mountFrame()

    await dispatchFrameMessage({ type: 'READY' })
    await vi.advanceTimersByTimeAsync(150)
    await flushAsync()

    expect(hoisted.clearThingsVisToken).toHaveBeenCalledTimes(1)
    expect(hoisted.getThingsVisDashboard).toHaveBeenCalledTimes(2)
    expect(postedPayloads(postMessage, 'tv:init')).toHaveLength(1)
  })

  it('replays init when the guest explicitly requests tv:request-init after a successful bootstrap', async () => {
    const { postMessage } = await mountFrame()

    await dispatchFrameMessage({ type: 'tv:ready' })
    await vi.advanceTimersByTimeAsync(150)
    await flushAsync()
    expect(postedPayloads(postMessage, 'tv:init')).toHaveLength(1)

    await dispatchFrameMessage({ type: 'tv:request-init' })
    await vi.advanceTimersByTimeAsync(150)
    await flushAsync()

    expect(postedPayloads(postMessage, 'tv:init')).toHaveLength(2)
  })

  it('saves through the host bridge, normalizes canvas background, strips editor-only data source flags, and retries 401', async () => {
    hoisted.updateThingsVisDashboard
      .mockResolvedValueOnce({ data: null, error: { status: 401, message: 'expired' } })
      .mockResolvedValueOnce({ data: { ok: true }, error: null })
    const { wrapper } = await mountFrame()

    await dispatchFrameMessage({
      type: 'tv:save',
      payload: {
        config: {
          meta: { name: 'Saved dashboard', thumbnail: 'saved-thumb.png' },
          canvas: { background: '#abcdef' },
          nodes: [{ id: 'node-1', props: { value: '{{ ds.__platform_dev-1__.data.temp }}' } }],
          dataSources: [
            { id: '__platform_dev-1__', type: 'PLATFORM_FIELD', __editorAutoManual: true, mode: 'manual' },
            { id: '__platform_unused__', type: 'PLATFORM_FIELD' },
            { id: 'external-api', type: 'HTTP' }
          ],
          variables: [{ name: 'site', value: 'north' }]
        }
      }
    })
    await flushAsync()

    expect(hoisted.clearThingsVisToken).toHaveBeenCalledTimes(1)
    expect(hoisted.updateThingsVisDashboard).toHaveBeenCalledTimes(2)
    expect(hoisted.updateThingsVisDashboard).toHaveBeenLastCalledWith(
      'dashboard-1',
      expect.objectContaining({
        name: 'Saved dashboard',
        thumbnail: 'saved-thumb.png',
        canvasConfig: { background: { color: '#abcdef' } },
        dataSources: [
          { id: '__platform_dev-1__', type: 'PLATFORM_FIELD' },
          { id: 'external-api', type: 'HTTP' }
        ]
      })
    )
    expect(wrapper.emitted('hostSaveSuccess')).toEqual([[{ id: 'dashboard-1', name: 'Saved dashboard' }]])
  })

  it('accepts the legacy host-save payload shape that uses root canvas/dataBindings fields', async () => {
    hoisted.updateThingsVisDashboard.mockResolvedValueOnce({ data: { ok: true }, error: null })
    const { wrapper } = await mountFrame()

    await dispatchFrameMessage({
      type: 'tv:save',
      payload: {
        meta: { name: 'Legacy payload dashboard' },
        thumbnail: 'legacy-thumb.png',
        canvas: { background: '#112233' },
        nodes: [{ id: 'node-legacy', props: { value: '{{ ds.__platform_dev-1__.data.temp }}' } }],
        dataBindings: [
          { id: '__platform_dev-1__', type: 'PLATFORM_FIELD', __editorAutoManual: true, mode: 'manual' },
          { id: '__platform_unused__', type: 'PLATFORM_FIELD' },
          { id: 'legacy-http', type: 'HTTP' }
        ]
      }
    })
    await flushAsync()

    expect(hoisted.updateThingsVisDashboard).toHaveBeenCalledWith(
      'dashboard-1',
      expect.objectContaining({
        name: 'Legacy payload dashboard',
        thumbnail: 'legacy-thumb.png',
        canvasConfig: { background: { color: '#112233' } },
        dataSources: [
          { id: '__platform_dev-1__', type: 'PLATFORM_FIELD' },
          { id: 'legacy-http', type: 'HTTP' }
        ]
      })
    )
    expect(wrapper.emitted('hostSaveSuccess')).toEqual([[{ id: 'dashboard-1', name: 'Legacy payload dashboard' }]])
  })

  it('does not emit hostSaveSuccess when the host save retry still fails', async () => {
    hoisted.updateThingsVisDashboard
      .mockResolvedValueOnce({ data: null, error: { status: 401, message: 'expired' } })
      .mockResolvedValueOnce({ data: null, error: { status: 500, message: 'still failed' } })
    const { wrapper } = await mountFrame()

    await dispatchFrameMessage({
      type: 'tv:save',
      payload: {
        config: {
          meta: { name: 'Failed dashboard' },
          nodes: [],
          dataSources: []
        }
      }
    })
    await flushAsync()

    expect(hoisted.clearThingsVisToken).toHaveBeenCalledTimes(1)
    expect(hoisted.updateThingsVisDashboard).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('hostSaveSuccess')).toBeUndefined()
  })

  it('serves device groups, filter options, grouped devices, search results, and thing-model fields to ThingsVis', async () => {
    const { postMessage } = await mountFrame()

    await dispatchFrameMessage({ type: 'thingsvis:requestDeviceGroups', payload: {} })
    await dispatchFrameMessage({ type: 'thingsvis:requestDeviceFilterOptions', payload: { reqId: 'filters-1' } })
    await dispatchFrameMessage({ type: 'thingsvis:requestDevicesByGroup', payload: { groupId: 'group-a' } })
    await dispatchFrameMessage({
      type: 'thingsvis:searchDevicesPaged',
      payload: { reqId: 'search-1', groupId: 'group-a', keyword: 'pump', page: 2, pageSize: 5, isOnline: '1' }
    })
    await dispatchFrameMessage({
      type: 'thingsvis:requestDeviceFields',
      payload: { deviceId: 'dev-1', templateId: 'tpl-1' }
    })
    await flushAsync()

    expect(postedPayloads(postMessage, 'tv:device-groups')[0].payload.groups).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ groupId: '__all__' }),
        expect.objectContaining({ groupId: 'group-a', groupName: 'Workshop A' })
      ])
    )
    expect(postedPayloads(postMessage, 'tv:device-filter-options')[0].payload).toMatchObject({
      reqId: 'filters-1',
      deviceConfigs: [{ value: 'config-1', label: 'Config 1' }]
    })
    expect(postedPayloads(postMessage, 'tv:devices-by-group')[0].payload.devices[0]).toMatchObject({
      deviceId: 'dev-1',
      deviceName: 'Pump 1',
      templateId: 'tpl-1',
      fields: expect.arrayContaining([expect.objectContaining({ id: 'temp' })]),
      presets: expect.arrayContaining([expect.objectContaining({ id: 'tpl-1-web-node-preset-node' })])
    })
    expect(hoisted.deviceList).toHaveBeenCalledWith(
      expect.objectContaining({ group_id: 'group-a', search: 'pump', page: 2, page_size: 5, is_online: 1 })
    )
    expect(postedPayloads(postMessage, 'tv:search-devices-paged-result')[0].payload).toMatchObject({
      reqId: 'search-1',
      total: 1,
      page: 2,
      pageSize: 5
    })
    expect(postedPayloads(postMessage, 'tv:device-fields')[0].payload).toMatchObject({
      deviceId: 'dev-1',
      templateId: 'tpl-1',
      fields: expect.arrayContaining([
        expect.objectContaining({ id: 'temp', dataType: 'telemetry' }),
        expect.objectContaining({ id: 'setpoint', dataType: 'attribute' })
      ])
    })
  })

  it('resolves requestDeviceFields from deviceConfigId through the device config thing-model map', async () => {
    const { postMessage } = await mountFrame()

    await dispatchFrameMessage({
      type: 'thingsvis:requestDeviceFields',
      payload: { deviceId: 'dev-1', deviceConfigId: 'config-1' }
    })
    await flushAsync()

    expect(hoisted.getDeviceConfigList).toHaveBeenCalledWith({
      page: 1,
      page_size: expect.any(Number)
    })
    expect(hoisted.telemetryApi).toHaveBeenCalledWith({ page: 1, page_size: expect.any(Number), device_template_id: 'tpl-1' })
    expect(hoisted.attributesApi).toHaveBeenCalledWith({ page: 1, page_size: expect.any(Number), device_template_id: 'tpl-1' })
    expect(hoisted.commandsApi).toHaveBeenCalledWith({ page: 1, page_size: expect.any(Number), device_template_id: 'tpl-1' })
    expect(hoisted.eventsApi).toHaveBeenCalledWith({ page: 1, page_size: expect.any(Number), device_template_id: 'tpl-1' })
    expect(postedPayloads(postMessage, 'tv:device-fields')[0].payload).toMatchObject({
      deviceId: 'dev-1',
      templateId: 'tpl-1',
      fields: expect.arrayContaining([
        expect.objectContaining({ id: 'temp', dataType: 'telemetry' }),
        expect.objectContaining({ id: 'setpoint', dataType: 'attribute' })
      ])
    })
  })

  it('builds requested platform data from telemetry, attributes, RDI metadata, and alarm status', async () => {
    const { postMessage } = await mountFrame()

    await dispatchFrameMessage({
      type: 'thingsvis:requestFieldData',
      payload: {
        deviceId: 'dev-1',
        dataSourceId: '__platform_dev-1__',
        fieldIds: ['temp', 'setpoint', 'firmware_version', 'device_alarm_count', 'device_alarm_highest_level']
      }
    })
    await flushAsync()

    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('dev-1', { silentError: true })
    expect(hoisted.getAttributeDataSet).toHaveBeenCalledWith({ device_id: 'dev-1' }, { silentError: true })
    expect(hoisted.rdiDeviceConfig).toHaveBeenCalledWith('dev-1', { silentError: true })
    expect(hoisted.deviceAlarmStatus).toHaveBeenCalledWith({ device_id: 'dev-1', page: 1, page_size: 20 })

    const platformMessages = postedPayloads(postMessage, 'tv:platform-data')
    expect(platformMessages[0].payload).toMatchObject({
      dataSourceId: '__platform_dev-1__',
      deviceId: 'dev-1',
      fields: {
        temp: 31,
        setpoint: 40,
        firmware_version: '1.2.3',
        device_alarm_count: 1,
        device_alarm_highest_level: 'critical'
      }
    })
    expect(MockWebSocket.instances.map(instance => instance.url)).toEqual([
      'wss://platform.test/telemetry/datas/current/ws',
      'wss://platform.test/device/online/status/ws'
    ])
  })

  it('returns structured host errors for ThingsVis device catalog failures', async () => {
    const { postMessage } = await mountFrame({ mode: 'editor' })
    hoisted.deviceList.mockRejectedValueOnce(new Error('search failed'))
    hoisted.telemetryApi.mockRejectedValueOnce(new Error('fields failed'))

    await dispatchFrameMessage({
      type: 'thingsvis:searchDevicesPaged',
      payload: { reqId: 'search-failed', page: 3, pageSize: 10 }
    })
    await dispatchFrameMessage({
      type: 'thingsvis:requestDeviceFields',
      payload: { deviceId: 'dev-1', templateId: 'tpl-error-1' }
    })
    await flushAsync()

    expect(postedPayloads(postMessage, 'tv:search-devices-paged-result')[0].payload).toMatchObject({
      reqId: 'search-failed',
      page: 3,
      pageSize: 10,
      devices: [],
      total: 0,
      success: false,
      error: {
        code: 'host_request_failed',
        message: 'search failed',
        scope: 'search_devices'
      }
    })
    expect(postedPayloads(postMessage, 'tv:device-fields')[0].payload).toMatchObject({
      deviceId: 'dev-1',
      templateId: 'tpl-error-1',
      fields: [],
      success: false,
      error: {
        code: 'host_request_failed',
        message: 'fields failed',
        scope: 'device_fields'
      }
    })
  })

  it('resolves aetherlink data source device ids only from explicit config', async () => {
    const schema = {
      ...defaultSchema(),
      dataSources: [
        {
          id: 'aetherlink_current_device',
          type: 'PLATFORM_FIELD',
          config: { deviceId: 'dev-1', requestedFields: ['temp'] }
        },
        {
          id: 'aetherlink_rdi_device',
          type: 'PLATFORM_FIELD',
          config: { requestedFields: ['temp'] }
        }
      ]
    }
    const { postMessage } = await mountFrame({ schema })

    await dispatchFrameMessage({ type: 'tv:ready' })
    await flushAsync()
    hoisted.telemetryDataCurrent.mockClear()
    hoisted.telemetryDataPub.mockClear()

    await dispatchFrameMessage({
      type: 'thingsvis:requestFieldData',
      payload: {
        dataSourceId: 'aetherlink_current_device',
        fieldIds: ['temp']
      }
    })
    await dispatchFrameMessage({
      type: 'tv:platform-write',
      requestId: 'write-aetherlink',
      payload: { dataSourceId: 'aetherlink_current_device', data: { temp: 33 } }
    })
    await flushAsync()

    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledWith('dev-1', { silentError: true })
    expect(hoisted.telemetryDataPub).toHaveBeenCalledWith({ device_id: 'dev-1', value: JSON.stringify({ temp: 33 }) })
    expect(postedPayloads(postMessage, 'tv:platform-data')).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          payload: expect.objectContaining({
            dataSourceId: 'aetherlink_current_device',
            deviceId: 'dev-1'
          })
        })
      ])
    )

    hoisted.telemetryDataCurrent.mockClear()
    await dispatchFrameMessage({
      type: 'thingsvis:requestFieldData',
      payload: {
        dataSourceId: 'aetherlink_rdi_device',
        fieldIds: ['temp']
      }
    })
    await flushAsync()

    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(0)
  })

  it('selects the highest active alarm level without changing latest alarm fields', async () => {
    hoisted.deviceAlarmStatus.mockResolvedValue({
      data: {
        total: 2,
        list: [
          { alarm_status: 'active', alarm_level: 'warning', alarm_name: 'Recent warning', last_trigger_time: '11:00' },
          { alarm_status: 'active', alarm_level: 'critical', alarm_name: 'Older critical', last_trigger_time: '10:00' }
        ]
      }
    })
    const { postMessage } = await mountFrame()

    await dispatchFrameMessage({
      type: 'thingsvis:requestFieldData',
      payload: {
        deviceId: 'dev-1',
        dataSourceId: '__platform_dev-1__',
        fieldIds: ['device_alarm_highest_level', 'latest_device_alarm_title', 'latest_device_alarm_level']
      }
    })
    await flushAsync()

    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(0)
    expect(hoisted.getAttributeDataSet).toHaveBeenCalledTimes(0)
    expect(hoisted.rdiDeviceConfig).toHaveBeenCalledTimes(0)
    expect(postedPayloads(postMessage, 'tv:platform-data')[0].payload).toMatchObject({
      fields: {
        device_alarm_highest_level: 'critical',
        latest_device_alarm_title: 'Recent warning',
        latest_device_alarm_level: 'warning'
      }
    })
  })

  it('hydrates every viewer platform data source when the same device is reused', async () => {
    const schema = {
      ...defaultSchema(),
      nodes: [
        {
          id: 'node-1',
          props: {
            primary: '{{ ds.__platform_dev_1_primary__.data.temp }}',
            secondary: '{{ ds.__platform_dev_1_secondary__.data.setpoint }}'
          }
        }
      ],
      dataSources: [
        {
          id: '__platform_dev_1_primary__',
          type: 'PLATFORM_FIELD',
          config: { deviceId: 'dev-1', requestedFields: ['temp'] }
        },
        {
          id: '__platform_dev_1_secondary__',
          type: 'PLATFORM_FIELD',
          config: { deviceId: 'dev-1', requestedFields: ['setpoint'] }
        }
      ]
    }
    const { postMessage } = await mountFrame({ mode: 'viewer', schema })

    await dispatchFrameMessage({ type: 'tv:ready' })
    await vi.advanceTimersByTimeAsync(150)
    await flushAsync()
    await vi.advanceTimersByTimeAsync(200)
    await flushAsync()

    const targetedPlatformMessages = postedPayloads(postMessage, 'tv:platform-data')
      .map(message => message.payload)
      .filter(payload => payload?.dataSourceId)

    expect(targetedPlatformMessages).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          dataSourceId: '__platform_dev_1_primary__',
          deviceId: 'dev-1',
          fields: { temp: 31 }
        }),
        expect.objectContaining({
          dataSourceId: '__platform_dev_1_secondary__',
          deviceId: 'dev-1',
          fields: { setpoint: 40 }
        })
      ])
    )
    expect(targetedPlatformMessages).toHaveLength(2)
    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(1)
    expect(hoisted.getAttributeDataSet).toHaveBeenCalledTimes(1)
    expect(MockWebSocket.instances.map(instance => instance.url)).toEqual([
      'wss://platform.test/telemetry/datas/current/ws',
      'wss://platform.test/device/online/status/ws'
    ])
  })

  it('scopes editor prefetch packets for every platform data source when the same device is reused', async () => {
    const schema = {
      ...defaultSchema(),
      nodes: [
        {
          id: 'node-1',
          props: {
            primary: '{{ ds.__platform_dev_1_primary__.data.temp }}',
            secondary: '{{ ds.__platform_dev_1_secondary__.data.setpoint }}'
          }
        }
      ],
      dataSources: [
        {
          id: '__platform_dev_1_primary__',
          type: 'PLATFORM_FIELD',
          config: { deviceId: 'dev-1', requestedFields: ['temp'] }
        },
        {
          id: '__platform_dev_1_secondary__',
          type: 'PLATFORM_FIELD',
          config: { deviceId: 'dev-1', requestedFields: ['setpoint'] }
        }
      ]
    }
    const { postMessage } = await mountFrame({ mode: 'editor', schema })

    await dispatchFrameMessage({ type: 'tv:ready' })
    await vi.advanceTimersByTimeAsync(300)
    await flushAsync()

    const targetedPlatformMessages = postedPayloads(postMessage, 'tv:platform-data')
      .map(message => message.payload)
      .filter(payload => payload?.dataSourceId)

    expect(targetedPlatformMessages).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          dataSourceId: '__platform_dev_1_primary__',
          deviceId: 'dev-1',
          fields: { temp: 31 }
        }),
        expect.objectContaining({
          dataSourceId: '__platform_dev_1_secondary__',
          deviceId: 'dev-1',
          fields: { setpoint: 40 }
        })
      ])
    )
    expect(targetedPlatformMessages).toHaveLength(2)
    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(1)
    expect(hoisted.getAttributeDataSet).toHaveBeenCalledTimes(1)
  })

  it('ignores empty field-data requests without opening device websockets or hitting APIs', async () => {
    const { postMessage } = await mountFrame()

    await dispatchFrameMessage({
      type: 'thingsvis:requestFieldData',
      payload: {
        deviceId: 'dev-1',
        dataSourceId: '__platform_dev-1__',
        fieldIds: []
      }
    })
    await flushAsync()

    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(0)
    expect(hoisted.getAttributeDataSet).toHaveBeenCalledTimes(0)
    expect(hoisted.rdiDeviceConfig).toHaveBeenCalledTimes(0)
    expect(hoisted.deviceAlarmStatus).toHaveBeenCalledTimes(0)
    expect(MockWebSocket.instances).toHaveLength(0)
    expect(postedPayloads(postMessage, 'tv:platform-data')).toHaveLength(0)
  })

  it('returns structured host errors for ThingsVis field-data failures', async () => {
    const { postMessage } = await mountFrame({ mode: 'viewer' })
    hoisted.telemetryDataCurrent.mockRejectedValueOnce(new Error('telemetry failed'))

    await dispatchFrameMessage({
      type: 'thingsvis:requestFieldData',
      payload: {
        deviceId: 'dev-1',
        dataSourceId: 'source-1',
        fieldIds: ['temp']
      }
    })
    await flushAsync()

    expect(postedPayloads(postMessage, 'tv:platform-data')[0].payload).toMatchObject({
      dataSourceId: 'source-1',
      deviceId: 'dev-1',
      fields: {},
      success: false,
      error: {
        code: 'host_request_failed',
        message: 'telemetry failed',
        scope: 'field_data'
      }
    })
  })

  it('routes platform writes to telemetry by default, attributes after field metadata is loaded, and commands with command payloads', async () => {
    const { postMessage } = await mountFrame()

    await dispatchFrameMessage({
      type: 'tv:platform-write',
      requestId: 'write-telemetry',
      payload: { deviceId: 'dev-1', data: { temp: 32 } }
    })
    await dispatchFrameMessage({
      type: 'thingsvis:requestDeviceFields',
      payload: { deviceId: 'dev-1', templateId: 'tpl-1' }
    })
    await dispatchFrameMessage({
      type: 'tv:platform-write',
      requestId: 'write-attribute',
      payload: { deviceId: 'dev-1', data: { setpoint: 41 } }
    })
    await dispatchFrameMessage({
      type: 'tv:platform-write',
      requestId: 'write-command',
      payload: { deviceId: 'dev-1', data: { reboot: { delay: 5 } } }
    })
    await flushAsync()

    expect(hoisted.telemetryDataPub).toHaveBeenCalledWith({ device_id: 'dev-1', value: JSON.stringify({ temp: 32 }) })
    expect(hoisted.attributeDataPub).toHaveBeenCalledWith({
      device_id: 'dev-1',
      value: JSON.stringify({ setpoint: 41 })
    })
    expect(hoisted.commandDataPub).toHaveBeenCalledWith({
      device_id: 'dev-1',
      identify: 'reboot',
      value: JSON.stringify({ delay: 5 })
    })
    expect(postedPayloads(postMessage, 'tv:platform-write-result')).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ requestId: 'write-telemetry', success: true }),
        expect.objectContaining({ requestId: 'write-attribute', success: true }),
        expect.objectContaining({ requestId: 'write-command', success: true })
      ])
    )
  })

  it('ignores untrusted origins, opens preview through the router, and closes device websockets on unmount', async () => {
    const { wrapper, postMessage } = await mountFrame()
    const wrongSource = { postMessage: vi.fn() } as unknown as MessageEventSource

    await dispatchFrameMessage(
      { type: 'tv:save', payload: { config: { meta: { name: 'Ignored' } } } },
      'https://evil.test'
    )
    await dispatchFrameMessage(
      { type: 'tv:save', payload: { config: { meta: { name: 'Wrong source' } } } },
      TARGET_ORIGIN,
      wrongSource
    )
    await dispatchFrameMessage(
      {
        type: 'tv:platform-write',
        requestId: 'wrong-origin-write',
        payload: { deviceId: 'dev-1', data: { temp: 99 } }
      },
      'https://evil.test'
    )
    await dispatchFrameMessage(
      {
        type: 'thingsvis:requestFieldData',
        payload: { deviceId: 'dev-1', fieldIds: ['temp'] }
      },
      TARGET_ORIGIN,
      wrongSource
    )
    expect(hoisted.updateThingsVisDashboard).toHaveBeenCalledTimes(0)
    expect(hoisted.telemetryDataPub).toHaveBeenCalledTimes(0)
    expect(hoisted.telemetryDataCurrent).toHaveBeenCalledTimes(0)
    expect(postedPayloads(postMessage, 'tv:platform-write-result')).toHaveLength(0)

    await dispatchFrameMessage({
      type: 'thingsvis:requestFieldData',
      payload: { deviceId: 'dev-1', fieldIds: ['temp'] }
    })
    await dispatchFrameMessage({ type: 'tv:preview', projectId: 'dashboard-2' })

    expect(hoisted.routerResolve).not.toHaveBeenCalled()
    expect(window.open).toHaveBeenCalledWith('/visualization/thingsvis-preview?id=dashboard-2', '_blank', 'noopener,noreferrer')
    expect(postedPayloads(postMessage, 'tv:platform-data')).toHaveLength(2)

    wrapper.unmount()

    expect(MockWebSocket.instances).toHaveLength(2)
    expect(MockWebSocket.instances.every(instance => instance.close.mock.calls.length === 1)).toBe(true)
  })

  it('adjusts the host iframe height from trusted ThingsVis content-height messages', async () => {
    const { wrapper } = await mountFrame()
    const iframe = wrapper.get('iframe').element as HTMLIFrameElement
    const container = wrapper.get('.thingsvis-frame-container').element as HTMLElement

    await dispatchFrameMessage({
      type: 'tv:content-height',
      payload: { height: 860.2 }
    })

    expect(container.style.height).toBe('861px')
    expect(iframe.style.height).toBe('861px')

    await dispatchFrameMessage(
      {
        type: 'thingsvis:content-height',
        payload: { height: 1200 }
      },
      'https://evil.test'
    )

    expect(container.style.height).toBe('861px')
    expect(iframe.style.height).toBe('861px')

    await dispatchFrameMessage({
      type: 'thingsvis:resize',
      contentHeight: '120px'
    })

    expect(container.style.height).toBe('320px')
    expect(iframe.style.height).toBe('320px')
  })

  it('falls back to the host dashboard id when preview projectId is not a string', async () => {
    await mountFrame()

    await dispatchFrameMessage({ type: 'tv:preview', projectId: { id: 'dashboard-2' } })

    expect(hoisted.routerResolve).not.toHaveBeenCalled()
    expect(window.open).toHaveBeenCalledWith('/visualization/thingsvis-preview?id=dashboard-1', '_blank', 'noopener,noreferrer')
  })
})
