/**
 * 文件用途：覆盖 management/entity-version/index.vue 迁移 useListPage 后的列表契约。
 * 核心逻辑：mock 实体版本 API，验证 entity_id 缺失时不发起请求、查询参数透传分页、
 *   建快照与恢复成功后回刷列表。
 * 关键注意事项：本套件只覆盖前端组件行为，不证明后端快照/恢复语义。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  entityVersionList: vi.fn(),
  entityVersionCreate: vi.fn(),
  entityVersionGet: vi.fn(),
  entityVersionRestore: vi.fn(),
  entityVersionDiff: vi.fn(),
  messageWarning: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api', () => ({
  entityVersionList: hoisted.entityVersionList,
  entityVersionCreate: hoisted.entityVersionCreate,
  entityVersionGet: hoisted.entityVersionGet,
  entityVersionRestore: hoisted.entityVersionRestore,
  entityVersionDiff: hoisted.entityVersionDiff
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => ({
      warning: hoisted.messageWarning,
      success: hoisted.messageSuccess,
      error: vi.fn()
    })
  }
})

function tagStub(tag = 'div') {
  return defineComponent({
    name: 'TagStub',
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      return () => h(tag, attrs, slots.default?.())
    }
  })
}

import EntityVersionPage from '../index.vue'

const versionFixture = {
  id: 'ver-1',
  version_number: 1,
  entity_type: 'board',
  entity_id: 'board-1',
  remark: 'first',
  snapshot: '{}',
  created_at: '2026-08-01T00:00:00Z'
}

const mountedWrappers: Array<VueWrapper> = []

function mountComponent() {
  const wrapper = shallowMount(EntityVersionPage, {
    global: {
      stubs: {
        'n-card': tagStub(),
        'n-select': tagStub('select'),
        'n-input': tagStub('input'),
        'n-button': tagStub('button'),
        'n-data-table': tagStub(),
        'n-empty': tagStub(),
        'n-modal': tagStub(),
        'n-spin': tagStub(),
        'n-code': tagStub(),
        VersionCompareModal: tagStub()
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function getSetupState(wrapper: VueWrapper) {
  return wrapper.vm.$.setupState as unknown as Record<string, any>
}

describe('management/entity-version/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.entityVersionList.mockResolvedValue({
      data: { total: 1, list: [{ ...versionFixture }] },
      error: null
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('does not request the list when entity_id is missing', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.entityVersionList).not.toHaveBeenCalled()

    await getSetupState(wrapper).getTableData()
    await flushPromises()

    expect(hoisted.entityVersionList).not.toHaveBeenCalled()
    expect(hoisted.messageWarning).toHaveBeenCalled()
  })

  it('fetches versions with entity context and pagination through useListPage', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.filter.entity_id = 'board-1'
    await state.getTableData()
    await flushPromises()

    expect(hoisted.entityVersionList).toHaveBeenCalledWith({
      entity_type: 'board',
      entity_id: 'board-1',
      page: 1,
      page_size: 10
    })
    expect(state.tableData).toHaveLength(1)
    expect(state.pagination.itemCount).toBe(1)
  })

  it('creates a snapshot with the trimmed entity id and reloads', async () => {
    hoisted.entityVersionCreate.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.filter.entity_id = '  board-1  '
    await state.handleCreateSnapshot()
    await flushPromises()

    expect(hoisted.entityVersionCreate).toHaveBeenCalledWith({ entity_type: 'board', entity_id: 'board-1' })
    expect(hoisted.entityVersionList).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalled()
  })

  it('does not create a snapshot when entity_id is missing', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    await getSetupState(wrapper).handleCreateSnapshot()
    await flushPromises()

    expect(hoisted.entityVersionCreate).not.toHaveBeenCalled()
    expect(hoisted.messageWarning).toHaveBeenCalled()
  })

  it('restores a version through the confirm action and reloads', async () => {
    hoisted.entityVersionRestore.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.filter.entity_id = 'board-1'
    await state.getTableData()
    await flushPromises()

    await state.handleRestore(state.tableData[0])
    await flushPromises()

    expect(hoisted.entityVersionRestore).toHaveBeenCalledWith('ver-1', false)
    expect(hoisted.entityVersionList).toHaveBeenCalledTimes(2)
  })
})
