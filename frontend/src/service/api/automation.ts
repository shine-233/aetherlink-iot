/**
 * 文件用途: 自动化场景和联动规则 API wrapper。
 * 核心逻辑: 封装设备/配置指标菜单、scene、scene_automations 的增删改查、启停和详情接口。
 * 关键注意事项: 条件组、动作组和 metrics 参数直接影响后端规则解析，字段漂移会导致规则误配置。
 * 重构建议: 将菜单查询、普通场景、联动规则拆成独立模块，并补 payload mapper 的契约测试。
 */
import { request } from '../request'

export type SceneAutomationDryRunNode = Record<string, unknown>
export type SceneAutomationDryRunConditionGroup = SceneAutomationDryRunNode[]

export interface SceneAutomationDryRunPayload {
  id?: string
  name?: string | null
  description?: string | null
  enabled?: string
  trigger_condition_groups: SceneAutomationDryRunConditionGroup[]
  actions: SceneAutomationDryRunNode[]
}

export interface SceneAutomationDryRunStats {
  condition_group_count?: number
  condition_count?: number
  action_count?: number
  condition_types?: Record<string, number>
  action_types?: Record<string, number>
  target_kinds?: Record<string, number>
  reference_counts?: Record<string, number>
}

export interface SceneAutomationDryRunResult {
  supported?: boolean
  valid?: boolean
  can_save?: boolean
  canSave?: boolean
  summary?: string
  dry_run?: SceneAutomationDryRunStats
  dryRun?: SceneAutomationDryRunStats
  reference_counts?: Record<string, number>
  referenceCounts?: Record<string, number>
  blocking_errors?: string[]
  blockers?: string[]
  matched_devices?: number
  matchedDevices?: number
  skipped_conditions?: string[]
  skippedConditions?: string[]
  unavailable_actions?: string[]
  unavailableActions?: string[]
  condition_results?: SceneAutomationDryRunNode[]
  action_results?: SceneAutomationDryRunNode[]
  warnings?: string[]
  errors?: string[]
  diagnostics?: Array<{
    severity?: 'success' | 'info' | 'warning' | 'error' | string
    scope?: string
    message?: string
  }>
  next_steps?: string[]
  [key: string]: unknown
}

export interface SceneDryRunPayload {
  id?: string
  name?: string | null
  description?: string | null
  actions: SceneAutomationDryRunNode[]
}
/** 获取设备列表下拉菜单 */
export const deviceListAll = async (params: object) => {
  return await request.get('/device/tenant/list', { params })
}

/** 获取设备配置下拉菜单 */
export const deviceConfigAll = async (params: object) => {
  return await request.get('/device_config/menu', { params })
}

/** 单个设备条件选择下拉菜单 */
export const deviceMetricsConditionMenu = async (params: object) => {
  return await request.get(`/device/metrics/condition/menu`, { params })
}

/** 单类设备条件选择下拉菜单 */
export const configMetricsConditionMenu = async (params: object) => {
  return await request.get(`/device_config/metrics/condition/menu`, { params })
}

/** 单个设备动作选择下拉菜单 */
export const deviceMetricsMenu = async (params: object) => {
  return await request.get(`/device/metrics/menu`, { params })
}

/** 单类设备动作选择下拉菜单 */
export const deviceConfigMetricsMenu = async (params: object) => {
  return await request.get(`/device_config/metrics/menu`, { params })
}

/** 创建场景 */
export const sceneAdd = async (params: object) => {
  return await request.post(`/scene`, params)
}

/** 修改场景 */
export const sceneEdit = async (params: object) => {
  return await request.put(`/scene`, params)
}

/** 获取场景列表 */
export const sceneGet = async (params: object) => {
  return await request.get(`/scene`, { params })
}

/** 删除场景 */
export const sceneDel = async (id: string | number) => {
  return await request.delete(`/scene/${id}`)
}

/** 获取场景详情 */
export const sceneInfo = async (id: string | number) => {
  return await request.get(`/scene/detail/${id}`)
}

/** 获取场景日志 */
export const sceneLog = async (params: object) => {
  return await request.get(`/scene/log`, { params })
}

/** 激活场景 */
export const sceneActive = async (id: string | number) => {
  return await request.post(`/scene/active/${id}`)
}

/** Preview ordinary scene action references without saving or executing. */
export const sceneDryRun = async (params: SceneDryRunPayload) => {
  return await request.post<SceneAutomationDryRunResult>(`/scene/dry-run`, params, {
    silentError: true
  })
}

/** 创建场景 */
export const sceneAutomationsAdd = async (params: object) => {
  return await request.post(`/scene_automations`, params)
}

/** 修改场景 */
export const sceneAutomationsEdit = async (params: object) => {
  return await request.put(`/scene_automations`, params)
}

/** 获取场景列表 */
export const sceneAutomationsGet = async (params: object) => {
  return await request.get(`/scene_automations/list`, { params })
}

/** 删除场景 */
export const sceneAutomationsDel = async (id: string | number) => {
  return await request.delete(`/scene_automations/${id}`)
}

/** 获取场景详情 */
export const sceneAutomationsInfo = async (id: string | number) => {
  return await request.get(`/scene_automations/detail/${id}`)
}

/** 获取场景日志 */
export const sceneAutomationsLog = async (params: object) => {
  return await request.get(`/scene_automations/log`, { params })
}

/** 激活场景 */
export const sceneAutomationsSwitch = async (id: string | number) => {
  return await request.post(`/scene_automations/switch/${id}`)
}

/** Preview the execution contract without saving or running the automation. */
export const sceneAutomationsDryRun = async (params: SceneAutomationDryRunPayload) => {
  return await request.post<SceneAutomationDryRunResult>(`/scene_automations/dry-run`, params, {
    silentError: true
  })
}
