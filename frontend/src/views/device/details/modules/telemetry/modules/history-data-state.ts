export function buildTelemetryHistoryDownloadUrl(baseUrl: string, filePath: string | undefined | null) {
  if (!filePath) return ''

  const normalizedBase = baseUrl.replace(/\/api\/v1\/?$/, '/')
  const sanitizedPath = String(filePath).replace(/\\/g, '/').replace(/^\/+/, '')

  if (!sanitizedPath || /^[a-z][a-z\d+.-]*:/i.test(sanitizedPath) || sanitizedPath.split('/').includes('..')) {
    return ''
  }

  return new URL(sanitizedPath, normalizedBase).toString()
}
