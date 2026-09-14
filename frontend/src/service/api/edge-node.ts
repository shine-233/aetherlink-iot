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
