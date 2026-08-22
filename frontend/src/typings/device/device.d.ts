/**
 * 文件用途：声明 设备领域类型声明 使用的类型契约。
 * 核心逻辑：集中描述组件配置、业务字段或全局类型，供实现文件和测试复用。
 * 关键注意事项：类型字段需要与真实 API、store 数据和组件入参保持一致，不能用过宽类型掩盖漂移。
 * 重构建议：可按业务对象拆分声明，并为关键类型补充示例 fixture 或类型级验证。
 */
declare namespace DeviceManagement {
  interface Group {
    id: string
    parent_id: string
    tier: number
    name: string
    description: string | null
    created_at: string
    updated_at: string
    remark: string | null
    tenant_id: string
  }

  interface TreeNode {
    group: Group
    children?: TreeNode[] // TreeNode类型的可选数组，用于描述子节点
  }

  // 用于描述包含根节点和可能的子节点的整个树结构
  type TreeStructure = TreeNode[]

  interface DetailData {
    detail: {
      created_at: string
      description: string
      id: string
      name: string
      parent_id: string
      remark: string
      tenant_id: string
      tier: number
      updated_at: string
    }
    tier: {
      group_path: string
    }
  }

  interface GroupDeviceData {
    any
  }

  interface RDISystemInfoSummary {
    installation_location?: string
    address?: string
    installation_date?: string
    installer_company?: string
    installer_contact?: string
    installer_name?: string
    installer_phone?: string
    installer_email?: string
    controller_serial_number?: string
    maintenance_technician?: string
  }

  interface DeviceData {
    id: string
    activate_flag: string
    current_version: string
    device_config_id: string
    device_number: string
    device_type: number
    group_id: string
    is_enabled: string
    is_online?: number
    label: string
    name: string
    product_id: string
    protocol: string
    rdi_system_info_summary?: RDISystemInfoSummary
  }

  interface DeviceDatas {
    list: DeviceData[]
    total: number
  }

  interface DeviceDetail {
    id: string
    name: string
    voucher: string // 凭证
    tenant_id: string
    is_enabled: string // 启用/禁用 enabled-启用 disabled-禁用 默认禁用，激活后默认启用
    activate_flag: string // 激活标志 inactive-未激活 active-已激活
    created_at: string
    update_at: string
    device_number: string // 设备编号
    product_id: string // 产品id
    parent_id: string // 网关id
    label: string // 标签 单标签，英文逗号隔开
    location: string // 地理位置
    sub_device_addr: string // 子设备地址
    current_version: string // 固件版本
    additional_info: string // 附件信息 json字符串
    protocol_config: string // 协议插件设备配置 协议插件相关的设备配置
    device_config_name: string
    remark1: string
    remark2: string
    remark3: string
    device_config_id: string // 设备配置id
    batch_number: string // 批次号
    activate_at: string // 激活时间
    is_online: number // 是否在线
    ts?: number
    device_config?: Partial<ConfigData>
  }

  interface telemetryData {
    device_id: string
    key: string
    tenant_id: string
    ts: string
    value: number
    unit: string
    label: string
    name: string
  }

  /** GET /telemetry/datas/current/{id} 返回遥测行数组（与后端 buildTelemetryCurrentRows 对齐） */
  type telemetryCurrent = telemetryData[]

  interface ConfigData {
    id: string
    name: string
    device_template_id: string
    device_type: string
    protocol_type: string
    voucher_type: string
    protocol_config: string
    device_conn_type: string
    additional_info: string
    description: string
    tenant_id: string
    created_at: string
    updated_at: string
    remark: null
    device_count: number
  }

  interface ConfigDatas {
    list: ConfigData[]
    total: number
  }

  interface ProtocolAndServiceItem {
    name: string
    service_identifier: string
  }

  interface ServiceSelectServiceItem extends ProtocolAndServiceItem {
    service_plugin_id: string
  }

  interface ProtocolAndService {
    protocol: ProtocolAndServiceItem[]
    service: ServiceSelectServiceItem[]
  }

  interface ServiceData {
    id: string
    name: string
    service_plugin_id: string
    voucher: string
    description: string
    service_access_config: string
    remark: string
    create_at: string
    update_at: string
    tenant_id: string
  }

  interface ServiceList {
    list: ServiceData[]
    total: number
  }

  /** type alias（而非 interface）：调用方常把这些对象赋给带索引签名的本地类型 */
  type TemplateData = {
    id: string
    name: string
    app_chart_config?: string
    web_chart_config?: string
  }

  interface TemplateDatas {
    list: TemplateData[]
    total: number
  }

  interface TemplateDetailData extends TemplateData {}

  interface DataScriptItem {
    id: string
    name: string
    content: string
    description: string
    device_config_id: string
    enable_flag: string
    script_type: string
  }

  interface DataScriptDatas {
    list: DataScriptItem[]
    total: number
  }
}
