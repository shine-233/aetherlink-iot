/**
 * 文件用途: 设备域 API 聚合入口（device.ts）与各领域子模块之间的导出合同测试。
 * 核心逻辑: 断言 device.ts 的运行时导出 == 各子模块运行时导出的并集，且是同一函数引用（纯 re-export），
 * 同时断言同一个导出名不会出现在两个子模块里，避免拆分后出现"两份实现、各改一半"的漂移。
 * 关键注意事项: 新增设备接口时放进对应领域文件并在 device.ts re-export，本测试会自动覆盖；
 * 若要删除/改名导出，需同步检查 `@/service/api` barrel 快照（index.exports.test.ts）。
 */
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/service/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn(), delete2: vi.fn() }
}))

import * as aggregator from '../device'
import * as listApi from '../device-list-api'
import * as configApi from '../device-config-api'
import * as accessApi from '../device-access-api'
import * as shareRdiApi from '../device-share-rdi-api'
import * as commandJobsApi from '../device-command-jobs-api'
import * as onboardingApi from '../device-onboarding-api'
import * as telemetryTwinApi from '../device-telemetry-twin-api'

const domainModules: Record<string, Record<string, unknown>> = {
  'device-list-api': listApi,
  'device-config-api': configApi,
  'device-access-api': accessApi,
  'device-share-rdi-api': shareRdiApi,
  'device-command-jobs-api': commandJobsApi,
  'device-onboarding-api': onboardingApi,
  'device-telemetry-twin-api': telemetryTwinApi
}

const runtimeNames = (mod: Record<string, unknown>) =>
  Object.keys(mod).filter((name) => name !== 'default' && name !== '__esModule')

describe('device API aggregator', () => {
  it('re-exports every runtime export of the split domain modules by reference', () => {
    const agg = aggregator as Record<string, unknown>
    for (const [file, mod] of Object.entries(domainModules)) {
      for (const name of runtimeNames(mod)) {
        expect(agg[name], `${file}.${name} must be re-exported by device.ts`).toBe(mod[name])
      }
    }
  })

  it('exposes nothing beyond the union of domain modules (no own implementations)', () => {
    const union = new Set(Object.values(domainModules).flatMap(runtimeNames))
    const own = runtimeNames(aggregator as Record<string, unknown>).filter((n) => !union.has(n))
    expect(own).toEqual([])
  })

  it('keeps the four split domains disjoint', () => {
    const owner = new Map<string, string>()
    const clashes: string[] = []
    for (const file of ['device-list-api', 'device-config-api', 'device-access-api', 'device-share-rdi-api']) {
      for (const name of runtimeNames(domainModules[file])) {
        const prev = owner.get(name)
        if (prev) clashes.push(`${name}: ${prev} & ${file}`)
        owner.set(name, file)
      }
    }
    expect(clashes).toEqual([])
  })

  it('keeps the historical device.ts runtime surface size', () => {
    // 拆分前 device.ts 共 89 个自有运行时导出 + 37 个来自 command-jobs/onboarding/telemetry-twin 的 re-export。
    expect(runtimeNames(aggregator as Record<string, unknown>)).toHaveLength(126)
  })
})
