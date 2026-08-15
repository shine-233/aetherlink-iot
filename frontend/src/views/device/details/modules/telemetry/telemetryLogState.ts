/**
 * 文件用途：承接遥测操作日志查询的分页常量与查询参数拼装。
 * 核心逻辑：统一约束操作日志分页大小，并把设备 ID、操作类型、状态筛选转成接口参数。
 * 关键注意事项：页大小会同时影响前端分页器与后端查询参数，调整时要同步检查 UI 和接口负载。
 * 重构建议：后续若日志筛选条件继续增长，可把查询参数抽成独立 type/validator，避免散落在多个 composable 中。
 */
export const TELEMETRY_LOG_PAGE_SIZE = 5

export type TelemetryLogQueryOptions = {
  deviceId: string
  page: number
  operationType: string
  status: string
}

export const buildTelemetryLogQuery = (options: TelemetryLogQueryOptions) => ({
  page: options.page,
  page_size: TELEMETRY_LOG_PAGE_SIZE,
  device_id: options.deviceId,
  operation_type: options.operationType,
  status: options.status
})

export const telemetryLogPageCount = (count: number) => Math.ceil(Math.max(count, 0) / TELEMETRY_LOG_PAGE_SIZE)
