/*
 * 文件用途：验证 ThingsVis iframe SDK 的 postMessage 安全边界。
 * 核心逻辑：断言消息发送使用最终 iframe URL origin，并只信任目标 iframe window 和 origin。
 * 关键注意事项：测试覆盖的是嵌入通信安全契约，不能随意放宽来源校验。
 * 重构建议：如增加消息类型，应补发送、接收和拒绝分支测试。
 */
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    debug: vi.fn(),
    warn: vi.fn()
  })
}))

vi.mock('@/utils/thingsvis/constants', () => ({
  getPlatformApiBase: () => 'https://platform.test/api'
}))

import { ThingsVisClient } from './client'

const mountClient = (url = 'https://studio.test/thingsvis/#/embed?token=abc') => {
  const originalCreateElement = document.createElement.bind(document)
  const iframe = originalCreateElement('div') as unknown as HTMLIFrameElement
  const iframeWindow = { postMessage: vi.fn() } as unknown as Window

  Object.defineProperty(iframe, 'src', {
    configurable: true,
    writable: true,
    value: ''
  })
  Object.defineProperty(iframe, 'contentWindow', {
    configurable: true,
    value: iframeWindow
  })
  vi.spyOn(document, 'createElement').mockImplementation((tagName: string, options?: ElementCreationOptions) => {
    if (tagName === 'iframe') return iframe
    return originalCreateElement(tagName, options)
  })

  const container = document.createElement('div')
  document.body.appendChild(container)
  const client = new ThingsVisClient({
    container,
    mode: 'widget',
    url
  })

  return {
    client,
    container,
    iframe,
    iframeWindow,
    postMessage: iframeWindow.postMessage as unknown as ReturnType<typeof vi.fn>
  }
}

describe('ThingsVisClient postMessage guard', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('posts to the origin parsed from the final iframe URL', () => {
    const { client, iframe, postMessage } = mountClient()
    client.ready = true

    client.updateSchema([{ id: 'temp' }])

    expect(iframe.src).toContain('https://studio.test/thingsvis/#/embed?token=abc')
    expect(postMessage).toHaveBeenCalledWith(
      { type: 'tv:event', payload: { event: 'updateSchema', payload: [{ id: 'temp' }] } },
      'https://studio.test'
    )
  })

  it('appends embed params to the hash route query when the studio entry has its own query', () => {
    const { iframe } = mountClient('https://studio.test/main.html?tenant=fixture#/embed')

    expect(iframe.src).toBe(
      'https://studio.test/main.html?tenant=fixture#/embed?mode=embedded&showTopLeft=0&showTopRight=0'
    )
  })

  it('trusts inbound messages only from the current iframe window and target origin', () => {
    const { client, iframeWindow } = mountClient()
    const trustedEvent = new MessageEvent('message', {
      data: { type: 'tv:ready' },
      source: iframeWindow,
      origin: 'https://studio.test'
    })
    const wrongOriginEvent = new MessageEvent('message', {
      data: { type: 'tv:ready' },
      source: iframeWindow,
      origin: 'https://evil.test'
    })
    const wrongSourceEvent = new MessageEvent('message', {
      data: { type: 'tv:ready' },
      source: { postMessage: vi.fn() } as unknown as MessageEventSource,
      origin: 'https://studio.test'
    })

    expect(client.isTrustedMessageEvent(trustedEvent)).toBe(true)
    expect(client.isTrustedMessageEvent(wrongOriginEvent)).toBe(false)
    expect(client.isTrustedMessageEvent(wrongSourceEvent)).toBe(false)
  })
})
