/*
 * 文件用途：验证版本信息 Hook 的本地版本、缓存和可选公网检查边界。
 * 核心逻辑：挂载最小组件触发 onMounted，并观察平台版本与最新版本响应式值。
 * 关键注意事项：默认部署不得访问 GitHub；只有显式环境开关才能启用远端检查。
 */
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { getSysVersionMock } = vi.hoisted(() => ({
  getSysVersionMock: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  getSysVersion: getSysVersionMock
}))

const CACHE_KEY = 'aetherlink_iot_latest_version_cache_v1'

async function mountVersionInfo(waitForLoad = true) {
  const { default: useVersionInfo } = await import('../business/use-version-info')
  const wrapper = mount(defineComponent({
    setup() {
      const { currentVersion, latestVersion } = useVersionInfo()
      return () => h('div', [
        h('span', { 'data-testid': 'current' }, currentVersion.value),
        h('span', { 'data-testid': 'latest' }, latestVersion.value)
      ])
    }
  }))
  if (waitForLoad) await flushPromises()
  return wrapper
}

describe('useVersionInfo remote boundary', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.unstubAllEnvs()
    localStorage.clear()
    getSysVersionMock.mockReset()
    getSysVersionMock.mockResolvedValue({ data: { version: 'v1.2.3' } })
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
  })

  it('does not access GitHub by default', async () => {
    const wrapper = await mountVersionInfo()

    expect(fetch).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="current"]').text()).toBe('1.2.3')
    expect(wrapper.get('[data-testid="latest"]').text()).toBe('--')
  })

  it('queries GitHub only when explicitly enabled', async () => {
    vi.stubEnv('VITE_ENABLE_REMOTE_VERSION_CHECK', 'Y')
    vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify([{ name: 'v2.0.0' }]), { status: 200 }))

    const wrapper = await mountVersionInfo()

    expect(fetch).toHaveBeenCalledOnce()
    expect(fetch).toHaveBeenCalledWith('https://api.github.com/repos/shine-233/aetherlink-iot/tags', {
      signal: expect.any(AbortSignal)
    })
    expect(wrapper.get('[data-testid="latest"]').text()).toBe('2.0.0')
  })

  it('uses a valid local cache without accessing GitHub', async () => {
    vi.stubEnv('VITE_ENABLE_REMOTE_VERSION_CHECK', 'Y')
    localStorage.setItem(CACHE_KEY, JSON.stringify({ version: 'v1.9.0', expiresAt: Date.now() + 60_000 }))

    const wrapper = await mountVersionInfo()

    expect(fetch).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="latest"]').text()).toBe('1.9.0')
  })

  it('keeps the platform version available when the optional request fails', async () => {
    vi.stubEnv('VITE_ENABLE_REMOTE_VERSION_CHECK', 'Y')
    vi.mocked(fetch).mockRejectedValue(new Error('offline'))

    const wrapper = await mountVersionInfo()

    expect(wrapper.get('[data-testid="current"]').text()).toBe('1.2.3')
    expect(wrapper.get('[data-testid="latest"]').text()).toBe('--')
  })

  it('times out a stalled optional request and allows a later retry', async () => {
    vi.useFakeTimers()
    vi.stubEnv('VITE_ENABLE_REMOTE_VERSION_CHECK', 'Y')
    vi.mocked(fetch)
      .mockImplementationOnce(() => new Promise<Response>(() => undefined))
      .mockResolvedValueOnce(new Response(JSON.stringify([{ name: 'v2.1.0' }]), { status: 200 }))

    const firstWrapper = await mountVersionInfo(false)
    expect(fetch).toHaveBeenCalledOnce()
    const firstSignal = vi.mocked(fetch).mock.calls[0]?.[1]?.signal

    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()

    expect(firstSignal?.aborted).toBe(true)
    expect(firstWrapper.get('[data-testid="current"]').text()).toBe('1.2.3')
    expect(firstWrapper.get('[data-testid="latest"]').text()).toBe('--')

    const secondWrapper = await mountVersionInfo()
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(secondWrapper.get('[data-testid="latest"]').text()).toBe('2.1.0')

    vi.useRealTimers()
  })
})
