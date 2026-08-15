import { beforeEach, describe, expect, it } from 'vitest'
import {
  clearNativeBoardTenantContext,
  readNativeBoardTenantContext,
  writeNativeBoardTenantContext
} from './native-tenant-context'

describe('native board tenant context', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('round-trips a selected tenant for the same authenticated user', () => {
    const user = { id: 'sys-admin-1', authority: 'SYS_ADMIN' }

    writeNativeBoardTenantContext(user, 'tenant-1')

    expect(readNativeBoardTenantContext(user)).toBe('tenant-1')
    expect(readNativeBoardTenantContext({ id: 'sys-admin-2', authority: 'SYS_ADMIN' })).toBe('')
  })

  it('clears an empty selection and tolerates users without a stable identity', () => {
    const user = { id: 'sys-admin-1' }
    writeNativeBoardTenantContext(user, 'tenant-1')
    writeNativeBoardTenantContext(user, '')

    expect(readNativeBoardTenantContext(user)).toBe('')
    writeNativeBoardTenantContext({ authority: 'SYS_ADMIN' }, 'tenant-2')
    expect(readNativeBoardTenantContext({ authority: 'SYS_ADMIN' })).toBe('')
  })

  it('exposes an explicit cleanup operation for logout/session reset flows', () => {
    const user = { userName: 'admin@example.com' }
    writeNativeBoardTenantContext(user, 'tenant-1')
    clearNativeBoardTenantContext()

    expect(readNativeBoardTenantContext(user)).toBe('')
  })
})
