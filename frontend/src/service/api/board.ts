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
  /** 看板项目过滤（P1.x）：项目 ID 或 "none"=内置项目（不落库的默认分组） */
  project_id?: string
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

// ---- P1.x 看板项目分组（native-board-provider 项目增删改） ----

export interface BoardProject {
  id: string
  tenant_id: string
  name: string
  description: string | null
  created_at: string
  updated_at: string
}

export interface BoardProjectListParams {
  /** 反查包含该看板的项目（后端校验看板必须在租户内） */
  board_id?: string
}

export interface CreateBoardProjectPayload {
  name: string
  description?: string | null
}

export interface UpdateBoardProjectPayload {
  name: string
  description?: string | null
}

export function fetchBoardProjects(params?: BoardProjectListParams) {
  return request.get<BoardProject[]>('/board/projects', { params })
}

export function fetchBoardProjectById(id: string) {
  return request.get<BoardProject>(`/board/projects/${encodeURIComponent(id)}`)
}

export function createBoardProject(payload: CreateBoardProjectPayload) {
  return request.post<BoardProject>('/board/projects', payload)
}

export function updateBoardProject(id: string, payload: UpdateBoardProjectPayload) {
  return request.put<BoardProject>(`/board/projects/${encodeURIComponent(id)}`, payload)
}

export function deleteBoardProject(id: string) {
  return request.delete<null>(`/board/projects/${encodeURIComponent(id)}`)
}

export function addBoardToProject(projectId: string, boardId: string) {
  return request.put<null>(`/board/projects/${encodeURIComponent(projectId)}/boards/${encodeURIComponent(boardId)}`)
}

export function removeBoardFromProject(projectId: string, boardId: string) {
  return request.delete<null>(`/board/projects/${encodeURIComponent(projectId)}/boards/${encodeURIComponent(boardId)}`)
}

/** 看板所属项目；null = 内置项目（不落库的默认分组） */
export function fetchBoardProjectMembership(boardId: string) {
  return request.get<BoardProject | null>(`/board/projects/member-of/${encodeURIComponent(boardId)}`)
}
