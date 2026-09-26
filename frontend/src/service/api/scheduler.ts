/**
 * 文件用途：统一调度器（scheduler_events，TB-48）前端 API 客户端。
 * 核心逻辑：封装三源聚合列表（GET /scheduler/events，只读聚合既有三套调度 + 注册行）
 *   与 scheduler_events 注册面 CRUD（创建/详情/更新/删除）。
 * 关键注意事项：
 *   1. 聚合条目 source_type（scene|report|rpc）是前端着色与图例的依据；origin 标识来源
 *      系统（scene_timer/report_schedule/fleet_command_job/scheduler_registry）；
 *   2. scene 注册事件由后端同事务落 scene_automation_timers 执行（执行行与注册行同 id），
 *      前端不做执行语义推断；同场景仅允许一条启用定时触发（后端 201002 拒绝）；
 *   3. rpc 事件为一次性触发：必须提供 next_run_at（未来时刻），cron 必须为空；
 *      scene/report 事件的 next_run_at 由后端按 UTC 从 cron 计算，请求里传了会被拒绝。
 * 重构建议：若后续注册面开放 report/rpc 事件接管执行（当前明确不做），另立写面路径并
 *   单独评审，不要在现有 CRUD 函数上加载荷开关。
 */
import { request } from '../request'

export type SchedulerEventType = 'scene' | 'report' | 'rpc'
export type SchedulerEventOrigin =
  | 'scene_timer'
  | 'report_schedule'
  | 'fleet_command_job'
  | 'scheduler_registry'

/** 聚合列表统一条目（三套存量调度 + 注册行归一后的形状） */
export interface SchedulerEventItem {
  id: string
  name: string
  /** 来源类型：scene | report | rpc（前端按此着色） */
  source_type: SchedulerEventType
  /** 来源系统：scene_timer | report_schedule | fleet_command_job | scheduler_registry */
  origin: SchedulerEventOrigin
  ref_type: string
  ref_id: string
  /** 5/6 段 cron 表达式；rpc 一次性事件为空 */
  cron: string
  timezone: string
  enabled: boolean
  /** 下一次触发时刻（RFC3339）；日历按此聚合到日 */
  next_run_at?: string | null
  /** 最近一次执行时刻（仅聚合部分来源携带） */
  last_run_at?: string | null
}

export interface SchedulerEventListParams {
  page?: number
  page_size?: number
  source_type?: SchedulerEventType
  search?: string
  enabled?: boolean
  /** 窗口起点（epoch 毫秒，next_run_at >= from_ms） */
  from_ms?: number
  /** 窗口终点（epoch 毫秒，next_run_at <= to_ms） */
  to_ms?: number
}

export interface SchedulerEventListResponse {
  list: SchedulerEventItem[]
  total: number
  page: number
  page_size: number
}

/** 注册事件行（CRUD 读路径返回的注册表本体） */
export interface SchedulerEventRecord {
  id: string
  tenant_id: string
  name: string
  event_type: SchedulerEventType
  ref_type: string
  ref_id: string
  cron: string
  next_run_at?: string | null
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface SchedulerEventCreatePayload {
  name: string
  event_type: SchedulerEventType
  ref_type?: string
  ref_id?: string
  /** scene/report 必填；rpc 必须为空 */
  cron?: string
  /** 仅 rpc 事件传入（未来时刻，RFC3339）；scene/report 由后端按 cron 计算 */
  next_run_at?: string
  enabled?: boolean
}

export interface SchedulerEventUpdatePayload {
  name?: string
  ref_type?: string
  ref_id?: string
  cron?: string
  /** 仅 rpc 事件可传（未来时刻） */
  next_run_at?: string
  enabled?: boolean
}

/** 只读聚合三套存量调度 + 注册行为统一事件列表（含来源类型） */
export const getSchedulerEvents = async (params?: SchedulerEventListParams) => {
  return await request.get<SchedulerEventListResponse>('/scheduler/events', { params })
}

/** 注册调度事件；scene 事件同步落既有 scene automation timer 机制执行 */
export const createSchedulerEvent = async (payload: SchedulerEventCreatePayload) => {
  return await request.post<SchedulerEventRecord>('/scheduler/events', payload)
}

/** 注册事件详情 */
export const getSchedulerEvent = async (id: string) => {
  return await request.get<SchedulerEventRecord>(`/scheduler/events/${encodeURIComponent(id)}`)
}

/** 更新注册事件（event_type 不可变；scene 事件同步执行行） */
export const updateSchedulerEvent = async (id: string, payload: SchedulerEventUpdatePayload) => {
  return await request.put<SchedulerEventRecord>(`/scheduler/events/${encodeURIComponent(id)}`, payload)
}

/** 删除注册事件；scene 事件同步删除执行行 */
export const deleteSchedulerEvent = async (id: string) => {
  return await request.delete<{ id: string }>(`/scheduler/events/${encodeURIComponent(id)}`)
}
