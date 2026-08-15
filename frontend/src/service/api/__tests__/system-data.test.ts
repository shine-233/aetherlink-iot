/**
 * 文件用途: 系统数据 wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证系统统计、租户、物模型、遥测、属性、事件和命令请求。
 * 关键注意事项: 物模型真实约束和数据统计准确性不在 mock wrapper 测试中证明。
 * 重构建议: 按统计、租户、物模型和遥测/属性/事件/命令拆分用例。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost,
    put: mockPut,
    delete: mockDelete
  }
}))

import {
  addAttributes,
  addCommands,
  addEvents,
  addTelemetry,
  addTemplat,
  attributesApi,
  commandsApi,
  delAttributes,
  delCommands,
  delEvents,
  delTelemetry,
  deviceCustomCommandsAdd,
  deviceCustomCommandsDel,
  deviceCustomCommandsList,
  deviceCustomCommandsPut,
  deviceCustomControlAdd,
  deviceCustomControlDel,
  deviceCustomControlList,
  deviceCustomControlPut,
  eventsApi,
  getAlarmCount,
  getLatestTelemetryData,
  getOnlineDeviceTrend,
  getSysVersion,
  getSystemMetricsCurrent,
  getSystemMetricsHistory,
  getTemplat,
  putAttributes,
  putCommands,
  putEvents,
  putTelemetry,
  putTemplat,
  sumData,
  telemetryApi,
  telemetryLatestApi,
  tenant,
  tenantNum,
  totalNumber
} from '../system-data'

describe('system-data API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('covers dashboard statistic and trend endpoints', async () => {
    mockGet.mockResolvedValue({ data: {}, error: null })

    await totalNumber()
    await sumData()
    await tenantNum()
    await tenant()
    await getOnlineDeviceTrend()
    await getAlarmCount()

    expect(mockGet).toHaveBeenNthCalledWith(1, '/board/device')
    expect(mockGet).toHaveBeenNthCalledWith(2, '/board/tenant/device/info')
    expect(mockGet).toHaveBeenNthCalledWith(3, '/telemetry/datas/msg/count')
    expect(mockGet).toHaveBeenNthCalledWith(4, '/board/tenant')
    expect(mockGet).toHaveBeenNthCalledWith(5, '/board/trend', { params: undefined })
    expect(mockGet).toHaveBeenNthCalledWith(6, '/alarm/device/counts')
  })

  it('passes the explicit all-tenant alarm-count scope', async () => {
    mockGet.mockResolvedValue({ data: {}, error: null })

    await getAlarmCount({ all_tenants: true })

    expect(mockGet).toHaveBeenCalledWith('/alarm/device/counts', {
      params: { all_tenants: true }
    })
  })

  it('passes the explicit all-tenant device-overview scope', async () => {
    mockGet.mockResolvedValue({ data: {}, error: null })

    await sumData({ all_tenants: true })

    expect(mockGet).toHaveBeenCalledWith('/board/tenant/device/info', {
      params: { all_tenants: true }
    })
  })

  it('passes dashboard trend time range and tenant filter params', async () => {
    mockGet.mockResolvedValue({ data: {}, error: null })

    await getOnlineDeviceTrend({ start_time: 1700000000, end_time: 1700086400, tenant_id: 'tenant-rdi-1' })

    expect(mockGet).toHaveBeenCalledWith('/board/trend', {
      params: { start_time: 1700000000, end_time: 1700086400, tenant_id: 'tenant-rdi-1' }
    })
  })

  it('covers thing model create, update, and detail endpoints', async () => {
    mockPost.mockResolvedValue({ data: { id: 'template-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockGet.mockResolvedValue({ data: { id: 'template-1' }, error: null })

    const payload = {
      name: 'Temperature template',
      protocol_type: 'MQTT',
      device_type: '1'
    }

    await addTemplat(payload)
    await putTemplat({ id: 'template-1', ...payload })
    await getTemplat('template-1')

    expect(mockPost).toHaveBeenCalledWith('/device/template', payload)
    expect(mockPut).toHaveBeenCalledWith('/device/template', { id: 'template-1', ...payload })
    expect(mockGet).toHaveBeenCalledWith('/device/template/detail/template-1')
  })

  it('covers telemetry, attributes, events, and commands model query/create/update/delete contracts', async () => {
    mockGet.mockResolvedValue({ data: { list: [] }, error: null })
    mockPost.mockResolvedValue({ data: { id: 'model-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const query = { device_template_id: 'template-1', page: 1, page_size: 10 }
    const payload = { device_template_id: 'template-1', data_identifier: 'temperature', data_name: 'Temperature' }

    await telemetryApi(query)
    await attributesApi(query)
    await eventsApi(query)
    await commandsApi(query)
    await addTelemetry(payload)
    await putTelemetry({ id: 'telemetry-1', ...payload })
    await addAttributes(payload)
    await putAttributes({ id: 'attr-1', ...payload })
    await addEvents(payload)
    await putEvents({ id: 'event-1', ...payload })
    await addCommands(payload)
    await putCommands({ id: 'command-1', ...payload })
    await delTelemetry('telemetry-1')
    await delAttributes('attr-1')
    await delEvents('event-1')
    await delCommands('command-1')

    expect(mockGet).toHaveBeenNthCalledWith(1, '/device/model/telemetry', { params: query })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/device/model/attributes', { params: query })
    expect(mockGet).toHaveBeenNthCalledWith(3, '/device/model/events', { params: query })
    expect(mockGet).toHaveBeenNthCalledWith(4, '/device/model/commands', { params: query })
    expect(mockPost).toHaveBeenNthCalledWith(1, '/device/model/telemetry', payload)
    expect(mockPut).toHaveBeenNthCalledWith(1, '/device/model/telemetry', { id: 'telemetry-1', ...payload })
    expect(mockPost).toHaveBeenNthCalledWith(2, '/device/model/attributes', payload)
    expect(mockPut).toHaveBeenNthCalledWith(2, '/device/model/attributes', { id: 'attr-1', ...payload })
    expect(mockPost).toHaveBeenNthCalledWith(3, '/device/model/events', payload)
    expect(mockPut).toHaveBeenNthCalledWith(3, '/device/model/events', { id: 'event-1', ...payload })
    expect(mockPost).toHaveBeenNthCalledWith(4, '/device/model/commands', payload)
    expect(mockPut).toHaveBeenNthCalledWith(4, '/device/model/commands', { id: 'command-1', ...payload })
    expect(mockDelete).toHaveBeenNthCalledWith(1, '/device/model/telemetry/telemetry-1')
    expect(mockDelete).toHaveBeenNthCalledWith(2, '/device/model/attributes/attr-1')
    expect(mockDelete).toHaveBeenNthCalledWith(3, '/device/model/events/event-1')
    expect(mockDelete).toHaveBeenNthCalledWith(4, '/device/model/commands/command-1')
  })

  it('covers current telemetry and latest telemetry aggregate endpoints', async () => {
    mockGet.mockResolvedValue({ data: [], error: null })

    await telemetryLatestApi('device-1')
    await getLatestTelemetryData()

    expect(mockGet).toHaveBeenNthCalledWith(1, '/telemetry/datas/current/device-1')
    expect(mockGet).toHaveBeenNthCalledWith(2, '/device/telemetry/latest')
  })

  it('covers custom command and custom control list/create/update/delete contracts', async () => {
    mockGet.mockResolvedValue({ data: { list: [] }, error: null })
    mockPost.mockResolvedValue({ data: { id: 'custom-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const query = { device_template_id: 'template-1', page: 1, page_size: 10 }
    const commandPayload = { device_template_id: 'template-1', name: 'reset', content: '{}' }
    const controlPayload = { device_template_id: 'template-1', name: 'switch', config: '{}' }

    await deviceCustomCommandsList(query)
    await deviceCustomCommandsAdd(commandPayload)
    await deviceCustomCommandsPut({ id: 'command-1', ...commandPayload })
    await deviceCustomCommandsDel('command-1')
    await deviceCustomControlList(query)
    await deviceCustomControlAdd(controlPayload)
    await deviceCustomControlPut({ id: 'control-1', ...controlPayload })
    await deviceCustomControlDel('control-1')

    expect(mockGet).toHaveBeenNthCalledWith(1, '/device/model/custom/commands', { params: query })
    expect(mockPost).toHaveBeenNthCalledWith(1, '/device/model/custom/commands', commandPayload)
    expect(mockPut).toHaveBeenNthCalledWith(1, '/device/model/custom/commands', {
      id: 'command-1',
      ...commandPayload
    })
    expect(mockDelete).toHaveBeenNthCalledWith(1, '/device/model/custom/commands/command-1')
    expect(mockGet).toHaveBeenNthCalledWith(2, '/device/model/custom/control', { params: query })
    expect(mockPost).toHaveBeenNthCalledWith(2, '/device/model/custom/control', controlPayload)
    expect(mockPut).toHaveBeenNthCalledWith(2, '/device/model/custom/control', {
      id: 'control-1',
      ...controlPayload
    })
    expect(mockDelete).toHaveBeenNthCalledWith(2, '/device/model/custom/control/control-1')
  })

  it('covers system metrics current/history and system version endpoints', async () => {
    mockGet.mockResolvedValue({ data: {}, error: null })

    await getSystemMetricsCurrent({ metric: 'cpu' })
    await getSysVersion({ include_build: true })
    await getSystemMetricsHistory({ metric: 'memory', duration: '1h' })

    expect(mockGet).toHaveBeenNthCalledWith(1, '/system/metrics/current', { params: { metric: 'cpu' } })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/sys_version', { params: { include_build: true } })
    expect(mockGet).toHaveBeenNthCalledWith(3, '/system/metrics/history', {
      params: { metric: 'memory', duration: '1h' }
    })
  })
})
