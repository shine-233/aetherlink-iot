import { createLogger } from '@/utils/logger'

const logger = createLogger('DistributionAttributePayload')

export function formatAttributeValue(value: any) {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch (error) {
      logger.error('Failed to stringify attribute value:', error)
      return ''
    }
  }
  return value
}

export function normalizeAttributeItem(item: any) {
  const type = (item.data_type || typeof item.value || 'string').toString().toLowerCase()
  return {
    ...item,
    checked: false,
    attributeType: type,
    inputValue: type === 'number' ? Number(item.value ?? '') : formatAttributeValue(item.value)
  }
}

export function parseBooleanValue(value: any) {
  if (typeof value === 'boolean') return value
  if (value === 'true' || value === '1' || value === 1) return true
  if (value === 'false' || value === '0' || value === 0) return false
  return Boolean(value)
}

export function parseNumberValue(value: any) {
  if (typeof value === 'number') return value
  const num = Number(value)
  return Number.isNaN(num) ? value : num
}

export function parseJsonValue(value: any) {
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch (error) {
    logger.warn('attribute payload JSON parse failed:', error)
    return value
  }
}

export const getDescriptionText = (item: any) => item?.description_cn || item?.description || ''

export function buildAttributePayload(attributeList: any[]) {
  const payload: Record<string, any> = {}
  attributeList
    .filter((item) => item.checked)
    .forEach((item) => {
      const key = item.key || item.data_identifier || item.data_name
      if (!key) return

      let value = item.inputValue
      switch (item.attributeType) {
        case 'number':
          value = parseNumberValue(value)
          break
        case 'boolean':
          value = parseBooleanValue(value)
          break
        case 'object':
        case 'array':
        case 'json':
          value = parseJsonValue(value)
          break
        default:
          break
      }
      payload[key] = value
    })
  return payload
}
