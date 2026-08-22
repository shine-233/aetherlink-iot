/**
 * 文件用途：封装产品 OTA 更新任务相关接口。
 * 核心逻辑：提供 OTA 包、任务或升级状态的请求函数。
 * 关键注意事项：OTA 接口影响设备升级链路，参数和路径变更需后端契约确认。
 * 重构建议：可补充类型化请求响应和失败重试语义说明。
 */
import { request } from '../request'

export const getOtaTaskList = async (params: object) => {
  return await request.get('/ota/task', { params })
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
export const deleteOtaPackage = (id: number) => request.delete(`/ota/package/${id}`)
// /ota/task
export const addOtaTask = async (data: object) => {
  return await request.post('/ota/task', data)
}
export const previewOtaTask = async (data: object) => {
  return await request.post('/ota/task/preview', data)
}
export const getOtaTaskSupportBundle = (taskId: string) =>
  request.get(`/ota/task/${encodeURIComponent(taskId)}/support-bundle`, { silentError: true })
// /ota/task/detail
export const getOtaTaskDetail = async (params: object) => {
  return await request.get(`/ota/task/detail`, { params })
}
export const editOtaTaskDetail = async (params: object) => {
  return await request.put(`/ota/task/detail`, params)
}
