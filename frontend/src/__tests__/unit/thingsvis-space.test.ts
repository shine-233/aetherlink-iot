/**
 * 文件用途：验证 ThingsVis 空间、看板和嵌入上下文解析。
 * 核心逻辑：构造空间参数组合，断言 iframe 或运行时上下文所需字段。
 * 关键注意事项：默认空间值涉及租户边界，不能随意写入跨租户兜底。
 * 重构建议：可继续增加多租户、空配置和兼容旧字段的参数化用例。
 */
/**
 * 文件：ThingsVis 空间解析测试。
 * 作用：验证系统管理员和租户用户映射到正确的 ThingsVis 空间。
 * 依赖：依赖 Vitest 与 ThingsVis space 工具函数。
 * 维护：用户角色、租户字段或系统空间 ID 变化时同步扩展兼容用例。
 */

import { describe, expect, it } from 'vitest'
import { isSysAdminUser, resolveThingsVisSpaceId, SYS_ADMIN_SPACE_ID } from '@/utils/thingsvis/space'

describe('thingsvis space helpers', () => {
  it('maps sys admin to the dedicated ThingsVis space', () => {
    const userInfo = {
      authority: 'SYS_ADMIN',
      roles: ['SYS_ADMIN']
    }

    expect(isSysAdminUser(userInfo)).toBe(true)
    expect(resolveThingsVisSpaceId(userInfo)).toBe(SYS_ADMIN_SPACE_ID)
  })

  it('keeps tenant users inside their tenant space', () => {
    const userInfo = {
      authority: 'TENANT_ADMIN',
      roles: ['TENANT_ADMIN'],
      tenantId: 'tenant-a'
    }

    expect(isSysAdminUser(userInfo)).toBe(false)
    expect(resolveThingsVisSpaceId(userInfo)).toBe('tenant-a')
  })

  it('supports older tenant_id payloads', () => {
    const userInfo = {
      authority: 'TENANT_USER',
      roles: ['TENANT_USER'],
      tenant_id: 'tenant-b'
    }

    expect(resolveThingsVisSpaceId(userInfo)).toBe('tenant-b')
  })
})
