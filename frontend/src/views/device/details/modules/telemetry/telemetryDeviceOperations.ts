/**
 * 文件用途：承接遥测页面和设备 API 之间的轻量适配动作。
 * 核心逻辑：统一处理“是否展示模拟上报入口”“读取模板控制项”“删除遥测项”三类网络动作的薄封装。
 * 关键注意事项：这里故意只做轻适配，不承接页面状态；失败分支仍交由上层 composable 决定是否提示或重试。
 * 重构建议：后续若设备详情模块形成统一请求结果类型，可把这里的 `RequestResult` 替换成共享契约。
 */
import {
  buildControlListQuery,
  normalizeControlList,
  type TelemetryControlItem,
  type TelemetryDeleteParams
} from './telemetryControlState'

type RequestResult<T> = {
  data: T
  error?: unknown
}

export const loadTelemetryControlList = async (
  deviceTemplateId: string,
  controlListRequest: (params: Record<string, unknown>) => Promise<RequestResult<{ list?: TelemetryControlItem[] }>>
): Promise<TelemetryControlItem[]> => {
  if (!deviceTemplateId) return []
  const { data } = await controlListRequest(buildControlListQuery(deviceTemplateId))
  return normalizeControlList(data)
}

export const deleteTelemetryItem = async (
  params: TelemetryDeleteParams,
  deleteRequest: (params: TelemetryDeleteParams) => Promise<RequestResult<unknown>>
) => {
  const { error } = await deleteRequest(params)
  return !error
}
