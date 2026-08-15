/**
 * Keeps the explicitly selected Native-board tenant context for the current
 * authenticated browser session.
 *
 * This is only a UX hint for SYS_ADMIN navigation. The backend remains the
 * authority: every board read/write still validates the tenant against the
 * authenticated claims. The entry is scoped to a stable user identity so a
 * second account in the same browser cannot inherit the previous account's
 * selected tenant.
 */

const STORAGE_KEY = 'aetherlink-native-board-tenant-context'

type ContextEntry = {
  userKey: string
  tenantId: string
  updatedAt: number
}
function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function firstString(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
    if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  }
  return ''
}

function userKey(userInfo: unknown): string {
  const record = asRecord(userInfo)
  if (!record) return ''
  return firstString(record, ['id', 'userId', 'user_id', 'userName', 'username', 'email'])
}

function getSessionStorage(): Storage | null {
  try {
    return typeof window === 'undefined' ? null : window.sessionStorage
  } catch {
    return null
  }
}

export function readNativeBoardTenantContext(userInfo: unknown): string {
  const currentUserKey = userKey(userInfo)
  if (!currentUserKey) return ''

  const storage = getSessionStorage()
  if (!storage) return ''

  try {
    const raw = storage.getItem(STORAGE_KEY)
    if (!raw) return ''
    const entry = JSON.parse(raw) as Partial<ContextEntry>
    if (entry.userKey !== currentUserKey || typeof entry.tenantId !== 'string') return ''
    return entry.tenantId.trim()
  } catch {
    return ''
  }
}

export function writeNativeBoardTenantContext(userInfo: unknown, tenantId: string | null | undefined): void {
  const currentUserKey = userKey(userInfo)
  const storage = getSessionStorage()
  if (!storage || !currentUserKey) return

  const normalizedTenantId = String(tenantId || '').trim()
  try {
    if (!normalizedTenantId) {
      storage.removeItem(STORAGE_KEY)
      return
    }
    const entry: ContextEntry = {
      userKey: currentUserKey,
      tenantId: normalizedTenantId,
      updatedAt: Date.now()
    }
    storage.setItem(STORAGE_KEY, JSON.stringify(entry))
  } catch {
    // Storage can be unavailable in privacy mode; board operations must still
    // work because this cache is not part of the authorization contract.
  }
}

export function clearNativeBoardTenantContext(): void {
  const storage = getSessionStorage()
  if (!storage) return
  try {
    storage.removeItem(STORAGE_KEY)
  } catch {
    // Ignore unavailable session storage.
  }
}
