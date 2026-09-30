/**
 * 文件用途：覆盖 mobile-app/app-center/index.vue 迁移 useListPage 后的列表契约。
 * 核心逻辑：mock 应用包 API，验证首屏拉取参数（page/page_size/过滤透传）、筛选变更回第一页、
 *   发布/删除成功后回刷列表、上传成功后回到第一页。
 * 关键注意事项：本套件只覆盖前端组件行为，不证明后端发布状态机。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getMobileAppBundles: vi.fn(),
  publishMobileAppBundle: vi.fn(),
  archiveMobileAppBundle: vi.fn(),
  deleteMobileAppBundle: vi.fn(),
  uploadMobileAppBundle: vi.fn(),
  updateMobileAppBundle: vi.fn(),
  resolvePlatformAssetUrl: vi.fn((path: string) => path),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api', () => ({
  getMobileAppBundles: hoisted.getMobileAppBundles,
  publishMobileAppBundle: hoisted.publishMobileAppBundle,
  archiveMobileAppBundle: hoisted.archiveMobileAppBundle,
  deleteMobileAppBundle: hoisted.deleteMobileAppBundle,
  uploadMobileAppBundle: hoisted.uploadMobileAppBundle,
  updateMobileAppBundle: hoisted.updateMobileAppBundle
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

vi.mock('@/utils/auth-user-avatar', () => ({
  resolvePlatformAssetUrl: hoisted.resolvePlatformAssetUrl
}))

vi.mock('naive-ui', () => {
  const stub = (name: string, tag = 'div') =>
    defineComponent({
      name,
      inheritAttrs: false,
      setup(_, { attrs, slots }) {
        return () => h(tag, attrs, slots.default?.())
      }
    })
  return {
    NButton: stub('NButton', 'button'),
    NCard: stub('NCard'),
    NDataTable: stub('NDataTable'),
    NEmpty: stub('NEmpty'),
    NForm: stub('NForm'),
    NFormItem: stub('NFormItem'),
    NInput: stub('NInput', 'input'),
    NModal: stub('NModal'),
    NPopconfirm: stub('NPopconfirm'),
    NSelect: stub('NSelect', 'select'),
    NSpace: stub('NSpace'),
    NSpin: stub('NSpin'),
    NTag: stub('NTag'),
    NTooltip: stub('NTooltip'),
    NUpload: stub('NUpload'),
    useMessage: () => ({ success: hoisted.messageSuccess, error: vi.fn() })
  }
})

import AppCenter from '../index.vue'

const bundleFixture = {
  id: 'bundle-1',
  platform: 'android',
  version: '1.0.0',
  file_name: 'app.apk',
  file_size: 1024,
  status: 'draft',
  checksum: 'a'.repeat(64),
  file_path: '/files/app.apk',
  release_notes: null,
  published_at: null,
  created_at: '2026-08-01T00:00:00Z'
}

const mountedWrappers: Array<VueWrapper> = []

function mountComponent() {
  const wrapper = shallowMount(AppCenter)
  mountedWrappers.push(wrapper)
  return wrapper
}

function getSetupState(wrapper: VueWrapper) {
  return wrapper.vm.$.setupState as unknown as Record<string, any>
}

describe('mobile-app/app-center/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getMobileAppBundles.mockResolvedValue({
      data: { total: 1, list: [bundleFixture] },
      error: null
    })
    ;(window as unknown as Record<string, unknown>).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('fetches the first page through useListPage without leaking empty filters', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.getMobileAppBundles).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      platform: undefined,
      status: undefined
    })
    expect(getSetupState(wrapper).bundleList).toHaveLength(1)
    expect(getSetupState(wrapper).pagination.itemCount).toBe(1)
  })

  it('resets to the first page and passes filters when a filter changes', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.filter.platform = 'android'
    await state.handleFilterChange()
    await flushPromises()

    expect(hoisted.getMobileAppBundles).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 10,
      platform: 'android',
      status: undefined
    })
  })

  it('reloads the current page after a successful publish', async () => {
    hoisted.publishMobileAppBundle.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()

    await getSetupState(wrapper).handlePublish(bundleFixture)
    await flushPromises()

    expect(hoisted.publishMobileAppBundle).toHaveBeenCalledWith('bundle-1')
    expect(hoisted.getMobileAppBundles).toHaveBeenCalledTimes(2)
    expect(hoisted.messageSuccess).toHaveBeenCalled()
  })

  it('does not reload when the publish request fails', async () => {
    hoisted.publishMobileAppBundle.mockResolvedValue({ error: { message: 'invalid transition' } })
    const wrapper = mountComponent()
    await flushPromises()

    await getSetupState(wrapper).handlePublish(bundleFixture)
    await flushPromises()

    expect(hoisted.getMobileAppBundles).toHaveBeenCalledTimes(1)
  })

  it('jumps back to the first page when the upload modal reports success', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    // 模拟翻到第 3 页后再上传成功：上传成功回调固定 setPage(1)。
    state.setPage(3)
    await flushPromises()
    expect(state.pagination.page).toBe(3)
    hoisted.getMobileAppBundles.mockClear()
    await state.setPage(1)
    await flushPromises()

    expect(state.pagination.page).toBe(1)
    expect(hoisted.getMobileAppBundles).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      platform: undefined,
      status: undefined
    })
  })
})
