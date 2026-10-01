import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/service/api/device', () => ({
  commandDataPub: vi.fn(),
  getCommandDeliveryDiagnostics: vi.fn()
}))
vi.mock('@/utils/storage', () => ({ localStg: { get: () => 'token-1' } }))
vi.mock('@/utils/common/tool', () => ({ getWebsocketServerUrl: () => 'ws://test' }))
vi.mock('@/locales', () => ({ $t: (key: string) => key }))

class FakeWebSocket {
  static OPEN = 1
  static instances: FakeWebSocket[] = []
  readyState = 0
  onopen: (() => void) | null = null
  onmessage: ((e: { data: unknown }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  sent: unknown[] = []
  constructor(public url: string) {
    FakeWebSocket.instances.push(this)
  }
  send(data: unknown) {
    this.sent.push(data)
  }
  close() {
    this.readyState = 3
    // 真实浏览器里 onclose 异步到达
    setTimeout(() => this.onclose?.(), 0)
  }
}

import DeviceDebugLive from '../device-debug-live.vue'

const mountIt = () =>
  shallowMount(DeviceDebugLive, {
    props: { id: 'dev-1' },
    global: { stubs: { NCard: true, NTag: true, NButton: true, NSpace: true, NInput: true, NSwitch: true } }
  })

describe('device-debug-live websocket lifecycle', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('does not reconnect after manual stop / unmount', () => {
    const wrapper = mountIt()
    const vm = wrapper.vm as unknown as { startSocket: () => void; stopSocket: () => void }
    vm.startSocket()
    expect(FakeWebSocket.instances).toHaveLength(1)
    wrapper.unmount()
    vi.advanceTimersByTime(10_000)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it('stale onclose of a stopped socket does not clobber a newer socket', () => {
    const wrapper = mountIt()
    const vm = wrapper.vm as unknown as { startSocket: () => void; stopSocket: () => void }
    vm.startSocket()
    vm.stopSocket()
    vm.startSocket()
    expect(FakeWebSocket.instances).toHaveLength(2)
    vi.advanceTimersByTime(10_000) // 旧连接 onclose 到达
    expect(FakeWebSocket.instances).toHaveLength(2)
    const second = FakeWebSocket.instances[1]
    second.readyState = 1
    second.onopen?.()
    expect(second.sent[0]).toContain('dev-1')
    wrapper.unmount()
  })

  it('still auto-reconnects when the server drops the connection', () => {
    const wrapper = mountIt()
    const vm = wrapper.vm as unknown as { startSocket: () => void }
    vm.startSocket()
    FakeWebSocket.instances[0].onclose?.()
    vi.advanceTimersByTime(3000)
    expect(FakeWebSocket.instances).toHaveLength(2)
    wrapper.unmount()
  })
})
