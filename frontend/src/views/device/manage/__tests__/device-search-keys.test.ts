// 文件用途: 守护 REQ-58 设备列表搜索增强——锁定设备列表 searchConfigs 的筛选键集。
// 核心逻辑: 直接调用 createDeviceManageSearchConfigs 工厂,取其返回项的 key,与权威清单
//   DEVICE_SEARCH_KEYS 比对,任何键被误删/漏加(无对应契约)即 FAIL。
// 关键注意事项: 此前 17+1 个内联筛选键零断言(仅 fleet 预设子集被测),属假覆盖。searchConfigs
//   已提取为可导入工厂(device-search-configs.ts),故断言真实返回值而非解析源码。

import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import { createDeviceManageSearchConfigs } from '../device-search-configs'
import { DEVICE_SEARCH_KEYS } from '../device-search-keys'

const noopOptions = async () => []

const actual = createDeviceManageSearchConfigs(
  {},
  { getDeviceGroupOptions: noopOptions, getDeviceConfigOptions: noopOptions }
).map((item) => item.key)

describe('device-manage search keys contract (REQ-58)', () => {
  it('searchConfigs exposes exactly the authoritative key set (no missing/extra)', () => {
    expect([...actual].sort()).toEqual([...DEVICE_SEARCH_KEYS].sort())
  })

  it('includes the REQ-58 search-enhancement keys named by the customer', () => {
    // 主清单点名的增强项:设备编号/PID/固件/描述/标签/共享状态/上报前后界/自由文本。
    const enhancement = [
      'device_number',
      'pid_number',
      'firmware_version',
      'description',
      'label',
      'shared_status',
      'last_reported_after',
      'last_reported_before',
      'search'
    ]
    for (const k of enhancement) expect(actual).toContain(k)
  })

  it('includes the REQ-05b lifecycle_status filter added this session', () => {
    expect(actual).toContain('lifecycle_status')
  })

  it('has no duplicate keys', () => {
    expect(new Set(actual).size).toBe(actual.length)
  })
})
