/**
 * 文件用途: device-detail-route 纯函数与 useDeviceOnlineStatusSocket 的单元测试。
 */
import { describe, expect, it, vi } from 'vitest'

const { mockSend } = vi.hoisted(() => ({ mockSend: vi.fn() }))
let capturedOnMessage: ((ws: WebSocket, event: MessageEvent) => void) | undefined

vi.mock('@vueuse/core', () => ({
  useWebSocket: (_url: string, options: { onMessage: typeof capturedOnMessage }) => {
    capturedOnMessage = options.onMessage
    return { send: mockSend }
  }
}))
vi.mock('@/utils/common/tool', () => ({ getWebsocketServerUrl: () => 'ws://socket.test' }))
vi.mock('@/utils/storage', () => ({ localStg: { get: () => 'tok' } }))

import {
  isDomEventPayload,
  isSharedRouteQuery,
  normalizeAlarmActive,
  normalizeRouteQueryParam,
  resolveDeviceAlarmActive
} from '../device-detail-route'
import { useDeviceOnlineStatusSocket } from '../useDeviceOnlineStatusSocket'

describe('device-detail-route', () => {
  it('normalizes route query params from arrays, nullish and non-strings', () => {
    expect(normalizeRouteQueryParam(['a', 'b'])).toBe('a')
    expect(normalizeRouteQueryParam(null)).toBe('')
    expect(normalizeRouteQueryParam(undefined)).toBe('')
    expect(normalizeRouteQueryParam(12)).toBe('12')
  })

  it('detects shared read-only entry query', () => {
    expect(isSharedRouteQuery({ shared: '1' })).toBe(true)
    expect(isSharedRouteQuery({ shared: 'TRUE' })).toBe(true)
    expect(isSharedRouteQuery({ access: 'Shared' })).toBe(true)
    expect(isSharedRouteQuery({ shared: '0', access: 'owner' })).toBe(false)
    expect(isSharedRouteQuery({})).toBe(false)
  })

  it('normalizes alarm flags across legacy value shapes', () => {
    for (const falsy of ['N', 'n', '0', 'false', 'off', 'no', '', ' ', 0, false, null, undefined]) {
      expect(normalizeAlarmActive(falsy)).toBe(false)
    }
    for (const truthy of ['Y', '1', 'true', 'ON', 'yes', 2, true]) {
      expect(normalizeAlarmActive(truthy)).toBe(true)
    }
  })

  it('resolves alarm status from the first present legacy field', () => {
    expect(resolveDeviceAlarmActive({ warn_status: 'Y' })).toBe(true)
    expect(resolveDeviceAlarmActive({ warnStatus: 'N', alarm_status: 'Y' })).toBe(false)
    expect(resolveDeviceAlarmActive({ alarmStatus: 1 })).toBe(true)
    expect(resolveDeviceAlarmActive(undefined)).toBe(false)
  })

  it('flags DOM-like change payloads so they do not trigger reloads', () => {
    expect(isDomEventPayload(new Event('change'))).toBe(true)
    expect(isDomEventPayload({ target: null, bubbles: true })).toBe(true)
    expect(isDomEventPayload({ refresh: true })).toBe(false)
    expect(isDomEventPayload(undefined)).toBe(false)
  })
})

describe('useDeviceOnlineStatusSocket', () => {
  it('applies only frames for the current device and ignores pong / non-JSON', () => {
    const socket = useDeviceOnlineStatusSocket(() => 'dev-1')
    socket.applyFrame('pong')
    socket.applyFrame('not-json')
    expect(socket.isOnline.value).toBe(0)

    socket.applyFrame(JSON.stringify({ device_id: 'dev-2', is_online: 1 }))
    expect(socket.isOnline.value).toBe(0)

    capturedOnMessage?.({} as WebSocket, { data: JSON.stringify({ device_id: 'dev-1', is_online: 1 }) } as MessageEvent)
    expect(socket.isOnline.value).toBe(1)
    expect(socket.updatedAt.value).not.toBe('')
    expect(socket.updatedAtDisplay.value).not.toBe('--')

    socket.reset()
    expect(socket.isOnline.value).toBe(0)
    expect(socket.updatedAtDisplay.value).toBe('--')
  })

  it('sends the subscription frame with the stored token', () => {
    const socket = useDeviceOnlineStatusSocket(() => 'dev-1')
    socket.subscribe('dev-9')
    expect(mockSend).toHaveBeenCalledWith(JSON.stringify({ device_id: 'dev-9', token: 'tok' }))
  })
})
