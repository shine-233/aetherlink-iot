export type CommandPayloadInsight = {
  type: 'default' | 'error' | 'info' | 'success'
  titleKey: string
  descKey: string
  formatted: string
  fieldCount: number
  canFormat: boolean
}

export function buildCommandPayloadInsight(value: string): CommandPayloadInsight {
  const raw = value.trim()
  if (!raw) {
    return {
      type: 'default',
      titleKey: 'custom.commandCenter.payloadAssistantEmptyTitle',
      descKey: 'custom.commandCenter.payloadAssistantEmptyDesc',
      formatted: '',
      fieldCount: 0,
      canFormat: false
    }
  }

  try {
    const parsed = JSON.parse(raw)
    const fieldCount = Array.isArray(parsed)
      ? parsed.length
      : parsed && typeof parsed === 'object'
        ? Object.keys(parsed).length
        : 1

    return {
      type: 'success',
      titleKey: 'custom.commandCenter.payloadAssistantValidTitle',
      descKey: 'custom.commandCenter.payloadAssistantValidDesc',
      formatted: JSON.stringify(parsed, null, 2),
      fieldCount,
      canFormat: true
    }
  } catch {
    const looksLikeJson = raw.startsWith('{') || raw.startsWith('[')
    return {
      type: 'error',
      titleKey: looksLikeJson
        ? 'custom.commandCenter.payloadAssistantInvalidTitle'
        : 'custom.commandCenter.payloadAssistantPlainTitle',
      descKey: looksLikeJson
        ? 'custom.commandCenter.payloadAssistantInvalidDesc'
        : 'custom.commandCenter.payloadAssistantPlainDesc',
      formatted: '',
      fieldCount: 0,
      canFormat: false
    }
  }
}
