import {
  deviceAdd,
  deviceConfigAdd,
  deviceConfigVoucherType,
  deviceProtocolServiceList,
  deviceTemplateAdd
} from '@/service/api/device'

export type HomeFirstRunProtocol = 'MQTT' | 'HTTP'

export interface HomeFirstRunQuickCreateResult {
  templateId: string
  configId: string
  deviceId: string
  deviceName: string
  protocol: HomeFirstRunProtocol
  configName: string
}

export const HOME_FIRST_RUN_TENANT_REQUIRED_CODE = 'HOME_FIRST_RUN_TENANT_REQUIRED'

export class HomeFirstRunTenantRequiredError extends Error {
  code = HOME_FIRST_RUN_TENANT_REQUIRED_CODE

  constructor() {
    super('当前账号还没有有效租户上下文，请先创建租户管理员/租户，再生成第一台设备。')
  }
}

export function getHomeFirstRunTenantId(userInfo: Record<string, unknown> | null | undefined) {
  return String(userInfo?.tenant_id || userInfo?.tenantId || userInfo?.TenantID || '').trim()
}

export function assertHomeFirstRunTenantContext(userInfo: Record<string, unknown> | null | undefined) {
  if (!getHomeFirstRunTenantId(userInfo)) {
    throw new HomeFirstRunTenantRequiredError()
  }
}

function unwrapResponseData(response: any): any {
  if (!response) return null
  if (response.data?.id) return response.data
  if (response.data?.data?.id) return response.data.data
  if (response.id) return response
  return response.data ?? response
}

function requireCreatedId(response: any, label: string): string {
  if (response?.error) {
    throw new Error(response.error?.message || response.message || `${label} 创建失败`)
  }
  const data = unwrapResponseData(response)
  const id = data?.id
  if (typeof id !== 'string' || !id) {
    throw new Error(`${label} 创建成功但未返回 ID`)
  }
  return id
}

function buildQuickCreateSuffix() {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}${pad(now.getHours())}${pad(
    now.getMinutes()
  )}${pad(now.getSeconds())}`
}

function normalizeHomeFirstRunProtocol(protocol?: string): HomeFirstRunProtocol {
  return String(protocol || '').toUpperCase() === 'HTTP' ? 'HTTP' : 'MQTT'
}

function unwrapProtocolServiceList(response: any) {
  return response?.data?.data || response?.data || response || {}
}

function unwrapVoucherTypeOptions(response: any) {
  return response?.data?.data || response?.data || response || {}
}

function findHTTPServiceIdentifier(services: any[]): string {
  const service = services.find((item) => {
    const identifier = String(item?.service_identifier || item?.value || item?.id || '').trim()
    const name = String(item?.name || item?.label || '').trim()
    return /http/i.test(identifier) || /http/i.test(name)
  })
  return String(service?.service_identifier || service?.value || service?.id || '').trim()
}

function getVoucherOptionText(option: any): string {
  return [
    option?.key,
    option?.value,
    option?.label,
    option?.name,
    option?.service_identifier,
    typeof option === 'string' ? option : ''
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
}

function normalizeVoucherOptions(raw: any): Array<{ key: string; value: string; label: string }> {
  if (Array.isArray(raw)) {
    return raw
      .map((item) => ({
        key: String(item?.key || item?.value || item?.id || item?.name || '').trim(),
        value: String(item?.value || item?.id || item?.key || item?.name || '').trim(),
        label: String(item?.label || item?.name || item?.key || item?.value || '').trim()
      }))
      .filter((item) => item.value)
  }
  if (raw && typeof raw === 'object') {
    return Object.entries(raw)
      .map(([key, value]) => ({
        key,
        value: String(value || key).trim(),
        label: key
      }))
      .filter((item) => item.value)
  }
  return []
}

async function resolveHomeFirstRunVoucherType(protocol: HomeFirstRunProtocol, protocolType: string): Promise<string> {
  if (protocol === 'MQTT') return 'ACCESSTOKEN'

  const response = await deviceConfigVoucherType({ device_type: '1', protocol_type: protocolType })
  const options = normalizeVoucherOptions(unwrapVoucherTypeOptions(response))
  const preferred = options.find((option) => /access|token|bearer/i.test(getVoucherOptionText(option)))
  const selected = preferred || options[0]
  if (!selected?.value) {
    throw new Error('HTTP 接入服务没有返回可用凭证类型。请先完善 HTTP 服务/协议插件配置，或先选择 MQTT 生成第一台设备。')
  }
  return selected.value
}

async function resolveHomeFirstRunDeviceProtocol(protocol: HomeFirstRunProtocol): Promise<string> {
  if (protocol === 'MQTT') return 'MQTT'

  const response = await deviceProtocolServiceList({ device_type: '1' })
  const data = unwrapProtocolServiceList(response)
  const serviceIdentifier = findHTTPServiceIdentifier(Array.isArray(data.service) ? data.service : [])
  if (!serviceIdentifier) {
    throw new Error('没有找到可用的 HTTP 接入服务。请先在服务/协议插件里配置 HTTP 服务，或先选择 MQTT 生成第一台设备。')
  }
  return serviceIdentifier
}

export async function createHomeFirstRunFirstDevice(
  options: { userInfo?: Record<string, unknown> | null; protocol?: HomeFirstRunProtocol } = {}
): Promise<HomeFirstRunQuickCreateResult> {
  assertHomeFirstRunTenantContext(options.userInfo)

  const protocol = normalizeHomeFirstRunProtocol(options.protocol)
  const protocolType = await resolveHomeFirstRunDeviceProtocol(protocol)
  const voucherType = await resolveHomeFirstRunVoucherType(protocol, protocolType)
  const suffix = buildQuickCreateSuffix()
  const templateName = `首台设备物模型 ${suffix}`
  const configName = `首台设备 ${protocol} 配置 ${suffix}`
  const deviceName = `第一台设备 ${suffix}`

  const templateResult = await deviceTemplateAdd({
    name: templateName,
    author: 'AetherLink',
    version: '1.0.0',
    label: 'first-device',
    description: '首页首次接入向导自动创建的默认物模型'
  })
  const templateId = requireCreatedId(templateResult, '物模型')

  const configResult = await deviceConfigAdd({
    name: configName,
    device_template_id: templateId,
    device_type: '1',
    device_conn_type: 'A',
    protocol_type: protocolType,
    voucher_type: voucherType,
    protocol_config: '{}',
    additional_info: JSON.stringify({
      source: 'home_first_run_quick_create',
      preferred_protocol: protocol,
      resolved_protocol_type: protocolType,
      resolved_voucher_type: voucherType,
      created_at: new Date().toISOString()
    }),
    description: `首页首次接入向导自动创建的 ${protocol} 直连设备配置`
  })
  const configId = requireCreatedId(configResult, '设备配置')

  const deviceResult = await deviceAdd({
    name: deviceName,
    device_number: `FIRST-${suffix}`,
    device_config_id: configId,
    access_way: 'A',
    protocol,
    label: 'first-device',
    description: '首页首次接入向导自动创建的第一台设备'
  })
  const deviceId = requireCreatedId(deviceResult, '设备')

  return {
    templateId,
    configId,
    deviceId,
    deviceName,
    protocol,
    configName
  }
}
