import { getCachedDeviceTemplateDetail } from '@/utils/thingsvis/template-detail-cache'
import { hasThingsVisChartContent } from '@/utils/thingsvis/template-presets'
import { createLogger } from '@/utils/logger'

const logger = createLogger('DeviceDetailTabPlan')

type DeviceDetailData = Record<string, any>

export type DeviceTabPlan = {
  deviceType: string
  hiddenKeys: Set<string>
  isRdi: boolean
}

export type DeviceChartTabResolution = {
  shouldShowChart: boolean
}

type DeviceTabPlanContext = {
  config: DeviceDetailData['device_config']
  deviceType: string
  templateId: string | number | undefined
  deviceConfigName: unknown
  chartAvailability: boolean | null
  isRdi: boolean
}

const templateChartAvailabilityCache = new Map<string, boolean>()

function normalizeTemplateId(templateId: string | number | undefined) {
  return String(templateId || '').trim()
}

function isRdiDevice(deviceData: DeviceDetailData): boolean {
  if (!deviceData) return false

  const dn = deviceData.device_number || ''
  if (/^[A-Za-z0-9]{12}$/.test(dn)) return true

  try {
    const info =
      typeof deviceData.additional_info === 'string'
        ? JSON.parse(deviceData.additional_info)
        : deviceData.additional_info
    if (info && (info.rdi_config || info.rdi_system_info)) return true
  } catch {
    /* ignore */
  }

  return false
}

function getDeviceConfig(data: DeviceDetailData) {
  return data?.device_config
}

function getDeviceTypeForTabs(data: DeviceDetailData) {
  return getDeviceConfig(data)?.device_type || ''
}

function getDeviceTemplateId(data: DeviceDetailData) {
  return getDeviceConfig(data)?.device_template_id
}

function resolveDeviceChartAvailabilityFlag(data: DeviceDetailData): boolean | null {
  const directFlag = data?.has_chart_config
  if (typeof directFlag === 'boolean') return directFlag

  const configFlag = data?.device_config?.has_chart_config
  return typeof configFlag === 'boolean' ? configFlag : null
}

function createDeviceTabPlanContext(data: DeviceDetailData): DeviceTabPlanContext {
  return {
    config: getDeviceConfig(data),
    deviceType: getDeviceTypeForTabs(data),
    templateId: getDeviceTemplateId(data),
    deviceConfigName: data?.device_config_name,
    chartAvailability: resolveDeviceChartAvailabilityFlag(data),
    isRdi: isRdiDevice(data)
  }
}

function resolveDeviceTabHiddenKeys(context: DeviceTabPlanContext) {
  const hiddenKeys = new Set<string>()

  applyConfigDrivenHiddenTabs(hiddenKeys, {
    config: context.config,
    deviceType: context.deviceType,
    templateId: context.templateId,
    deviceConfigName: context.deviceConfigName,
    chartAvailability: context.chartAvailability
  })

  if (!context.isRdi) {
    hiddenKeys.add('rdi')
  } else {
    hiddenKeys.delete('chart')
  }

  return hiddenKeys
}

function shouldHideDeviceAnalysisTab(deviceType: string, deviceConfigName: unknown) {
  return deviceType !== '2' || !deviceConfigName
}

function shouldHideJoinTab(deviceType: string) {
  return deviceType === '3'
}

async function shouldHideChartTab(templateId: string | number | undefined, chartAvailability: boolean | null = null) {
  const normalizedTemplateId = normalizeTemplateId(templateId)
  if (!normalizedTemplateId) {
    return true
  }

  if (chartAvailability !== null) {
    templateChartAvailabilityCache.set(normalizedTemplateId, chartAvailability)
    return !chartAvailability
  }

  const hasTemplateChart = await resolveTemplateHasChartContent(normalizedTemplateId)
  return !hasTemplateChart
}

function shouldHideChartTabBeforeTemplateLoad(templateId: string | number | undefined, chartAvailability: boolean | null) {
  const normalizedTemplateId = normalizeTemplateId(templateId)
  if (!normalizedTemplateId) {
    return true
  }

  if (chartAvailability !== null) {
    templateChartAvailabilityCache.set(normalizedTemplateId, chartAvailability)
    return !chartAvailability
  }

  if (!templateChartAvailabilityCache.has(normalizedTemplateId)) {
    return true
  }
  return !templateChartAvailabilityCache.get(normalizedTemplateId)
}

function applyConfigDrivenHiddenTabs(
  hiddenKeys: Set<string>,
  options: {
    config: DeviceDetailData['device_config']
    deviceType: string
    templateId: string | number | undefined
    deviceConfigName: unknown
    chartAvailability: boolean | null
  }
) {
  const { config, deviceType, templateId, deviceConfigName, chartAvailability } = options

  if (!config) {
    if (!deviceConfigName) {
      hiddenKeys.add('device-analysis')
      hiddenKeys.add('chart')
    }
    return
  }

  if (shouldHideDeviceAnalysisTab(deviceType, deviceConfigName)) {
    hiddenKeys.add('device-analysis')
  }

  if (shouldHideJoinTab(deviceType)) {
    hiddenKeys.add('join')
  }

  if (shouldHideChartTabBeforeTemplateLoad(templateId, chartAvailability)) {
    hiddenKeys.add('chart')
  }
}

const resolveTemplateHasChartContent = async (templateId?: string | number) => {
  const normalizedTemplateId = normalizeTemplateId(templateId)
  if (!normalizedTemplateId) return false

  if (templateChartAvailabilityCache.has(normalizedTemplateId)) {
    return templateChartAvailabilityCache.get(normalizedTemplateId) || false
  }

  try {
    const res = await getCachedDeviceTemplateDetail(normalizedTemplateId)
    const template = res?.data || {}
    const hasChart =
      hasThingsVisChartContent(template?.web_chart_config) || hasThingsVisChartContent(template?.app_chart_config)

    templateChartAvailabilityCache.set(normalizedTemplateId, hasChart)
    return hasChart
  } catch (err) {
    logger.warn('[DeviceDetailTabPlan] Failed to load template chart capability.', {
      templateId: normalizedTemplateId,
      error: err instanceof Error ? err.message : err
    })
    templateChartAvailabilityCache.set(normalizedTemplateId, false)
    return false
  }
}

export function createDeviceTabPlan(data: DeviceDetailData): DeviceTabPlan {
  const context = createDeviceTabPlanContext(data)
  const hiddenKeys = resolveDeviceTabHiddenKeys(context)

  return {
    deviceType: context.deviceType,
    hiddenKeys,
    isRdi: context.isRdi
  }
}

export async function resolveDeviceChartTabResolution(data: DeviceDetailData): Promise<DeviceChartTabResolution | null> {
  const context = createDeviceTabPlanContext(data)
  if (context.isRdi) {
    return {
      shouldShowChart: true
    }
  }

  if (!context.config || !context.templateId) {
    return null
  }

  return {
    shouldShowChart: !(await shouldHideChartTab(context.templateId, context.chartAvailability))
  }
}
