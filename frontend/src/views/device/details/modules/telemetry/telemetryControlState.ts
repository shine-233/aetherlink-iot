/**
 * 文件用途：承接遥测控制项、模拟上报入口与删除动作的纯参数拼装规则。
 * 核心逻辑：统一构造控制列表查询、模拟上报可见性判断、遥测删除参数和控制项下发 payload。
 * 关键注意事项：这里输出的字段名直接对接后端接口，修改 `device_id`、`device_template_id`、`key`、`value` 等键名会影响联调。
 * 重构建议：后续如继续收口，可把控制项和遥测项的最小类型契约抽到共享 type 文件，避免 helper 间重复声明。
 */
export type TelemetryControlListQuery = {
  device_template_id: string
  page: number
  page_size: number
  enable_status: 'enable'
}

export type TelemetryDeleteParams = {
  key: string
  device_id: string
}

export type TelemetryControlPublishPayload = {
  device_id: string
  value: string
}

export type TelemetryControlItem = {
  id?: string | number
  key?: string
  name?: string
  content?: string
}

type DeviceConfigLike = {
  protocol_type?: string
}

type TelemetryControlListResponse = {
  list?: TelemetryControlItem[]
}

export const buildControlListQuery = (deviceTemplateId: string) => ({
  device_template_id: deviceTemplateId,
  page: 1,
  page_size: 100,
  enable_status: 'enable'
}) satisfies TelemetryControlListQuery

export const shouldShowSimulationEntry = (deviceConfig?: DeviceConfigLike) => {
  if (deviceConfig !== undefined) {
    return deviceConfig.protocol_type === 'MQTT'
  }
  return true
}

export const buildDeleteParams = (item: Pick<TelemetryControlItem, 'key'>, deviceId: string) => ({
  key: item.key || '',
  device_id: deviceId
}) satisfies TelemetryDeleteParams

export const buildControlPublishPayload = (deviceId: string, value: string) => ({
  device_id: deviceId,
  value
}) satisfies TelemetryControlPublishPayload

export const normalizeControlList = (data?: TelemetryControlListResponse) => data?.list || []
