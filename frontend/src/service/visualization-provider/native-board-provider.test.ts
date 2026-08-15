import { beforeEach, describe, expect, it, vi } from 'vitest'

const boardApi = vi.hoisted(() => ({
  createBoard: vi.fn(),
  deleteBoard: vi.fn(),
  fetchPublishedBoardByShareToken: vi.fn(),
  fetchBoardById: vi.fn(),
  fetchBoards: vi.fn(),
  publishBoard: vi.fn(),
  updateBoard: vi.fn()
}))

vi.mock('@/service/api/board', () => boardApi)

import {
  NATIVE_BOARD_PROJECT_ID,
  nativeBoardProvider
} from './native-board-provider'

const timestamp = '2026-08-01T00:00:00.000Z'
const config = JSON.stringify({ version: 1, columns: 24, rowHeight: 60, widgets: [] })
const board = (overrides: Record<string, unknown> = {}) => ({
  id: 'board-1',
  name: 'Native board',
  tenant_id: 'tenant-1',
  created_at: timestamp,
  updated_at: timestamp,
  home_flag: 'N',
  config,
  description: 'Description',
  remark: null,
  menu_flag: 'N',
  vis_type: 'native',
  published: false,
  published_at: null,
  share_token: null,
  ...overrides
})

describe('native board visualization provider', () => {
  beforeEach(() => vi.clearAllMocks())

  it('is the local-default provider and keeps external project operations fail-closed', () => {
    expect(nativeBoardProvider).toMatchObject({
      id: 'native-board',
      kind: 'local',
      deploymentMode: 'local-default'
    })
  })

  it('exposes one built-in project and makes project mutation fail closed', async () => {
    expect(await nativeBoardProvider.listProjects()).toMatchObject({
      ok: true,
      data: { items: [{ id: NATIVE_BOARD_PROJECT_ID }], total: 1 }
    })
    expect(await nativeBoardProvider.getProject(NATIVE_BOARD_PROJECT_ID)).toMatchObject({ ok: true })
    expect(await nativeBoardProvider.createProject({ name: 'Other' })).toMatchObject({
      ok: false,
      error: { code: 'unsupported-operation' }
    })
  })

  it('maps paged native boards and preserves home state', async () => {
    boardApi.fetchBoards.mockResolvedValue({ data: { list: [board({ home_flag: 'Y' })], total: 21 }, error: null })

    expect(await nativeBoardProvider.listDashboards({
      projectId: NATIVE_BOARD_PROJECT_ID,
      page: 2,
      limit: 10,
      name: ' Native '
    })).toMatchObject({
      ok: true,
      data: {
        page: 2,
        limit: 10,
        total: 21,
        totalPages: 3,
        items: [{ id: 'board-1', home: true, projectId: NATIVE_BOARD_PROJECT_ID }]
      }
    })
    expect(boardApi.fetchBoards).toHaveBeenCalledWith({ page: 2, page_size: 10, vis_type: 'native', name: 'Native' })
  })

  it('maps list summaries when the paged API omits renderer config', async () => {
    boardApi.fetchBoards.mockResolvedValue({
      data: { list: [board({ config: undefined, description: null })], total: 1 },
      error: null
    })

    await expect(nativeBoardProvider.listDashboards({
      projectId: NATIVE_BOARD_PROJECT_ID,
      page: 1,
      limit: 12
    })).resolves.toMatchObject({
      ok: true,
      data: {
        items: [{ id: 'board-1', name: 'Native board', description: null }]
      }
    })
  })

  it('loads config as renderer data and rejects malformed or non-native boards', async () => {
    boardApi.fetchBoardById.mockResolvedValueOnce({ data: board(), error: null })
    expect(await nativeBoardProvider.getDashboard('board-1')).toMatchObject({
      ok: true,
      data: { id: 'board-1', description: 'Description', version: 1, rendererData: { version: 1, widgets: [] } }
    })

    boardApi.fetchBoardById.mockResolvedValueOnce({ data: board({ config: '{' }), error: null })
    expect(await nativeBoardProvider.getDashboard('board-1')).toMatchObject({ ok: false })

    boardApi.fetchBoardById.mockResolvedValueOnce({ data: board({ vis_type: 'thingsvis' }), error: null })
    expect(await nativeBoardProvider.getDashboard('board-1')).toMatchObject({ ok: false })
  })

  it('creates, updates, duplicates and deletes through the board API', async () => {
    const rendererData = { version: 1, columns: 24, rowHeight: 60, widgets: [] }
    boardApi.createBoard.mockResolvedValueOnce({ data: board(), error: null })
    expect(await nativeBoardProvider.createDashboard({
      name: 'Native board',
      description: 'Description',
      projectId: NATIVE_BOARD_PROJECT_ID,
      rendererData,
      tenantId: ' tenant-2 '
    })).toMatchObject({ ok: true, data: { id: 'board-1', description: 'Description' } })
    expect(boardApi.createBoard).toHaveBeenLastCalledWith(expect.objectContaining({
      name: 'Native board',
      description: 'Description',
      home_flag: 'N',
      menu_flag: 'N',
      vis_type: 'native',
      config: JSON.stringify(rendererData),
      tenant_id: 'tenant-2'
    }))

    boardApi.fetchBoardById.mockResolvedValue({ data: board(), error: null })
    boardApi.updateBoard.mockResolvedValue({ data: board({ name: 'Changed' }), error: null })
    expect(await nativeBoardProvider.updateDashboard('board-1', { name: 'Changed' })).toMatchObject({
      ok: true,
      data: { name: 'Changed' }
    })
    expect(boardApi.updateBoard).toHaveBeenCalledWith(expect.objectContaining({
      id: 'board-1',
      name: 'Changed',
      description: 'Description',
      home_flag: 'N',
      vis_type: 'native'
    }))

    boardApi.createBoard.mockResolvedValueOnce({ data: board({ id: 'board-2', name: 'Native board Copy' }), error: null })
    expect(await nativeBoardProvider.duplicateDashboard('board-1')).toMatchObject({
      ok: true,
      data: { id: 'board-2', name: 'Native board Copy' }
    })

    boardApi.deleteBoard.mockResolvedValue({ data: null, error: null })
    expect(await nativeBoardProvider.deleteDashboard('board-1')).toEqual({ ok: true, data: undefined })
  })

  it('rejects unsupported dashboard fields without calling board write APIs and accepts neutral values', async () => {
    const rendererData = { version: 1, columns: 24, rowHeight: 60, widgets: [] }
    const createPayload = {
      name: 'Native board',
      projectId: NATIVE_BOARD_PROJECT_ID,
      rendererData
    }

    for (const [field, value] of Object.entries({
      canvasConfig: { mode: 'fixed' },
      nodes: [{ id: 'node-1' }],
      dataSources: [{ id: 'source-1' }],
      variables: [{ id: 'variable-1' }]
    })) {
      expect(await nativeBoardProvider.createDashboard({ ...createPayload, [field]: value })).toMatchObject({
        ok: false,
        error: { code: 'unsupported-operation' }
      })
    }
    expect(boardApi.createBoard).not.toHaveBeenCalled()

    for (const [field, value] of Object.entries({
      thumbnail: 'thumbnail.png',
      canvasConfig: { mode: 'fixed' },
      nodes: [{ id: 'node-1' }],
      dataSources: [{ id: 'source-1' }],
      variables: [{ id: 'variable-1' }]
    })) {
      expect(await nativeBoardProvider.updateDashboard('board-1', { [field]: value })).toMatchObject({
        ok: false,
        error: { code: 'unsupported-operation' }
      })
    }
    expect(boardApi.fetchBoardById).not.toHaveBeenCalled()
    expect(boardApi.updateBoard).not.toHaveBeenCalled()

    boardApi.createBoard.mockResolvedValueOnce({ data: board(), error: null })
    expect(await nativeBoardProvider.createDashboard({
      ...createPayload,
      canvasConfig: null,
      nodes: [],
      dataSources: [],
      variables: []
    } as Parameters<typeof nativeBoardProvider.createDashboard>[0])).toMatchObject({ ok: true })

    boardApi.fetchBoardById.mockResolvedValue({ data: board(), error: null })
    boardApi.updateBoard.mockResolvedValue({ data: board({ description: '' }), error: null })
    expect(await nativeBoardProvider.updateDashboard('board-1', {
      description: '',
      thumbnail: null,
      canvasConfig: {},
      nodes: [],
      dataSources: [],
      variables: []
    } as Parameters<typeof nativeBoardProvider.updateDashboard>[1])).toMatchObject({ ok: true })
    expect(boardApi.updateBoard).toHaveBeenCalledWith(expect.objectContaining({ description: '' }))
  })

  it('updates home state with a complete payload and loads the native home board', async () => {
    boardApi.fetchBoardById.mockResolvedValue({ data: board(), error: null })
    boardApi.updateBoard.mockResolvedValue({ data: board({ home_flag: 'Y' }), error: null })

    expect(await nativeBoardProvider.setHomeDashboard('board-1')).toEqual({ ok: true, data: undefined })
    expect(boardApi.updateBoard).toHaveBeenCalledWith(expect.objectContaining({
      id: 'board-1',
      name: 'Native board',
      config,
      description: 'Description',
      home_flag: 'Y',
      vis_type: 'native'
    }))

    boardApi.fetchBoards.mockResolvedValue({ data: { list: [board({ home_flag: 'Y' })], total: 1 }, error: null })
    expect(await nativeBoardProvider.getHomeDashboard()).toMatchObject({ ok: true, data: { id: 'board-1' } })
    expect(boardApi.fetchBoards).toHaveBeenCalledWith({ page: 1, page_size: 1, home_flag: 'Y', vis_type: 'native' })
  })

  it('passes an explicit tenant context when loading a SYS_ADMIN home board', async () => {
    boardApi.fetchBoards.mockResolvedValue({
      data: { list: [board({ home_flag: 'Y', tenant_id: 'tenant-1' })], total: 1 },
      error: null
    })

    await expect(nativeBoardProvider.getHomeDashboard({ tenantId: 'tenant-1' })).resolves.toMatchObject({
      ok: true,
      data: { id: 'board-1', tenantId: 'tenant-1' }
    })
    expect(boardApi.fetchBoards).toHaveBeenCalledWith({
      page: 1,
      page_size: 1,
      home_flag: 'Y',
      vis_type: 'native',
      tenant_id: 'tenant-1'
    })
  })

  it('publishes native boards and loads them through a public share token', async () => {
    boardApi.fetchBoardById.mockResolvedValue({ data: board(), error: null })
    boardApi.publishBoard.mockResolvedValue({
      data: board({ published: true, published_at: timestamp, share_token: 'share-token-1' }),
      error: null
    })

    await expect(nativeBoardProvider.publishDashboard('board-1')).resolves.toMatchObject({
      ok: true,
      data: { id: 'board-1', published: true, publishedAt: timestamp, shareToken: 'share-token-1' }
    })
    expect(boardApi.publishBoard).toHaveBeenCalledWith('board-1')

    boardApi.fetchPublishedBoardByShareToken.mockResolvedValue({
      data: board({ published: true, published_at: timestamp, share_token: 'share-token-1' }),
      error: null
    })
    await expect(nativeBoardProvider.getDashboardByShareToken?.('share-token-1')).resolves.toMatchObject({
      ok: true,
      data: { published: true, shareToken: 'share-token-1' }
    })
    expect(boardApi.fetchPublishedBoardByShareToken).toHaveBeenCalledWith('share-token-1')
  })

  it('rejects invalid configs and API failures', async () => {
    expect(await nativeBoardProvider.createDashboard({
      name: 'Unsafe',
      projectId: NATIVE_BOARD_PROJECT_ID,
      rendererData: { version: 1, widgets: [{ id: 'x', x: 0, y: 0, w: 1, h: 1, type: 'text', config: { text: 'https://remote.test' } }] }
    })).toMatchObject({ ok: false })

    boardApi.fetchBoards.mockResolvedValue({ data: null, error: { message: 'offline' } })
    expect(await nativeBoardProvider.listDashboards({ projectId: NATIVE_BOARD_PROJECT_ID })).toMatchObject({
      ok: false,
      error: { code: 'provider-failure' }
    })
  })
})
