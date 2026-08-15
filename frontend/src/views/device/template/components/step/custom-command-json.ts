export const CUSTOM_COMMAND_INSTRUCT_STARTER = {
  method: 'setSwitch',
  params: {
    power: true
  }
}

export interface CustomCommandJsonValidation {
  valid: boolean
  formatted?: string
  error?: string
}

function jsonErrorLocation(value: string, error: unknown) {
  const message = error instanceof Error ? error.message : String(error)
  const positionMatch = message.match(/position (\d+)/i)
  if (!positionMatch) return message

  const position = Number(positionMatch[1])
  const beforeError = value.slice(0, position)
  const line = beforeError.split('\n').length
  const column = beforeError.length - beforeError.lastIndexOf('\n')
  return `${message} (line ${line}, column ${column})`
}

export function validateCustomCommandInstruct(value: string): CustomCommandJsonValidation {
  const source = value?.trim()
  if (!source) {
    return { valid: false, error: 'JSON is required' }
  }

  try {
    const parsed = JSON.parse(source)
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      return { valid: false, error: 'Command content must be a JSON object' }
    }
    return { valid: true, formatted: JSON.stringify(parsed, null, 2) }
  } catch (error) {
    return { valid: false, error: jsonErrorLocation(source, error) }
  }
}

export function formatCustomCommandInstruct(value: string): CustomCommandJsonValidation {
  return validateCustomCommandInstruct(value)
}

export function buildCustomCommandInstructStarter() {
  return JSON.stringify(CUSTOM_COMMAND_INSTRUCT_STARTER, null, 2)
}
