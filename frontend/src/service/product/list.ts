/**
 * 文件用途：封装产品与预注册设备列表相关接口。
 * 核心逻辑：通过 request 包装产品增删改查、设备列表和字典查询。
 * 关键注意事项：接口路径与后端契约绑定，历史注释异常不代表可改业务路径。
 * 重构建议：可把请求参数替换为产品领域 DTO 并补充错误路径测试。
 */
import { request } from '../request'

export const getProductList = async (params: object) => {
  return await request.get('/product', { params })
}
export const getDeviceList = async (params: object) => {
  return await request.get('/device', { params })
}
export const getPreProductList = async (params: object) => {
  return await request.get('/device/preRegister', { params })
}
export const addProduct = async (data: object) => {
  return await request.post('/product', data)
}
export const editProduct = async (data: object) => {
  return await request.put('/product', data)
}
export const deleteProduct = (id: string) => request.delete(`/product/${id}`)
// /device/Reeegiprrst;
export const addDevice = async (data: object) => {
  return await request.post('/device/preRegister', data)
}
// /device/preRegister/export
export const exportDevice = async (params: object) => {
  return await request.get('/device/preRegister/export', { params })
}
// /device_config/{ id };
export const delDeviceConfig = (id: string) => request.delete(`/device_config/${id}`)
export const getDict = async (params: object) => {
  return await request.get('/dict', { params })
}
