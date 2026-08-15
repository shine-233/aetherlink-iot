/**
 * 文件用途: 场景相关 API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证设备菜单、配置菜单、场景列表、详情、启停和开关请求。
 * 关键注意事项: 场景执行正确性不由本测试证明，需要后端自动化 service 及运行时证据。
 * 重构建议: 与 `automation.test.ts` 明确职责边界，并补条件、动作 payload 的结构断言。
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
  deviceListAll,
  deviceConfigAll,
  sceneAdd,
  sceneEdit,
  sceneGet,
  sceneDel,
  sceneInfo,
  sceneLog,
  sceneActive,
  sceneAutomationsAdd,
  sceneAutomationsEdit,
  sceneAutomationsGet,
  sceneAutomationsDel,
  sceneAutomationsInfo,
  sceneAutomationsLog,
  sceneAutomationsSwitch
} from '../automation'

describe('scene (automation) API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('deviceListAll', () => {
    it('calls GET /device/tenant/list with params', async () => {
      mockGet.mockResolvedValue({ data: [], error: null })
      await deviceListAll({})
      expect(mockGet).toHaveBeenCalledWith('/device/tenant/list', { params: {} })
    })
  })

  describe('deviceConfigAll', () => {
    it('calls GET /device_config/menu with params', async () => {
      mockGet.mockResolvedValue({ data: [], error: null })
      await deviceConfigAll({})
      expect(mockGet).toHaveBeenCalledWith('/device_config/menu', { params: {} })
    })
  })

  describe('sceneAdd', () => {
    it('calls POST /scene with params', async () => {
      mockPost.mockResolvedValue({ data: {}, error: null })
      const params = { name: 'test-scene' }
      await sceneAdd(params)
      expect(mockPost).toHaveBeenCalledWith('/scene', params)
    })
  })

  describe('sceneEdit', () => {
    it('calls PUT /scene with params', async () => {
      mockPut.mockResolvedValue({ data: {}, error: null })
      const params = { id: '1', name: 'updated-scene' }
      await sceneEdit(params)
      expect(mockPut).toHaveBeenCalledWith('/scene', params)
    })
  })

  describe('sceneGet', () => {
    it('calls GET /scene with params', async () => {
      mockGet.mockResolvedValue({ data: { list: [] }, error: null })
      await sceneGet({ page: 1 })
      expect(mockGet).toHaveBeenCalledWith('/scene', { params: { page: 1 } })
    })
  })

  describe('sceneDel', () => {
    it('calls DELETE /scene/{id}', async () => {
      mockDelete.mockResolvedValue({ data: {}, error: null })
      await sceneDel('scene-1')
      expect(mockDelete).toHaveBeenCalledWith('/scene/scene-1')
    })
  })

  describe('sceneInfo', () => {
    it('calls GET /scene/detail/{id}', async () => {
      mockGet.mockResolvedValue({ data: {}, error: null })
      await sceneInfo('scene-1')
      expect(mockGet).toHaveBeenCalledWith('/scene/detail/scene-1')
    })
  })

  describe('sceneLog', () => {
    it('calls GET /scene/log with params', async () => {
      mockGet.mockResolvedValue({ data: { list: [] }, error: null })
      await sceneLog({ page: 1 })
      expect(mockGet).toHaveBeenCalledWith('/scene/log', { params: { page: 1 } })
    })
  })

  describe('sceneActive', () => {
    it('calls POST /scene/active/{id}', async () => {
      mockPost.mockResolvedValue({ data: {}, error: null })
      await sceneActive('scene-1')
      expect(mockPost).toHaveBeenCalledWith('/scene/active/scene-1')
    })
  })

  describe('sceneAutomationsAdd', () => {
    it('calls POST /scene_automations with params', async () => {
      mockPost.mockResolvedValue({ data: {}, error: null })
      const params = { name: 'test-automation' }
      await sceneAutomationsAdd(params)
      expect(mockPost).toHaveBeenCalledWith('/scene_automations', params)
    })
  })

  describe('sceneAutomationsEdit', () => {
    it('calls PUT /scene_automations with params', async () => {
      mockPut.mockResolvedValue({ data: {}, error: null })
      const params = { id: '1', name: 'updated-automation' }
      await sceneAutomationsEdit(params)
      expect(mockPut).toHaveBeenCalledWith('/scene_automations', params)
    })
  })

  describe('sceneAutomationsGet', () => {
    it('calls GET /scene_automations/list with params', async () => {
      mockGet.mockResolvedValue({ data: { list: [] }, error: null })
      await sceneAutomationsGet({ page: 1 })
      expect(mockGet).toHaveBeenCalledWith('/scene_automations/list', { params: { page: 1 } })
    })
  })

  describe('sceneAutomationsDel', () => {
    it('calls DELETE /scene_automations/{id}', async () => {
      mockDelete.mockResolvedValue({ data: {}, error: null })
      await sceneAutomationsDel('auto-1')
      expect(mockDelete).toHaveBeenCalledWith('/scene_automations/auto-1')
    })
  })

  describe('sceneAutomationsInfo', () => {
    it('calls GET /scene_automations/detail/{id}', async () => {
      mockGet.mockResolvedValue({ data: {}, error: null })
      await sceneAutomationsInfo('auto-1')
      expect(mockGet).toHaveBeenCalledWith('/scene_automations/detail/auto-1')
    })
  })

  describe('sceneAutomationsLog', () => {
    it('calls GET /scene_automations/log with params', async () => {
      mockGet.mockResolvedValue({ data: { list: [] }, error: null })
      await sceneAutomationsLog({ page: 1 })
      expect(mockGet).toHaveBeenCalledWith('/scene_automations/log', { params: { page: 1 } })
    })
  })

  describe('sceneAutomationsSwitch', () => {
    it('calls POST /scene_automations/switch/{id}', async () => {
      mockPost.mockResolvedValue({ data: {}, error: null })
      await sceneAutomationsSwitch('auto-1')
      expect(mockPost).toHaveBeenCalledWith('/scene_automations/switch/auto-1')
    })
  })
})
