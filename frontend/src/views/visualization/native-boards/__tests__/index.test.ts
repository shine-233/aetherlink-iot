import { defineComponent, h, nextTick, reactive } from 'vue'
import { flushPromises, shallowMount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  execute: vi.fn(),
  listDashboards: vi.fn(),
  createDashboard: vi.fn(),
  deleteDashboard: vi.fn(),
  publishDashboard: vi.fn(),
  fetchUserList: vi.fn(),
  writeClipboardText: vi.fn(),
  routerPushByKey: vi.fn(),
  message: { error: vi.fn(), success: vi.fn() },
  userInfo: undefined as unknown as { authority: string; roles: string[] }
}))

vi.mock('@/service/visualization-provider/composition', () => ({
  getDefaultVisualizationProviderFacade: () => ({ execute: hoisted.execute })
}))
vi.mock('@/service/api/auth', () => ({ fetchUserList: hoisted.fetchUserList }))
vi.mock('@/hooks/common/router', () => ({ useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey }) }))
vi.mock('@/store/modules/auth', () => ({ useAuthStore: () => ({ userInfo: hoisted.userInfo }) }))
vi.mock('@/locales', () => ({ $t: (key: string) => key }))
vi.mock('@/utils/clipboard', () => ({ writeClipboardText: hoisted.writeClipboardText }))
vi.mock('naive-ui', () => {
  const container = (name: string) =>
    defineComponent({
      name,
      inheritAttrs: false,
      setup(_, { attrs, slots }) {
        return () => h('div', attrs, slots.default?.())
      }
    })
  const button = defineComponent({
    name: 'NButton',
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      return () => h('button', attrs, slots.default?.())
    }
  })
  const input = defineComponent({
    name: 'NInput',
    props: ['value'],
    emits: ['update:value', 'keyup'],
    inheritAttrs: false,
    setup(props, { attrs, emit }) {
      return () =>
        h('input', {
          ...attrs,
          value: props.value,
          onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value),
          onKeyup: (event: KeyboardEvent) => emit('keyup', event)
        })
    }
  })
  const select = defineComponent({
    name: 'NSelect',
    props: ['value', 'options', 'loading'],
    emits: ['update:value'],
    inheritAttrs: false,
    setup(props, { attrs, emit }) {
      return () =>
        h(
          'select',
          {
            ...attrs,
            value: props.value ?? '',
            onChange: (event: Event) => emit('update:value', (event.target as HTMLSelectElement).value)
          },
          (props.options as Array<{ label: string; value: string }> | undefined)?.map((option) =>
            h('option', { value: option.value }, option.label)
          )
        )
    }
  })
  const pagination = defineComponent({
    name: 'NPagination',
    props: ['page', 'pageSize', 'itemCount'],
    emits: ['update:page'],
    inheritAttrs: false,
    setup(props, { attrs, emit }) {
      return () => h('button', { ...attrs, onClick: () => emit('update:page', Number(props.page) + 1) }, 'next')
    }
  })
  const popconfirm = defineComponent({
    name: 'NPopconfirm',
    emits: ['positiveClick'],
    inheritAttrs: false,
    setup(_, { attrs, emit, slots }) {
      return () =>
        h('div', attrs, [
          slots.trigger?.(),
          slots.default?.(),
          h(
            'button',
            {
              'data-testid': 'native-board-delete-confirm',
              onClick: () => emit('positiveClick')
            },
            'confirm'
          )
        ])
    }
  })
  return {
    createDiscreteApi: () => ({ message: hoisted.message, notification: {}, dialog: {}, loadingBar: {} }),
    NButton: button,
    NInput: input,
    NSelect: select,
    NPagination: pagination,
    NPopconfirm: popconfirm,
    NCard: container('NCard'),
    NEmpty: container('NEmpty'),
    NForm: container('NForm'),
    NFormItem: container('NFormItem'),
    NGrid: container('NGrid'),
    NGridItem: container('NGridItem'),
    NModal: container('NModal'),
    NSpin: container('NSpin'),
    NTag: container('NTag'),
    useMessage: () => hoisted.message
  }
})

import NativeBoards from '../index.vue'

