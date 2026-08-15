/**
 * 文件用途: 自动化 API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证自动化规则、日志、启停、开关和菜单查询请求。
 * 关键注意事项: 规则执行正确性不在本测试层证明，仍需后端自动化 service 和 E2E 证据。
 * 重构建议: 按规则 CRUD、执行日志、启停操作拆分测试，并补 payload 结构断言。
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
  configMetricsConditionMenu,
  deviceConfigAll,
  deviceConfigMetricsMenu,
  deviceListAll,
  deviceMetricsConditionMenu,
  deviceMetricsMenu,
  sceneActive,
  sceneAdd,
  sceneAutomationsAdd,
  sceneAutomationsDel,
  sceneAutomationsEdit,
  sceneAutomationsGet,
  sceneAutomationsInfo,
  sceneAutomationsLog,
  sceneAutomationsSwitch,
  sceneDel,
  sceneEdit,
  sceneGet,
  sceneInfo,
  sceneLog
} from '../automation'

describe('automation API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches device, config, condition, and action menus used by scene builders', async () => {
    mockGet.mockResolvedValue({ data: [], error: null })

    const deviceQuery = { page: 1, page_size: 100, name: 'pump' }
    const configQuery = { product_type: 'rdi' }
    const metricsQuery = { device_id: 'device-1' }

    await deviceListAll(deviceQuery)
    await deviceConfigAll(configQuery)
    await deviceMetricsConditionMenu(metricsQuery)
    await configMetricsConditionMenu(configQuery)
    await deviceMetricsMenu(metricsQuery)
    await deviceConfigMetricsMenu(configQuery)

    expect(mockGet).toHaveBeenNthCalledWith(1, '/device/tenant/list', { params: deviceQuery })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/device_config/menu', { params: configQuery })
    expect(mockGet).toHaveBeenNthCalledWith(3, '/device/metrics/condition/menu', { params: metricsQuery })
    expect(mockGet).toHaveBeenNthCalledWith(4, '/device_config/metrics/condition/menu', { params: configQuery })
    expect(mockGet).toHaveBeenNthCalledWith(5, '/device/metrics/menu', { params: metricsQuery })
    expect(mockGet).toHaveBeenNthCalledWith(6, '/device_config/metrics/menu', { params: configQuery })
  })

  it('covers scene CRUD, detail, log, and activation contracts', async () => {
    mockGet.mockResolvedValue({ data: {}, error: null })
    mockPost.mockResolvedValue({ data: { id: 'scene-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const payload = {
      name: 'temperature alarm',
      conditions: [{ key: 'temperature', operator: '>', value: 30 }],
      actions: [{ type: 'notify' }]
    }
    const query = { page: 1, page_size: 10 }

    await sceneAdd(payload)
    await sceneEdit({ id: 'scene-1', ...payload })
    await sceneGet(query)
    await sceneInfo('scene-1')
    await sceneLog({ scene_id: 'scene-1' })
    await sceneActive('scene-1')
    await sceneDel('scene-1')

    expect(mockPost).toHaveBeenNthCalledWith(1, '/scene', payload)
    expect(mockPut).toHaveBeenCalledWith('/scene', { id: 'scene-1', ...payload })
    expect(mockGet).toHaveBeenNthCalledWith(1, '/scene', { params: query })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/scene/detail/scene-1')
    expect(mockGet).toHaveBeenNthCalledWith(3, '/scene/log', { params: { scene_id: 'scene-1' } })
    expect(mockPost).toHaveBeenNthCalledWith(2, '/scene/active/scene-1')
    expect(mockDelete).toHaveBeenCalledWith('/scene/scene-1')
  })

  it('covers scene automation CRUD, detail, log, and switch contracts', async () => {
    mockGet.mockResolvedValue({ data: {}, error: null })
    mockPost.mockResolvedValue({ data: { id: 'automation-1' }, error: null })
    mockPut.mockResolvedValue({ data: null, error: null })
    mockDelete.mockResolvedValue({ data: null, error: null })

    const payload = {
      name: 'device linkage',
      trigger_type: 'telemetry',
      conditions: [{ metric: 'switch', value: true }]
    }
    const query = { page: 2, page_size: 20, name: 'device' }

    await sceneAutomationsAdd(payload)
    await sceneAutomationsEdit({ id: 'automation-1', ...payload })
    await sceneAutomationsGet(query)
    await sceneAutomationsInfo('automation-1')
    await sceneAutomationsLog({ scene_automation_id: 'automation-1' })
    await sceneAutomationsSwitch('automation-1')
    await sceneAutomationsDel('automation-1')

    expect(mockPost).toHaveBeenNthCalledWith(1, '/scene_automations', payload)
    expect(mockPut).toHaveBeenCalledWith('/scene_automations', { id: 'automation-1', ...payload })
    expect(mockGet).toHaveBeenNthCalledWith(1, '/scene_automations/list', { params: query })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/scene_automations/detail/automation-1')
    expect(mockGet).toHaveBeenNthCalledWith(3, '/scene_automations/log', {
      params: { scene_automation_id: 'automation-1' }
    })
    expect(mockPost).toHaveBeenNthCalledWith(2, '/scene_automations/switch/automation-1')
    expect(mockDelete).toHaveBeenCalledWith('/scene_automations/automation-1')
  })
})
