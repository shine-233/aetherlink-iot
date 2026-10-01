/**
 * 文件用途: 设备域 API wrapper —— 设备认领（claim token 签发/查询/撤销/兑换）等接入控制面接口。
 * 核心逻辑: 对接后端 `/device/claim-tokens` 接口，支撑设备在租户间转移的"认领"流程。
 * 关键注意事项: issueDeviceClaimToken 返回的明文 claim_key 只在响应中出现一次，不应被日志或缓存持久化；
 * revokeDeviceClaimToken 作用于终态（consumed/revoked/replaced）不可逆的令牌生命周期。
 * 来源: 从 device.ts 按"设备接入控制"域拆分而来，签名与行为保持不变。
 */
import { request } from '../request'

/** TB-12 设备认领：签发一次性认领令牌（明文 claim_key 只在本响应出现一次） */
export const issueDeviceClaimToken = async (params: { device_id: string; ttl_seconds?: number }) => {
  return await request.post('/device/claim-tokens', params)
}

/** TB-12 设备认领：签发方回查令牌历史（无明文无哈希） */
export const listDeviceClaimTokens = async (device_id: string) => {
  return await request.get('/device/claim-tokens', { params: { device_id } })
}

/** TB-12 设备认领：撤销 active 令牌（consumed/revoked/replaced 终态不可逆） */
export const revokeDeviceClaimToken = async (token_id: string) => {
  return await request.delete(`/device/claim-tokens/${token_id}`)
}

/** TB-12 设备认领：认领设备（设备从签发租户转移到当前租户） */
export const redeemDeviceClaim = async (params: { device_number: string; claim_key: string }) => {
  return await request.post('/device/claim-tokens/redeem', params)
}
