import { createLogger } from '@/utils/logger'

const logger = createLogger('DistributionCommandPayload')

export function buildCommandPayload(paramsData: any[]) {
  const payload: Record<string, any> = {}
  paramsData.forEach((item) => {
    if (!item?.data_identifier) return
    payload[item.data_identifier] = item[item.data_identifier]
  })
  return payload
}

export function parseCommandParamsTemplate(rawParams: unknown) {
  if (!rawParams) return []
  if (Array.isArray(rawParams)) return rawParams
  if (typeof rawParams !== 'string') return []

  try {
    const parsed = JSON.parse(rawParams)
    return Array.isArray(parsed) ? parsed : []
  } catch (error) {
    logger.error('Failed to parse command params template:', error)
    return []
  }
}

export function commandParamsForIdentifier(options: any[] | undefined, identifier: string) {
  const option = options?.find((item: any) => item.data_identifier === identifier)
  return parseCommandParamsTemplate(option?.params)
}
