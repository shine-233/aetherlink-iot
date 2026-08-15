export interface RestorePlaceholderOptions {
  normalizeArray?: (value: any[], targetComponentId: string) => any[]
  normalizeObject?: (value: Record<string, any>, targetComponentId: string) => any
}

export function formatImportExportError(error: unknown, fallback = 'Unknown import/export error'): string {
  if (error instanceof Error) return error.message
  if (typeof error === 'string') return error
  if (error === null || error === undefined) return fallback

  try {
    return JSON.stringify(error)
  } catch (_stringifyError) {
    return String(error)
  }
}

function replaceCurrentComponentPlaceholder(value: string, placeholder: string, targetComponentId: string): string {
  if (value === placeholder) {
    return targetComponentId
  }

  if (!value.includes(placeholder)) {
    return value
  }

  return value.replace(new RegExp(placeholder, 'g'), targetComponentId)
}

export function restorePlaceholderDeep(
  value: any,
  placeholder: string,
  targetComponentId: string,
  options: RestorePlaceholderOptions = {}
): any {
  if (value === null || value === undefined) {
    return value
  }

  if (typeof value === 'string') {
    return replaceCurrentComponentPlaceholder(value, placeholder, targetComponentId)
  }

  if (Array.isArray(value)) {
    const restoredArray = value.map((item) => restorePlaceholderDeep(item, placeholder, targetComponentId, options))
    return options.normalizeArray ? options.normalizeArray(restoredArray, targetComponentId) : restoredArray
  }

  if (typeof value === 'object') {
    const restoredObject = Object.fromEntries(
      Object.entries(value).map(([key, childValue]) => [
        key,
        restorePlaceholderDeep(childValue, placeholder, targetComponentId, options)
      ])
    )
    return options.normalizeObject ? options.normalizeObject(restoredObject, targetComponentId) : restoredObject
  }

  return value
}
