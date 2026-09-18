/**
 * 文件用途：解决方案模板引擎（ROADMAP TB-19）管理视图单元测试。
 * 核心逻辑：验证列表加载、创建方案、一键安装结果展示与删除交互契约。
 */
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import SolutionsManagementView from '../index.vue'

const mocks = vi.hoisted(() => ({
  listIndustrySolutions: vi.fn(),
  createIndustrySolution: vi.fn(),
  getIndustrySolution: vi.fn(),
  installIndustrySolution: vi.fn(),
  deleteIndustrySolution: vi.fn()
}))

vi.mock('@/service/api/solution', () => mocks)

// useDialog 需要可观察的桩：把 warning(options) 存到 window 上供用例触发确认回调。
vi.mock('naive-ui', async importOriginal => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useDialog: () => ({
      warning: (options: { title?: string; content?: string; positiveText?: string; negativeText?: string; onPositiveClick?: () => void | Promise<void> }) => {
        (globalThis as any).__dialogWarnings = [
          ...(((globalThis as any).__dialogWarnings as any[]) || []),
          options
        ]
      }
    })
  }
})

const fixtureSolution = {
  id: 'sol-1',
  tenant_id: 't-1',
  name: '水务监测方案',
  description: 'demo',
  resources: [
    { resource_type: 'device_template', resource_id: 'tpl-1' },
    { resource_type: 'board_template', resource_id: 'board-1', target_name: '水务看板' }
  ],
  status: 'active',
  created_at: '2026-09-19T00:00:00Z',
  updated_at: '2026-09-19T00:00:00Z'
}

const mountView = () => mount(SolutionsManagementView)

describe('SolutionsManagementView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listIndustrySolutions.mockResolvedValue({
      data: { total: 1, list: [fixtureSolution] },
      error: null
    })
    mocks.getIndustrySolution.mockResolvedValue({
      data: {
        solution: fixtureSolution,
        installs: [
          {
            id: 'inst-1',
            tenant_id: 't-1',
            solution_id: 'sol-1',
            solution_name: '水务监测方案',
            item_index: 0,
            resource_type: 'device_template',
            resource_id: 'tpl-1',
            target_id: 'tpl-new',
            status: 'applied',
            created_at: '2026-09-19T01:00:00Z'
          }
        ]
      },
      error: null
    })
    mocks.installIndustrySolution.mockResolvedValue({
      data: {
        solution_id: 'sol-1',
        solution_name: '水务监测方案',
        total: 2,
        applied: 1,
        failed: 1,
        items: [
          { item_index: 0, resource_type: 'device_template', resource_id: 'tpl-1', status: 'applied', target_id: 'tpl-new' },
          { item_index: 1, resource_type: 'board_template', resource_id: 'board-1', status: 'failed', error: 'conflict' }
        ]
      },
      error: null
    })
    mocks.createIndustrySolution.mockResolvedValue({ data: fixtureSolution, error: null })
    mocks.deleteIndustrySolution.mockResolvedValue({ data: { deleted: true }, error: null })
  })

  it('loads the solution list on mount', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(mocks.listIndustrySolutions).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.solutions.length).toBe(1)
    expect(state.solutions[0].name).toBe('水务监测方案')
  })

  it('shows per-item install outcome and refuses install result fabrication on partial failure', async () => {
    const wrapper = mountView()
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>

    ;(globalThis as any).__dialogWarnings = []

    await state.openInstall(fixtureSolution)
    // openInstall 只弹确认框，未确认前不打安装请求。
    expect(mocks.installIndustrySolution).not.toHaveBeenCalled()
    const warnings = (globalThis as any).__dialogWarnings as any[]
    expect(warnings.length, 'install must raise a confirm dialog').toBe(1)

    // 模拟用户确认：直接调用 onPositiveClick。
    await warnings[warnings.length - 1].onPositiveClick()
    await flushPromises()

    expect(mocks.installIndustrySolution).toHaveBeenCalledWith('sol-1')
    const result = state.installResult
    expect(result.applied).toBe(1)
    expect(result.failed).toBe(1)
    expect(result.items[1].status).toBe('failed')
    expect(result.items[1].error).toBe('conflict')
  })

  it('creates a solution only with trimmed, usable resource refs', async () => {
    const wrapper = mountView()
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>

    state.openCreate()
    state.createForm.name = '  新方案  '
    state.createForm.resources = [
      { resource_type: 'device_template', resource_id: '  tpl-1  ', target_name: '' }
    ]
    await state.submitCreate()
    await flushPromises()

    expect(mocks.createIndustrySolution).toHaveBeenCalledWith({
      name: '新方案',
      description: undefined,
      resources: [{ resource_type: 'device_template', resource_id: 'tpl-1' }]
    })
    expect(state.createVisible).toBe(false)
    expect(mocks.listIndustrySolutions).toHaveBeenCalledTimes(2)
  })

  it('rejects create without a name or usable resource before hitting the API', async () => {
    const wrapper = mountView()
    await flushPromises()
    const state = wrapper.vm.$.setupState as Record<string, any>

    state.openCreate()
    state.createForm.name = '  '
    await state.submitCreate()
    expect(mocks.createIndustrySolution).not.toHaveBeenCalled()

    state.createForm.name = '有名字'
    state.createForm.resources = [{ resource_type: 'device_template', resource_id: '   ' }]
    await state.submitCreate()
    expect(mocks.createIndustrySolution).not.toHaveBeenCalled()
  })
})
