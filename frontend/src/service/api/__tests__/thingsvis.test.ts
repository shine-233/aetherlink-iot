/**
 * 文件用途: ThingsVis API helper 的请求合同和健康探测测试。
 * 核心逻辑: mock request/fetch 后验证 ThingsVis 项目、看板、缩略图、导入导出和后端可达性判断。
 * 关键注意事项: 这里不验证 ThingsVis SDK 渲染行为，运行时嵌入合同仍需页面或 E2E 证据。
 * 重构建议: 按项目、看板、导入导出、健康检查拆分用例，并补后端不可达的错误分支。
 */
import { afterEach, describe, expect, it, vi } from 'vitest'

import { probeThingsVisBackend } from '../thingsvis'

describe('ThingsVis API helpers - thingsvis.ts', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    vi.restoreAllMocks()
    global.fetch = originalFetch
  })

  it('treats auth-required responses as reachable backend', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 401,
      text: vi.fn().mockResolvedValue('unauthorized')
    }) as any

    await expect(probeThingsVisBackend()).resolves.toBe(true)
  })

  it('treats proxy refusal payloads as unreachable backend', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 500,
      text: vi.fn().mockResolvedValue('connect ECONNREFUSED 127.0.0.1:8000')
    }) as any

    await expect(probeThingsVisBackend()).resolves.toBe(false)
  })

  it('treats empty 5xx proxy responses as unreachable backend', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 500,
      text: vi.fn().mockResolvedValue('')
    }) as any

    await expect(probeThingsVisBackend()).resolves.toBe(false)
  })

  it('treats thrown network failures as unreachable backend', async () => {
    global.fetch = vi.fn().mockRejectedValue(new TypeError('Failed to fetch')) as any

    await expect(probeThingsVisBackend()).resolves.toBe(false)
  })
})
