/**
 * 文件用途：Data Converter（ThingsBoard 对标数据编解码器）前端 API 客户端。
 * 核心逻辑：封装数据转换器的增删改查与在线仿真调试。
 * 关键注意事项：PROTOBUF 模式（TB-19）专属 proto_schema 为 .proto 源全文，随实体存取；
 *   Dry-Run 载荷为 hex/base64 编码的二进制串，由后端自动识别。
 * 重构建议：与 backend/internal/model/data_converter.go 的 DTO 同步演进，字段增删需双侧核对。
 */
import { request } from '../request'

export type ConverterType = 'UPLINK' | 'DOWNLINK'
export type ConverterMode = 'SCRIPT' | 'HEX_BINARY' | 'JSON_PATH' | 'PROTOBUF'

export interface DataConverterItem {
  id: string
  name: string
  type: ConverterType
  converter_mode: ConverterMode
  debug_mode: boolean
  tenant_id: string
  configuration: string
  script?: string
  /** PROTOBUF 模式专属：.proto 源文件全文（135.sql） */
  proto_schema?: string
  description?: string
  created_at?: string
  updated_at?: string
}

export interface DataConverterListParams {
  page?: number
  page_size?: number
  type?: ConverterType
  search?: string
}

export interface DataConverterListResponse {
  list: DataConverterItem[]
  total: number
}

export interface CreateDataConverterParams {
  name: string
  type: ConverterType
  converter_mode: ConverterMode
  debug_mode?: boolean
  configuration?: string
  script?: string
  proto_schema?: string
  description?: string
}

export interface UpdateDataConverterParams {
  id: string
  name?: string
  type?: ConverterType
  converter_mode?: ConverterMode
  debug_mode?: boolean
  configuration?: string
  script?: string
  proto_schema?: string
  description?: string
}

export interface TestDataConverterParams {
  type?: ConverterType
  converter_mode?: ConverterMode
  payload: string
  metadata?: Record<string, string>
  configuration?: string
  script?: string
  proto_schema?: string
  converter_id?: string
}

export interface TestDataConverterResponse {
  success: boolean
  device_name?: string
  device_type?: string
  telemetry?: Record<string, any>
  attributes?: Record<string, any>
  raw_output?: string
  logs?: string[]
  error?: string
}

/** 分页与条件查询数据转换器列表 */
export const getDataConvertersList = async (params?: DataConverterListParams) => {
  return await request.get<DataConverterListResponse>('/converters', { params })
}

/** 获取单条转换器详情 */
export const getDataConverterDetail = async (id: string) => {
  return await request.get<DataConverterItem>(`/converters/${encodeURIComponent(id)}`)
}

/** 创建数据转换器 */
export const createDataConverter = async (data: CreateDataConverterParams) => {
  return await request.post<DataConverterItem>('/converters', data)
}

/** 更新数据转换器 */
export const updateDataConverter = async (data: UpdateDataConverterParams) => {
  return await request.put<DataConverterItem>('/converters', data)
}

/** 删除数据转换器 */
export const deleteDataConverter = async (id: string) => {
  return await request.delete<boolean>(`/converters/${encodeURIComponent(id)}`)
}

/** 在线仿真测试转换器 */
export const testDataConverter = async (data: TestDataConverterParams) => {
  return await request.post<TestDataConverterResponse>('/converters/test', data)
}
