/**
 * 文件用途：Integration（TB-45 统一集成实体）前端 API 客户端。
 * 核心逻辑：封装 integrations 的增删改查与分页检索；转换器绑定 ID 以字符串提交，后端做同租户校验。
 * 关键注意事项：connector_type 枚举与后端 CHECK 约束一致（opcua/snmp/plugin）；config 为 JSON 字符串，
 *
 *	空串绑定 ID 语义为"解绑"（后端归一为 NULL）。
 *
 * 重构建议：后续连接器类型增多时，为各 connector_type 拆分类型化 config 表单与校验。
 */
import { createResource } from './resource'

export type IntegrationConnectorType = 'opcua' | 'snmp' | 'plugin'

export interface IntegrationItem {
  id: string
  name: string
  tenant_id: string
  connector_type: IntegrationConnectorType
  converter_uplink_id?: string | null
  converter_downlink_id?: string | null
  config: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface IntegrationListParams {
  page?: number
  page_size?: number
  connector_type?: IntegrationConnectorType
  enabled?: boolean
  search?: string
}

export interface IntegrationListResponse {
  list: IntegrationItem[]
  total: number
}

export interface CreateIntegrationParams {
  name: string
  connector_type: IntegrationConnectorType
  converter_uplink_id?: string
  converter_downlink_id?: string
  config?: string
  enabled?: boolean
}

export interface UpdateIntegrationParams {
  id: string
  name?: string
  connector_type?: IntegrationConnectorType
  converter_uplink_id?: string
  converter_downlink_id?: string
  config?: string
  enabled?: boolean
}

const integrations = createResource<
  IntegrationListParams,
  IntegrationListResponse,
  IntegrationItem,
  CreateIntegrationParams,
  UpdateIntegrationParams,
  boolean
>({ collection: '/integrations' })

/** 分页与条件查询集成实例列表 */
export const getIntegrationsList = integrations.list

/** 获取单个集成实例详情 */
export const getIntegrationDetail = integrations.detail

/** 创建集成实例 */
export const createIntegration = integrations.create

/** 更新集成实例（部分字段更新；converter_*_id 传空串表示解绑） */
export const updateIntegration = integrations.update

/** 删除集成实例 */
export const deleteIntegration = integrations.remove
