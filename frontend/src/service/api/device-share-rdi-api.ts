/**
 * 文件用途: 设备域 API wrapper —— 设备影子（shadow）命令投递队列，服务共享/RDI 场景下的离线下发能力。
 * 核心逻辑: 对接后端 `/device/shadow/:deviceId` 接口；设备在线直接下发，离线则写入缓存队列待其上线后投递。
 * 关键注意事项: 当前生产下发链路仅支持 message_type: 'command'，扩展其它消息类型前需与后端契约对齐。
 * 来源: 从 device.ts 按"共享/RDI"域拆分而来，签名与行为保持不变。
 */
import { request } from '../request'

/** 设备影子命令负载。当前生产下发链路仅支持 command。 */
export interface DeviceShadowCommandParams {
  message_type: 'command'
  payload: unknown
  ttl_seconds?: number
}

/** 设备影子消息列表（可按 status 过滤） */
export const deviceShadowList = async (deviceId: string, params?: object) => {
  return await request.get(`/device/shadow/${deviceId}`, { params })
}
/** 设置设备影子命令：设备在线直接下发，离线写入缓存队列 */
export const deviceShadowSet = async (deviceId: string, params: DeviceShadowCommandParams) => {
  return await request.post(`/device/shadow/${deviceId}`, params)
}
/** 取消待投递的影子消息 */
export const deviceShadowCancel = async (deviceId: string, msgId: string) => {
  return await request.delete(`/device/shadow/${deviceId}/${msgId}`)
}