const summary = (overrides: Record<string, unknown> = {}) => ({
  id: 'board-1',
  name: 'Native board',
  description: 'Description',
  thumbnail: null,
  version: 1,
  published: false,
  publishedAt: null,
  shareToken: null,
  home: false,
  projectId: 'native-boards',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
  ...overrides
})
const schema = (overrides: Record<string, unknown> = {}) => ({
  ...summary(),
  canvasConfig: { mode: 'responsive', width: 1920, height: 1080, background: null },
  nodes: [],
  dataSources: [],
  rendererData: { version: 1, columns: 24, rowHeight: 60, widgets: [] },
  ...overrides
})
const pageResult = (items = [summary()], total = items.length) => ({
  ok: true as const,
  data: { items, page: 1, limit: 12, total, totalPages: total ? Math.ceil(total / 12) : 0 }
})
const success = <T>(data: T) => ({ ok: true as const, data })
const failure = () => ({ ok: false as const, error: { code: 'provider-failure' as const, message: 'failed' } })
const wrappers: VueWrapper[] = []

/**
 * beforeEach 里 fetchUserList 只返回一个 TENANT_ADMIN 行（tenant_id = tenant-1），
 * 于是 loadTenantOptions 会把该租户自动选中并写进列表查询 —— 首屏之后的所有列表请求
 * 都会带上 tenantId。创建流程的租户断言见 modules/__tests__/native-board-create-modal.test.ts。
 */
const TENANT_ID = 'tenant-1'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function mountPage(authority = 'SYS_ADMIN', roles: string[] = []) {
  hoisted.userInfo = reactive({ authority, roles })
  const wrapper = shallowMount(NativeBoards)
  wrappers.push(wrapper)
  return wrapper
}

function vm(wrapper: VueWrapper) {
  return wrapper.vm as unknown as {
    boards: ReturnType<typeof summary>[]
    total: number
    loading: boolean
    failed: boolean
    page: number
    searchInput: string
    listFilter: { name: string; tenantId: string }
    showCreateModal: boolean
    createModalPrefill: string | null
    deletingBoardId: string | null
    loadBoards: () => Promise<void>
    handleSearch: () => void
    handlePageChange: (page: number) => void
    openBoard: (id: string) => void
    editBoard: (id: string) => void
    openCreateModal: () => void
    handleCreated: (id: string) => void
    handleDelete: (id: string) => Promise<void>
    handlePublish: (id: string) => Promise<void>
    handleCopyLink: (board: ReturnType<typeof summary>) => Promise<void>
  }
}

