/**
 * 文件用途：封装产品与预注册设备列表相关接口。
 * 核心逻辑：通过 request 包装产品增删改查、设备列表和字典查询。
 * 关键注意事项：接口路径与后端契约绑定，历史注释异常不代表可改业务路径。
 * 重构建议：可把 any 参数替换为产品领域 DTO 并补充错误路径测试。
 */
import { request } from '../request'

export const getProductList = (params: any): Promise<any> => request.get('/product', { params })
export const getDeviceList = (params: any): Promise<any> => request.get('/device', { params })
export const getPreProductList = (params: any): Promise<any> => request.get('/device/preRegister', { params })
export const addProduct = (data: any): Promise<any> => request.post('/product', data)
export const editProduct = (data: any): Promise<any> => request.put('/product', data)
export const deleteProduct = (id: string): Promise<any> => request.delete(`/product/${id}`)
// /device/Reeegiprrst;
export const addDevice = (data: any): Promise<any> => request.post('/device/preRegister', data)
// /device/preRegister/export
export const exportDevice = (params: any): Promise<any> => request.get('/device/preRegister/export', { params })
// /device_config/{ id };
export const delDeviceConfig = (id: string): Promise<any> => request.delete(`/device_config/${id}`)
export const getDict = (params: any): Promise<any> => request.get('/dict', { params })
