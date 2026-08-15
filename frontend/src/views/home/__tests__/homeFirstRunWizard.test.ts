import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  createHomeFirstRunFirstDevice,
  getHomeFirstRunTenantId,
  HOME_FIRST_RUN_TENANT_REQUIRED_CODE
} from '../homeFirstRunWizard'

const hoisted = vi.hoisted(() => ({
  deviceTemplateAdd: vi.fn(),
  deviceConfigAdd: vi.fn(),
  deviceAdd: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceTemplateAdd: hoisted.deviceTemplateAdd,
  deviceConfigAdd: hoisted.deviceConfigAdd,
  deviceAdd: hoisted.deviceAdd
}))

describe('homeFirstRunWizard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceTemplateAdd.mockResolvedValue({ data: { id: 'tpl-1' } })
    hoisted.deviceConfigAdd.mockResolvedValue({ data: { id: 'cfg-1' } })
    hoisted.deviceAdd.mockResolvedValue({ data: { id: 'dev-1' } })
  })

  it('normalizes supported tenant id fields from user info', () => {
    expect(getHomeFirstRunTenantId({ tenant_id: ' tenant-a ' })).toBe('tenant-a')
    expect(getHomeFirstRunTenantId({ tenantId: 'tenant-b' })).toBe('tenant-b')
    expect(getHomeFirstRunTenantId({ TenantID: 'tenant-c' })).toBe('tenant-c')
  })

  it('blocks quick create before any product or device API call when tenant context is missing', async () => {
    await expect(createHomeFirstRunFirstDevice({ userInfo: { authority: 'SYS_ADMIN' } })).rejects.toMatchObject({
      code: HOME_FIRST_RUN_TENANT_REQUIRED_CODE
    })

    expect(hoisted.deviceTemplateAdd).not.toHaveBeenCalled()
    expect(hoisted.deviceConfigAdd).not.toHaveBeenCalled()
    expect(hoisted.deviceAdd).not.toHaveBeenCalled()
  })

  it('allows quick create when tenant context exists', async () => {
    const result = await createHomeFirstRunFirstDevice({ userInfo: { tenant_id: 'tenant-1' } })

    expect(result).toMatchObject({
      templateId: 'tpl-1',
      configId: 'cfg-1',
      deviceId: 'dev-1'
    })
    expect(hoisted.deviceTemplateAdd).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfigAdd).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceAdd).toHaveBeenCalledTimes(1)
  })
})
