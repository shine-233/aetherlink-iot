/**
 * 文件用途: 物模型、设备模型和设备选择器相关 API wrapper。
 * 核心逻辑: 封装物模型分页、详情、删除、模型增改查和可选设备列表接口。
 * 关键注意事项: 物模型 ID、模型 ID、设备选择筛选条件会影响设备配置页和物模型页的数据归属。
 * 重构建议: 将物模型管理、模型管理、设备选择拆成独立模块，并用具体类型替代宽泛对象。
 */
import { request } from '../request'

/** 获取设备物模型列表 */
export const deviceTemplate = async (params: { page: number; page_size: number; name?: string }) => {
  return await request.get<Api.BaseApi.Data | any>(`/device/template`, { params })
}

/** 删除设备物模型 */
export const deleteDeviceTemplate = async (id: string) => {
  return await request.delete<Api.BaseApi.Data | any>(`/device/template/${id}`)
}

export const getDeviceTemplateDetail = async (id: string) => {
  return await request.get<Api.BaseApi.Data | any>(`/device/template/detail/${id}`)
}

export const getDeviceModel = async (params: { page: number; page_size: number; device_template_id: string }) => {
  return await request.get<Api.BaseApi.Data | any>(`/device/model/telemetry`, { params })
}

export const postDeviceModel = async (params: {
  device_template_id: string
  data_identifier: string
  data_name?: string
  data_type?: string
  unit?: string
  description?: string
}) => {
  return await request.post<Api.BaseApi.Data | any>(`/device/model/telemetry`, { params })
}

export const putDeviceModel = async (params: {
  device_template_id: string
  data_identifier: string
  data_name?: string
  data_type?: string
  unit?: string
  description?: string
}) => {
  return await request.put<Api.BaseApi.Data | any>(`/device/model/telemetry`, { params })
}

/**
 * 获取用于下拉选择的设备列表 (根据文档更新)
 *
 * @param params - 查询参数 (可选)
 */
export const getDeviceListForSelect = async (params?: Api.Device.DeviceSelectorParams) => {
  // 使用文档提供的 URL 和参数类型
  // API返回格式: { code: 200, message: "操作成功", data: { total: number, list: DeviceSelectItem[] } }
  return await request.get<{
    total: number
    list: Api.Device.DeviceSelectItem[]
  }>(`/device/selector`, { params })
}
