import { request } from '../request'

export interface BoardDetail {
  id: string
  name: string
  tenant_id: string
  created_at: string
  updated_at: string
  home_flag: string
  config: string | null
  description: string | null
  remark: string | null
  menu_flag: string | null
  vis_type: string | null
  published?: boolean
  published_at?: string | null
  share_token?: string | null
}

export type BoardVisualizationType = 'native' | 'thingsvis'

export interface BoardListParams {
  page: number
  page_size: number
  name?: string
  home_flag?: string
  vis_type?: BoardVisualizationType
  tenant_id?: string
}

export interface BoardListResult {
  total: number
  list: BoardDetail[]
}

export interface CreateBoardPayload {
  name: string
  config?: string
  home_flag: string
  menu_flag?: string
  description?: string
  remark?: string
  vis_type?: BoardVisualizationType
  tenant_id?: string
}

export interface UpdateBoardPayload {
  id: string
  name?: string
  config?: string
  home_flag?: string
  menu_flag?: string
  description?: string
  remark?: string
  vis_type?: BoardVisualizationType
}

export function fetchBoardById(id: string) {
  return request.get<BoardDetail>(`/board/${encodeURIComponent(id)}`)
}

export function fetchBoards(params: BoardListParams) {
  return request.get<BoardListResult>('/board', { params })
}

export function createBoard(payload: CreateBoardPayload) {
  return request.post<BoardDetail>('/board', payload)
}

export function updateBoard(payload: UpdateBoardPayload) {
  return request.put<BoardDetail>('/board', payload)
}

export function deleteBoard(id: string) {
  return request.delete<null>(`/board/${encodeURIComponent(id)}`)
}

export function publishBoard(id: string) {
  return request.post<BoardDetail>(`/board/${encodeURIComponent(id)}/publish`)
}

export function fetchPublishedBoardByShareToken(token: string) {
  return request.get<BoardDetail>(`/board/shared/${encodeURIComponent(token)}`)
}
