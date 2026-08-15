/**
 * 文件用途：验证统一数据执行器的本地转换语义和外部能力边界。
 * 核心逻辑：覆盖 HTTP、静态、JSON 的共享转换，以及脚本和 WebSocket 阻断契约。
 * 关键注意事项：测试不得建立真实网络连接或执行任意脚本。
 */
import { afterEach, describe, expect, it, vi } from 'vitest'
import { request } from '@/service/request'
import { UnifiedDataExecutor, type UnifiedDataConfig } from './UnifiedDataExecutor'

vi.mock('@/service/request', () => ({
  request: vi.fn()
}))

const requestMock = vi.mocked(request)

const websocketConfig = (wsUrl?: string): UnifiedDataConfig => ({
  id: 'telemetry-stream',
  type: 'websocket',
  config: { wsUrl }
})

afterEach(() => {
  requestMock.mockReset()
  vi.unstubAllGlobals()
})

describe('UnifiedDataExecutor local transforms', () => {
  it('applies path and array filter to static data', async () => {
    const executor = new UnifiedDataExecutor()

    await expect(
      executor.execute({
        id: 'static-source',
        type: 'static',
        config: {
          data: {
            payload: [
              { id: 1, enabled: true },
              { id: 2, enabled: false },
              { id: 3, enabled: true }
            ]
          },
          transform: { path: 'payload', filter: { enabled: true } }
        }
      })
    ).resolves.toMatchObject({
      success: true,
      data: [
        { id: 1, enabled: true },
        { id: 3, enabled: true }
      ]
    })
  })

  it('applies path and field mapping to JSON data', async () => {
    const executor = new UnifiedDataExecutor()

    await expect(
      executor.execute({
        id: 'json-source',
        type: 'json',
        config: {
          jsonContent: JSON.stringify({ payload: { device: { id: 'dev-1' }, value: 0 } }),
          transform: {
            path: 'payload',
            mapping: { deviceId: 'device.id', reading: 'value', missing: 'unknown.path' }
          }
        }
      })
    ).resolves.toMatchObject({
      success: true,
      data: { deviceId: 'dev-1', reading: 0, missing: null }
    })
  })

  it('uses the project request adapter and applies the same HTTP transform', async () => {
    requestMock.mockResolvedValue({
      data: {
        payload: [
          { id: 'a', state: 'online' },
          { id: 'b', state: 'offline' }
        ]
      }
    } as any)
    const executor = new UnifiedDataExecutor()

    await expect(
      executor.execute({
        id: 'http-source',
        type: 'http',
        config: {
          url: '/api/devices',
          transform: { path: 'payload', filter: { state: 'online' } }
        }
      })
    ).resolves.toMatchObject({
      success: true,
      data: [{ id: 'a', state: 'online' }]
    })
    expect(requestMock).toHaveBeenCalledOnce()
  })

  it('returns null when a transform path does not exist', async () => {
    const executor = new UnifiedDataExecutor()

    await expect(
      executor.execute({
        id: 'missing-path',
        type: 'static',
        config: { data: { value: 1 }, transform: { path: 'payload.items' } }
      })
    ).resolves.toMatchObject({ success: true, data: null })
  })

  it('keeps malformed JSON distinct from transform failures', async () => {
    const executor = new UnifiedDataExecutor()

    await expect(
      executor.execute({ id: 'bad-json', type: 'json', config: { jsonContent: '{bad' } })
    ).resolves.toMatchObject({ success: false, errorCode: 'JSON_PARSE_ERROR' })
  })

  it.each([
    ['static', { data: { value: 1 } }],
    ['json', { jsonContent: '{"value":1}' }],
    ['http', { url: '/api/value' }]
  ] as const)('blocks %s script transforms without evaluating them', async (type, config) => {
    requestMock.mockResolvedValue({ data: { value: 1 } } as any)
    const executor = new UnifiedDataExecutor()

    await expect(
      executor.execute({
        id: `${type}-script`,
        type,
        config: { ...config, transform: { script: 'return globalThis.secret' } }
      })
    ).resolves.toMatchObject({
      success: false,
      errorCode: 'TRANSFORM_SCRIPT_EXTERNAL_BLOCKED'
    })
  })
})

describe('UnifiedDataExecutor file boundary', () => {
  const fileConfig = (filePath?: string): UnifiedDataConfig => ({
    id: 'local-file',
    type: 'file',
    config: { filePath, fileType: 'json', encoding: 'utf-8' }
  })

  it('keeps file as a recognized configuration type', () => {
    const executor = new UnifiedDataExecutor()

    expect(executor.getSupportedTypes()).toContain('file')
    expect(executor.validateConfig(fileConfig('/data/telemetry.json'))).toBe(true)
  })

  it('rejects a missing path before attempting external work', async () => {
    const executor = new UnifiedDataExecutor()

    await expect(executor.execute(fileConfig('   '))).resolves.toMatchObject({
      success: false,
      errorCode: 'FILE_NO_PATH',
      sourceId: 'local-file'
    })
  })

  it('reports the missing file adapter instead of treating file as unsupported', async () => {
    const executor = new UnifiedDataExecutor()

    await expect(executor.execute(fileConfig('/data/telemetry.json'))).resolves.toMatchObject({
      success: false,
      data: { status: 'external-blocked' },
      errorCode: 'FILE_EXTERNAL_BLOCKED',
      sourceId: 'local-file'
    })
    expect(requestMock).not.toHaveBeenCalled()
  })
})

describe('UnifiedDataExecutor WebSocket boundary', () => {
  it('keeps websocket as a recognized configuration type', () => {
    const executor = new UnifiedDataExecutor()

    expect(executor.getSupportedTypes()).toContain('websocket')
    expect(executor.validateConfig(websocketConfig('wss://example.test/telemetry'))).toBe(true)
  })

  it('rejects a missing endpoint before attempting external work', async () => {
    const executor = new UnifiedDataExecutor()

    await expect(executor.execute(websocketConfig('   '))).resolves.toMatchObject({
      success: false,
      errorCode: 'WS_NO_URL',
      sourceId: 'telemetry-stream'
    })
  })

  it('reports the unimplemented external adapter instead of a fake connecting success', async () => {
    const websocketConstructor = vi.fn()
    vi.stubGlobal('WebSocket', websocketConstructor)
    const executor = new UnifiedDataExecutor()

    await expect(executor.execute(websocketConfig('wss://example.test/telemetry'))).resolves.toMatchObject({
      success: false,
      data: { status: 'external-blocked' },
      errorCode: 'WS_EXTERNAL_BLOCKED',
      sourceId: 'telemetry-stream'
    })
    expect(websocketConstructor).not.toHaveBeenCalled()
  })

  it('allows cleanup without external resources', () => {
    const executor = new UnifiedDataExecutor()

    expect(() => executor.cleanup()).not.toThrow()
  })
})
