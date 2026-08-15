import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: {
    get: mockGet,
    post: mockPost,
    put: mockPut,
    delete: mockDelete
  }
}))

import { createBoard, deleteBoard, fetchBoardById, fetchBoards, updateBoard } from '../board'

describe('native board API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches a board by an encoded ID', async () => {
    await fetchBoardById('board/one two')

    expect(mockGet).toHaveBeenCalledWith('/board/board%2Fone%20two')
  })

  it('lists only the requested visualization type', async () => {
    const params = { page: 1, page_size: 20, vis_type: 'native' }

    await fetchBoards(params)

    expect(mockGet).toHaveBeenCalledWith('/board', { params })
  })

  it('sends the explicitly selected tenant in the native board create payload', async () => {
    const payload = {
      name: 'Native board',
      config: '{"version":1,"widgets":[]}',
      home_flag: 'N',
      menu_flag: 'N',
      vis_type: 'native',
      tenant_id: 'tenant-1'
    }

    await createBoard(payload)

    expect(mockPost).toHaveBeenCalledWith('/board', payload)
    expect(payload.tenant_id).toBe('tenant-1')
  })

  it('updates an existing board through the explicit ID payload', async () => {
    const payload = {
      id: 'board-1',
      name: 'Updated board',
      config: '{"version":1,"widgets":[]}',
      vis_type: 'native'
    }

    await updateBoard(payload)

    expect(mockPut).toHaveBeenCalledWith('/board', payload)
  })

  it('deletes a board by an encoded ID', async () => {
    await deleteBoard('board/one two')

    expect(mockDelete).toHaveBeenCalledWith('/board/board%2Fone%20two')
  })
})
