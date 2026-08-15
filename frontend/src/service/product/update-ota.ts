/**
 * 文件用途：封装产品 OTA 更新任务相关接口。
 * 核心逻辑：提供 OTA 包、任务或升级状态的请求函数。
 * 关键注意事项：OTA 接口影响设备升级链路，参数和路径变更需后端契约确认。
 * 重构建议：可补充类型化请求响应和失败重试语义说明。
 */
import { request } from '../request'

export const getOtaTaskList = (params: any): Promise<any> => request.get('/ota/task', { params })
export const getDeviceList = (params: any): Promise<any> => request.get('/device', { params })
export const addOtaPackage = (data: any): Promise<any> => request.post('/ota/package', data)
export const editOtaPackage = (data: any): Promise<any> => request.put('/ota/package', data)
export const deleteOtaPackage = (id: number): Promise<any> => request.delete(`/ota/package/${id}`)
// /ota/task
export const addOtaTask = (data: any): Promise<any> => request.post('/ota/task', data)
export const previewOtaTask = (data: any): Promise<any> => request.post('/ota/task/preview', data)
export const getOtaTaskSupportBundle = (taskId: string): Promise<any> =>
  request.get(`/ota/task/${encodeURIComponent(taskId)}/support-bundle`, { silentError: true })
// /ota/task/detail
export const getOtaTaskDetail = (params): Promise<any> => request.get(`/ota/task/detail`, { params })
export const editOtaTaskDetail = (params): Promise<any> => request.put(`/ota/task/detail`, params)
