/**
 * 文件用途：封装产品升级包管理相关接口。
 * 核心逻辑：提供升级包列表、上传、删除或状态查询的请求能力。
 * 关键注意事项：文件上传和包版本字段需与后端保持一致，避免升级包不可用。
 * 重构建议：可拆出 PackageVersion 类型和上传响应类型，降低 any 扩散。
 */
import { request } from '../request'

export const getOtaPackageList = async (params: object) => {
  return await request.get('/ota/package', { params })
}
export const getDeviceList = async (params: object) => {
  return await request.get('/device', { params })
}
export const addOtaPackage = async (data: object) => {
  return await request.post('/ota/package', data)
}
export const editOtaPackage = async (data: object) => {
  return await request.put('/ota/package', data)
}
export const deleteOtaPackage = (id: string) => request.delete(`/ota/package/${id}`)
