/**
 * 文件用途: 物模型市场 API wrapper 的请求合同测试。
 * 核心逻辑: mock request 层后验证市场登录、发布、物模型查询、详情和安装请求。
 * 关键注意事项: 外部市场认证和安装真实效果不由本测试证明，需要集成或人工验证。
 * 重构建议: 补充认证失败、安装失败和分页参数的断言。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockPost } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost
  }
}))

import {
  getMarketTemplateDetail,
  getMarketTemplates,
  installFromMarket,
  marketLogin,
  publishToMarket
} from '../market'

describe('market API service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('logs in to the template market with username and password', async () => {
    mockPost.mockResolvedValue({ data: { token: 'market-token' }, error: null })
    const credentials = { username: 'tenant@example.com', password: 'secret' }

    const result = await marketLogin(credentials)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/device/template/market/login', credentials)
    expect(result.data.token).toBe('market-token')
  })

  it('publishes a device configuration to the market with full template metadata', async () => {
    mockPost.mockResolvedValue({ data: { market_template_id: 'market-1' }, error: null })
    const payload = {
      device_config_id: 'cfg-1',
      market_token: 'market-token',
      market_name: 'Temperature Controller',
      brand: 'AetherLink',
      model: 'RDI-T',
      category: 'environment',
      version: '1.0.0',
      author: 'ops',
      description: 'factory temperature controller'
    }

    await publishToMarket(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/device/template/market/publish', payload)
  })

  it('fetches market template list using search, category, sort, and pagination params', async () => {
    mockGet.mockResolvedValue({ data: { total: 1, list: [] }, error: null })
    const params = {
      keyword: 'rdi',
      category: 'factory',
      sort_by: 'latest',
      page: 1,
      page_size: 12
    }

    await getMarketTemplates(params)

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/device/template/market/list', { params })
  })

  it('fetches market template detail by market id', async () => {
    mockGet.mockResolvedValue({ data: { id: 'market-1' }, error: null })

    await getMarketTemplateDetail('market-1')

    expect(mockGet).toHaveBeenCalledTimes(1)
    expect(mockGet).toHaveBeenCalledWith('/device/template/market/detail/market-1')
  })

  it('installs a selected market template version through backend proxy', async () => {
    mockPost.mockResolvedValue({ data: { device_config_id: 'cfg-new' }, error: null })
    const payload = {
      market_template_id: 'market-1',
      version: '1.0.1',
      market_token: 'market-token'
    }

    await installFromMarket(payload)

    expect(mockPost).toHaveBeenCalledTimes(1)
    expect(mockPost).toHaveBeenCalledWith('/device/template/market/install', payload)
  })
})
