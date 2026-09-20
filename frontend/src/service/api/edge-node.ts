import { request } from '../request'

/** P1.5 边缘节点（注册/心跳/Reconcile） */
export interface EdgeNodeEntry {
  id: string
  tenant_id: string
  version: string
  capabilities: string[]
  status: string
  last_seen_at: string | null
  health: 'online' | 'degraded' | 'offline' | 'unknown'
}

export interface RegisterEdgeNodePayload {
  node_id: string
  version: string
  capabilities?: string[]
}

export interface EdgeNodeReconcilePayload {
  gateway_device_id: string
  resources: { resource_type: 'dashboard' | 'rule_chain'; resource_id: string; revision?: number | null }[]
}

export function fetchEdgeNodes(limit?: number) {
  return request.get<EdgeNodeEntry[]>('/edge/nodes', { params: limit ? { limit } : undefined })
}

export function registerEdgeNode(payload: RegisterEdgeNodePayload) {
  return request.post<unknown>('/edge/nodes', payload)
}

export function heartbeatEdgeNode(nodeId: string) {
  return request.post<unknown>(`/edge/nodes/${encodeURIComponent(nodeId)}/heartbeat`)
}

export function reconcileEdgeNode(nodeId: string, payload: EdgeNodeReconcilePayload) {
  return request.post<unknown>(`/edge/nodes/${encodeURIComponent(nodeId)}/reconcile`, payload)
}

/** P1.5 边缘节点证书详情 */
export interface EdgeNodeCertificateInfo {
  id: string
  node_id: string
  serial_number: string
  fingerprint: string
  common_name: string
  certificate: string
  not_before: string
  not_after: string
  status: string
  issued_at: string
  revoked_at?: string | null
}

export interface IssueEdgeNodeCertificateResponse extends EdgeNodeCertificateInfo {
  private_key: string
}

export function fetchEdgeNodeCertificate(nodeId: string) {
  return request.get<EdgeNodeCertificateInfo>(`/edge/nodes/${encodeURIComponent(nodeId)}/certificate`)
}

export function issueEdgeNodeCertificate(nodeId: string, validityDays?: number) {
  return request.post<IssueEdgeNodeCertificateResponse>(`/edge/nodes/${encodeURIComponent(nodeId)}/certificate`, {
    validity_days: validityDays
  })
}

export function revokeEdgeNodeCertificate(nodeId: string) {
  return request.delete<{ message: string }>(`/edge/nodes/${encodeURIComponent(nodeId)}/certificate`)
}

/** P1.5 边缘节点升级与回滚 */
export interface EdgeNodeUpgradeHistoryEntry {
  id: string
  tenant_id: string
  node_id: string
  from_version: string
  target_version: string
  package_url?: string
  checksum?: string
  status: 'pending' | 'dispatched' | 'success' | 'failed' | 'rolled_back'
  operator_id: string
  description?: string
  created_at: string
  updated_at: string
}

export interface UpgradeEdgeNodePayload {
  target_version: string
  package_url?: string
  checksum?: string
  description?: string
}

export function upgradeEdgeNode(nodeId: string, payload: UpgradeEdgeNodePayload) {
  return request.post<{
    history_id: string
    node_id: string
    from_version: string
    target_version: string
    status: string
    message: string
  }>(`/edge/nodes/${encodeURIComponent(nodeId)}/upgrade`, payload)
}

export function rollbackEdgeNode(nodeId: string, historyId: string) {
  return request.post<{
    history_id: string
    node_id: string
    rolled_to_version: string
    status: string
    message: string
  }>(`/edge/nodes/${encodeURIComponent(nodeId)}/rollback`, { history_id: historyId })
}

export function fetchEdgeNodeUpgradeHistory(nodeId: string, limit?: number) {
  return request.get<EdgeNodeUpgradeHistoryEntry[]>(`/edge/nodes/${encodeURIComponent(nodeId)}/upgrade/history`, {
    params: limit ? { limit } : undefined
  })
}
