import { getPlatformApiBaseUrl } from '@/utils/common/tool'

type UserAvatarSource = Record<string, unknown> | null | undefined

const ABSOLUTE_URL_PATTERN = /^(?:[a-z]+:)?\/\//i

function parseAdditionalInfo(value: unknown): Record<string, unknown> {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return {}
    try {
      const parsed = JSON.parse(trimmed)
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
    } catch {
      return {}
    }
  }

  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

export function mergeUserAvatarIntoAdditionalInfo(value: unknown, avatarPath: string): string {
  const additionalInfo = parseAdditionalInfo(value)
  const nextAvatarPath = String(avatarPath || '').trim()
  if (nextAvatarPath) {
    additionalInfo.user_icon = nextAvatarPath
  } else {
    delete additionalInfo.user_icon
  }
  return JSON.stringify(additionalInfo)
}

export function resolveUserAvatarPath(source: UserAvatarSource): string {
  if (!source) return ''

  const additionalInfo = parseAdditionalInfo(source.additional_info ?? source.additionalInfo)
  const userIcon = typeof additionalInfo.user_icon === 'string' ? additionalInfo.user_icon.trim() : ''
  if (userIcon) return userIcon

  const avatarUrlValue = source.avatar_url ?? source.avatarUrl
  return typeof avatarUrlValue === 'string' ? avatarUrlValue.trim() : ''
}

export function resolvePlatformAssetUrl(path: string): string {
  const trimmed = String(path || '').trim()
  if (!trimmed) return ''
  if (trimmed.startsWith('data:') || ABSOLUTE_URL_PATTERN.test(trimmed)) {
    return trimmed
  }

  const normalizedPath = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  try {
    return new URL(normalizedPath, getPlatformApiBaseUrl()).href
  } catch {
    return normalizedPath
  }
}