describe('native boards page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.listDashboards.mockResolvedValue(pageResult())
    hoisted.createDashboard.mockResolvedValue(success(schema({ id: 'created-1' })))
    hoisted.deleteDashboard.mockResolvedValue(success(undefined))
    hoisted.publishDashboard.mockResolvedValue(success(summary({ published: true, shareToken: 'share-token' })))
    hoisted.writeClipboardText.mockResolvedValue(true)
    hoisted.fetchUserList.mockResolvedValue({
      data: {
        list: [
          {
            id: 'tenant-admin-1',
            name: 'Tenant admin',
            email: 'tenant-admin@example.com',
            authority: 'TENANT_ADMIN',
            tenant_id: 'tenant-1'
          }
        ],
        total: 1
      }
    })
    hoisted.execute.mockImplementation(
      (
        operation: (provider: {
          listDashboards: typeof hoisted.listDashboards
          createDashboard: typeof hoisted.createDashboard
          deleteDashboard: typeof hoisted.deleteDashboard
          publishDashboard: typeof hoisted.publishDashboard
        }) => unknown
      ) =>
        operation({
          listDashboards: hoisted.listDashboards,
          createDashboard: hoisted.createDashboard,
          deleteDashboard: hoisted.deleteDashboard,
          publishDashboard: hoisted.publishDashboard
        })
    )
  })

  afterEach(() => {
    wrappers.forEach((wrapper) => wrapper.unmount())
    wrappers.length = 0
  })

  it('loads native summaries and renders neutral fields', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(hoisted.listDashboards).toHaveBeenCalledWith({ projectId: 'native-boards', page: 1, limit: 12 })
    expect(wrapper.text()).toContain('Native board')
    expect(wrapper.text()).toContain('Description')
    expect(wrapper.text()).toContain('2026-08-01T00:00:00Z')
    expect(wrapper.text()).toContain('custom.nativeBoards.no')
  })

  it('trims search, resets page, and keeps the active name on pagination', async () => {
    const wrapper = mountPage()
    await flushPromises()
    vm(wrapper).page = 3
    vm(wrapper).searchInput = '  factory  '
    vm(wrapper).handleSearch()
    await flushPromises()
    // SYS_ADMIN + 仅一个租户时，loadTenantOptions 会自动选中该租户并写进列表查询，
    // 所以首屏之后的每次列表请求都带 tenantId。
    expect(hoisted.listDashboards).toHaveBeenLastCalledWith({
      projectId: 'native-boards',
      page: 1,
      limit: 12,
      name: 'factory',
      tenantId: TENANT_ID
    })
    vm(wrapper).handlePageChange(2)
    await flushPromises()
    expect(hoisted.listDashboards).toHaveBeenLastCalledWith({
      projectId: 'native-boards',
      page: 2,
      limit: 12,
      name: 'factory',
      tenantId: TENANT_ID
    })
  })

  it('opens viewer and permits editor only for admins', async () => {
    const wrapper = mountPage()
    await flushPromises()
    vm(wrapper).openBoard('board-1')
    vm(wrapper).editBoard('board-1')
    expect(hoisted.routerPushByKey).toHaveBeenNthCalledWith(1, 'visualization_native-board', {
      query: { id: 'board-1' }
    })
    expect(hoisted.routerPushByKey).toHaveBeenNthCalledWith(2, 'visualization_native-board-editor', {
      query: { id: 'board-1' }
    })
  })

  it.each([
    ['TENANT_USER', []],
    ['', ['OTHER']]
  ])('guards create, edit, and delete for non-admin users', async (authority, roles) => {
    const wrapper = mountPage(authority, roles)
    await flushPromises()
    vm(wrapper).editBoard('board-1')
    // 创建守卫在 openCreateModal（非管理员直接 return，弹窗不开）；表单校验与提交已下沉到
    // NativeBoardCreateModal，由该组件的专属测试守护。
    vm(wrapper).openCreateModal()
    await vm(wrapper).handleDelete('board-1')
    expect(vm(wrapper).showCreateModal).toBe(false)
    expect(hoisted.routerPushByKey).not.toHaveBeenCalled()
    expect(hoisted.createDashboard).not.toHaveBeenCalled()
    expect(hoisted.deleteDashboard).not.toHaveBeenCalled()
  })

  // 创建流程（表单校验 / 租户必选 / 提交去重 / 失败保持弹窗）已下沉到
  // modules/native-board-create-modal.vue，由该组件的专属测试守护。
  // 页面这边只保留两条契约：打开时注入租户 prefill，创建成功后路由到查看器。
  it('opens the create modal with the active tenant prefilled', async () => {
    const wrapper = mountPage()
    await flushPromises()
    vm(wrapper).openCreateModal()
    expect(vm(wrapper).showCreateModal).toBe(true)
    expect(vm(wrapper).createModalPrefill).toBe(TENANT_ID)
  })

  it('routes to the viewer when the modal reports a created board', async () => {
    const wrapper = mountPage()
    await flushPromises()
    vm(wrapper).handleCreated('created-1')
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('visualization_native-board', { query: { id: 'created-1' } })
  })

  it('deletes through the provider and reloads the active page', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await vm(wrapper).handleDelete('board-1')
    expect(hoisted.deleteDashboard).toHaveBeenCalledWith('board-1')
    expect(hoisted.listDashboards).toHaveBeenCalledTimes(2)
    expect(hoisted.message.success).toHaveBeenCalledWith('common.deleteSuccess')
  })

  it('publishes a native board through the provider and reloads the active page', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await vm(wrapper).handlePublish('board-1')
    expect(hoisted.publishDashboard).toHaveBeenCalledWith('board-1')
    expect(hoisted.listDashboards).toHaveBeenCalledTimes(2)
    expect(hoisted.message.success).toHaveBeenCalledWith('rdi.thingsvis.publishSuccess')
  })

  it('copies the published native board viewer link', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const published = summary({ published: true, shareToken: 'share-token' })
    await vm(wrapper).handleCopyLink(published)
    expect(hoisted.writeClipboardText).toHaveBeenCalledWith(
      expect.stringContaining('/tv-preview?id=board-1&projectId=native-boards&provider=native&shareToken=share-token')
    )
    expect(hoisted.message.success).toHaveBeenCalledWith('rdi.thingsvis.copyLinkSuccess')
  })

  it('moves to the previous page when deleting the last item', async () => {
    const wrapper = mountPage()
    await flushPromises()
    vm(wrapper).page = 2
    await vm(wrapper).handleDelete('board-1')
    expect(vm(wrapper).page).toBe(1)
    expect(hoisted.listDashboards).toHaveBeenLastCalledWith({
      projectId: 'native-boards',
      page: 1,
      limit: 12,
      tenantId: TENANT_ID
    })
  })

  it('keeps the list when delete fails and prevents duplicate deletes', async () => {
    const pending = deferred<ReturnType<typeof failure>>()
    hoisted.deleteDashboard.mockReturnValue(pending.promise)
    const wrapper = mountPage()
    await flushPromises()
    const current = [...vm(wrapper).boards]
    const first = vm(wrapper).handleDelete('board-1')
    const second = vm(wrapper).handleDelete('board-1')
    expect(hoisted.deleteDashboard).toHaveBeenCalledTimes(1)
    pending.resolve(failure())
    await Promise.all([first, second])
    expect(vm(wrapper).boards).toEqual(current)
    expect(hoisted.listDashboards).toHaveBeenCalledTimes(1)
    expect(hoisted.message.error).toHaveBeenCalledWith('common.deleteFailed')
  })

  it.each([
    ['provider failure', failure()],
    ['thrown failure', 'throw']
  ])('fails list closed for %s', async (_label, mode) => {
    if (mode === 'throw') hoisted.execute.mockRejectedValueOnce(new Error('failed'))
    else hoisted.listDashboards.mockResolvedValue(mode)
    const wrapper = mountPage()
    await flushPromises()
    expect(vm(wrapper).boards).toEqual([])
    expect(vm(wrapper).total).toBe(0)
    expect(vm(wrapper).failed).toBe(true)
    expect(vm(wrapper).loading).toBe(false)
  })

  it('keeps the current page visible while a reload is in flight', async () => {
    const pending = deferred<ReturnType<typeof pageResult>>()
    const wrapper = mountPage()
    await flushPromises()
    expect(vm(wrapper).boards).toHaveLength(1)

    hoisted.listDashboards.mockReturnValueOnce(pending.promise)
    const loading = vm(wrapper).loadBoards()

    // useListPage 的 load() 不再在请求前清空 rows（避免翻页/刷新时列表闪空），
    // 只有 clear() 才清空；被取代的响应交给 seq 门禁丢弃。
    expect(vm(wrapper).boards).toHaveLength(1)
    expect(vm(wrapper).loading).toBe(true)

    pending.resolve(pageResult())
    await loading
    expect(vm(wrapper).loading).toBe(false)
  })

  it('ignores stale success and stale failure without stopping the current load', async () => {
    const older = deferred<ReturnType<typeof pageResult>>()
    const newer = deferred<ReturnType<typeof pageResult>>()
    hoisted.listDashboards.mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise)
    const wrapper = mountPage()
    vm(wrapper).handlePageChange(2)
    await nextTick()
    older.resolve(pageResult([summary({ id: 'old', name: 'Old' })]))
    await flushPromises()
    expect(vm(wrapper).boards).toEqual([])
    expect(vm(wrapper).loading).toBe(true)
    newer.resolve(pageResult([summary({ id: 'new', name: 'New' })]))
    await flushPromises()
    expect(vm(wrapper).boards[0].id).toBe('new')
    expect(vm(wrapper).loading).toBe(false)
  })

  // 旧实现里页面自带"查询快照门禁"（查询变了但没发请求时，丢弃迟到响应）。
  // 迁到 useListPage 后该门禁由序号门禁取代，且页面只在提交时才改查询，
  // 中间不存在"查询变了却没发请求"的窗口；序号门禁本身由 useListPage 的
  // 单测（drops stale responses and aborts the previous request）与上面的
  // stale 用例共同守护。这里改为守护页面侧的真实契约：输入框不提交就不生效。
  it('only applies the search box to the query on submit', async () => {
    const wrapper = mountPage()
    await flushPromises()
    const callsBefore = hoisted.listDashboards.mock.calls.length

    vm(wrapper).searchInput = 'changed-without-submit'
    await flushPromises()

    expect(hoisted.listDashboards).toHaveBeenCalledTimes(callsBefore)
    expect(vm(wrapper).listFilter.name).toBe('')
  })
})
