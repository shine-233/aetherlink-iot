/**
 * 文件用途: SCADA / Widget 基础层与移动端 API wrapper（ROADMAP P1.3 / P1.4）。
 * 核心逻辑: 封装项目、画布文档、版本、控制审计与移动端能力/推送/命令接口。
 * 关键注意事项:
 *  1. 保存必须带 `expected_version`。后端按它做乐观并发条件更新，
 *     前端若省略会让用户的画布被别人的保存无声覆盖——后端会拒绝，
 *     但错误必须能在这里被识别出来，而不是笼统地报"保存失败"。
 *  2. `tenant_id` 只对系统管理员有意义（query 参数）；租户管理员传了也会被后端拒绝，
 *     这里照传是为了让后端给出明确错误，而不是前端悄悄改掉用户意图。
 *  3. 控制命令的确认令牌与命令本体分两个接口：先换令牌再下发，
 *     把"确认过"变成一个可验证的事实，而不是前端一个勾选项。
 */
import { request } from '../request'

export type ScadaDocumentStatus = 'DRAFT' | 'PUBLISHED' | 'ARCHIVED'
export type ControlOutcome = 'pending' | 'success' | 'denied' | 'failed'

export interface ScadaProject {
  id: string
  tenant_id: string
  name: string
  description?: string | null
  created_by?: string | null
  created_at: string
  updated_at: string
}

export interface ScadaDocument {
  id: string
  tenant_id: string
  project_id: string
  name: string
  status: ScadaDocumentStatus
  current_version: number
  published_version: number | null
  json_data: string | null
}

export interface ScadaDocumentVersion {
  id: string
  tenant_id: string
  document_id: string
  version: number
  json_data: string | null
  published_by?: string | null
  published_at: string
}

export interface ScadaControlAudit {
  id: string
  tenant_id: string
  document_id: string
  widget_id: string
  command: string
  params: string | null
  actor_user_id: string
  confirmation_token: string
  outcome: ControlOutcome
  detail?: string | null
  created_at: string
}

export interface MobileCapabilityMatrix {
  telemetry: boolean
  commands: boolean
  alarms: boolean
  shadow: boolean
  ota: boolean
  dashboards: boolean
  push: boolean
  offline_cache: boolean
}

/** 构造 tenant 查询串；为空时不带参数。 */
function tenantQuery(tenantId?: string): string {
  const trimmed = (tenantId ?? '').trim()
  return trimmed ? `?tenant_id=${encodeURIComponent(trimmed)}` : ''
}

// ---------------------------------------------------------------------------
// 项目
// ---------------------------------------------------------------------------

export function fetchScadaProjects(tenantId?: string) {
  return request.get<ScadaProject[]>(`/scada/projects${tenantQuery(tenantId)}`)
}

export function fetchScadaProject(id: string, tenantId?: string) {
  return request.get<ScadaProject>(`/scada/projects/${id}${tenantQuery(tenantId)}`)
}

export function createScadaProject(payload: { name: string; description?: string; tenant_id?: string }) {
  return request.post<ScadaProject>('/scada/projects', payload)
}

export function deleteScadaProject(id: string, tenantId?: string) {
  return request.delete(`/scada/projects/${id}${tenantQuery(tenantId)}`)
}

// ---------------------------------------------------------------------------
// 画布文档
// ---------------------------------------------------------------------------

export function fetchScadaDocuments(projectId: string, tenantId?: string) {
  return request.get<ScadaDocument[]>(`/scada/projects/${projectId}/documents${tenantQuery(tenantId)}`)
}

export function createScadaDocument(
  projectId: string,
  payload: { name: string; json_data?: string; tenant_id?: string }
) {
  return request.post<ScadaDocument>(`/scada/projects/${projectId}/documents`, payload)
}

export function fetchScadaDocument(id: string, tenantId?: string) {
  return request.get<ScadaDocument>(`/scada/documents/${id}${tenantQuery(tenantId)}`)
}

export function saveScadaDocument(
  id: string,
  payload: { expected_version: number; json_data: string; tenant_id?: string }
) {
  return request.put<ScadaDocument>(`/scada/documents/${id}`, payload)
}

export function publishScadaDocument(id: string, tenantId?: string) {
  return request.post<ScadaDocument>(`/scada/documents/${id}/publish${tenantQuery(tenantId)}`)
}

export function rollbackScadaDocument(id: string, version: number, tenantId?: string) {
  return request.post<ScadaDocument>(`/scada/documents/${id}/rollback${tenantQuery(tenantId)}`, { version })
}

export function archiveScadaDocument(id: string, tenantId?: string) {
  return request.post<ScadaDocument>(`/scada/documents/${id}/archive${tenantQuery(tenantId)}`)
}

export function fetchScadaDocumentVersions(id: string, tenantId?: string) {
  return request.get<ScadaDocumentVersion[]>(`/scada/documents/${id}/versions${tenantQuery(tenantId)}`)
}

export function fetchScadaControlAudits(id: string, tenantId?: string) {
  return request.get<ScadaControlAudit[]>(`/scada/documents/${id}/audits${tenantQuery(tenantId)}`)
}

// ---------------------------------------------------------------------------
// 实时控制
// ---------------------------------------------------------------------------

export function issueControlConfirmation(payload: {
  document_id: string
  widget_id: string
  command: string
  tenant_id?: string
}) {
  return request.post<{ confirmation_token: string }>('/scada/control/confirm', payload)
}

export function executeControl(payload: {
  device_id: string
  document_id: string
  widget_id: string
  widget_type: string
  version?: string
  command: string
  params?: Record<string, unknown>
  confirmation_token?: string
  tenant_id?: string
}) {
  return request.post<{ outcome: ControlOutcome }>('/scada/control', payload)
}

// ---------------------------------------------------------------------------
// 移动端（P1.4）
// ---------------------------------------------------------------------------

export function fetchMobileCapabilities() {
  return request.get<MobileCapabilityMatrix>('/mobile/capabilities')
}

export function subscribeMobilePush(payload: { platform: 'ios' | 'android' | 'h5'; token: string; provider?: string; tenant_id?: string }) {
  return request.post('/mobile/push/subscribe', payload)
}

export function unsubscribeMobilePush(id: string, tenantId?: string) {
  return request.delete(`/mobile/push/${id}${tenantQuery(tenantId)}`)
}
