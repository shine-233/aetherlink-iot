export const isJsonLike = (value: string) => {
  const trimmed = value.trim()
  return trimmed.startsWith('{') || trimmed.startsWith('[')
}

export const isHttpEndpoint = (value: string) => /^https?:\/\//i.test(value.trim())

export const findConnectInfoValue = (
  connectInfo: Record<string, unknown>,
  matchers: Array<(key: string, value: string) => boolean>
) => {
  for (const [key, raw] of Object.entries(connectInfo)) {
    const value = String(raw ?? '').trim()
    const normalizedKey = key.toLowerCase()
    if (value && matchers.some((matcher) => matcher(normalizedKey, value))) return value
  }

  return ''
}

export const inferProtocol = (connectInfo: Record<string, unknown>, endpoint: string) => {
  if (isHttpEndpoint(endpoint)) return 'HTTP'

  for (const [key, raw] of Object.entries(connectInfo)) {
    const normalizedKey = key.toLowerCase()
    const normalizedValue = String(raw ?? '')
      .trim()
      .toLowerCase()
    if (
      normalizedKey.includes('http') ||
      normalizedValue.startsWith('http://') ||
      normalizedValue.startsWith('https://')
    ) {
      return 'HTTP'
    }
    if (
      normalizedKey.includes('mqtt') ||
      normalizedValue.startsWith('mqtt://') ||
      normalizedValue.startsWith('mqtts://')
    ) {
      return 'MQTT'
    }
  }

  return 'MQTT'
}

export const splitMqttEndpoint = (endpoint: string) => {
  const trimmed = endpoint.trim()
  if (!trimmed) return { host: '<mqtt-host>', port: '1883' }

  const withoutScheme = trimmed.replace(/^mqtts?:\/\//i, '')
  if (/^\d{2,5}$/.test(withoutScheme)) {
    return { host: '<mqtt-host>', port: withoutScheme }
  }
  const [hostPart, portPart] = withoutScheme.split(':')
  return {
    host: hostPart || '<mqtt-host>',
    port: portPart || '1883'
  }
}
