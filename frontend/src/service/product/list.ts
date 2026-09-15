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
// /file/up：批次文件上传（type=importBatch，仅允许 .csv/.xlsx/.xls）
export const uploadImportBatchFile = async (formData: FormData) => {
  // 必须显式声明一个"非 JSON"的 Content-Type，否则文件传不上去。
  //
  // 原因：请求实例的默认头是 `Content-Type: application/json`
  // （packages/axios/src/options.ts 的 createAxiosConfig），而 axios 的
  // transformRequest 对 FormData 是这样分支的：
  //   if (utils.isFormData(data)) {
  //     return hasJSONContentType ? JSON.stringify(formDataToJSON(data)) : data
  //   }
  // 于是 File 部分无法序列化，请求体退化成 `{}`，
  // 后端 `c.FormFile("file")` 拿不到文件，返回业务码 202001「请选择需要上传的文件」。
  //
  // 声明 multipart 后：transformRequest 走 else 分支原样返回 FormData，
  // 而 axios 的 xhr 适配器对 FormData 会 `setContentType(false)` 把头交给浏览器，
  // 由浏览器自动补 boundary —— 所以这里写死不带 boundary 是正确做法，不要手写 boundary。
  //
  // 2026-09-15 实测：不这样写时预注册 CSV 导入在浏览器里根本提交不了
  // （HTTP 200 但业务码报错，页面表现为"点了没反应"）。
  return await request.post<{ path?: string }>('/file/up', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}
// /device_config/{ id };
export const delDeviceConfig = (id: string) => request.delete(`/device_config/${id}`)
export const getDict = async (params: object) => {
  return await request.get('/dict', { params })
}
