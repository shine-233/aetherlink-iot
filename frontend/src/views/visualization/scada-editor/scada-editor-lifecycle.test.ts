/**
 * 文件用途：SCADA 编辑器生命周期回归测试。
 * 核心逻辑：验证陈旧判定时钟（5s setInterval）在组件卸载后被清理，
 *           防止每次进出路由都泄漏一个永不停止的定时器。
 */
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({ userInfo: { authority: 'TENANT_ADMIN' } })
}))

vi.mock('@/service/api/scada', () => ({
  archiveScadaDocument: vi.fn(),
  createScadaDocument: vi.fn(),
  createScadaProject: vi.fn(),
  executeControl: vi.fn(),
  fetchScadaDocument: vi.fn(),
  fetchScadaDocumentVersions: vi.fn(),
  fetchScadaDocuments: vi.fn().mockResolvedValue({ data: [] }),
  fetchScadaProjects: vi.fn().mockResolvedValue({ data: [] }),
  issueControlConfirmation: vi.fn(),
  publishScadaDocument: vi.fn(),
  rollbackScadaDocument: vi.fn(),
  saveScadaDocument: vi.fn()
}))

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() })
  }
})

import ScadaEditor from './index.vue'

describe('scada-editor lifecycle', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('clears the stale-clock interval on unmount', async () => {
    const setSpy = vi.spyOn(globalThis, 'setInterval')
    const clearSpy = vi.spyOn(globalThis, 'clearInterval')
    const wrapper = shallowMount(ScadaEditor)
    await flushPromises()

    const clockCall = setSpy.mock.results.find((_, i) => setSpy.mock.calls[i][1] === 5000)
    expect(clockCall).toBeDefined()

    wrapper.unmount()

    expect(clearSpy).toHaveBeenCalledWith(clockCall!.value)
  })
})
