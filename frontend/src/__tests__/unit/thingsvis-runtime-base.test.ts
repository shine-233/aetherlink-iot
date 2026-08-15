/**
 * 文件用途：验证 ThingsVis 基础运行时参数和默认值契约。
 * 核心逻辑：通过小型用例覆盖运行时基址、平台字段和兼容别名。
 * 关键注意事项：这里是入口级运行时保护，变更需同步嵌入页面和预览路径。
 * 重构建议：可把重复 fixture 提炼为 ThingsVis runtime test helper。
 */
/**
 * 文件：ThingsVis 运行时基础路径测试。
 * 作用：验证 API 基础路径解析和嵌入协议别名保持 AetherLink 契约。
 * 依赖：依赖 Vitest、浏览器 window.location 和 ThingsVis 常量模块。
 * 维护：代理路径或平台别名变化时同步更新期望值和兼容性说明。
 */

import { describe, expect, it } from 'vitest'
import {
  getPlatformApiBase,
  getThingsVisApiBase,
  PLATFORM_API_BASE_PATH,
  THINGSVIS_API_PROXY_PATH,
  THINGSVIS_COMPAT_PLATFORM,
  THINGSVIS_COMPAT_PROVIDER,
  THINGSVIS_COMPAT_ALIAS
} from '@/utils/thingsvis/constants'

describe('thingsvis runtime base helpers', () => {
  it('resolves both API bases from the current host origin', () => {
    expect(getThingsVisApiBase()).toBe(`${window.location.origin}${THINGSVIS_API_PROXY_PATH}`)
    expect(getPlatformApiBase()).toBe(`${window.location.origin}${PLATFORM_API_BASE_PATH}`)
  })

  it('pins the embedded ThingsVis protocol alias to AetherLink', () => {
    expect(THINGSVIS_COMPAT_PROVIDER).toBe(THINGSVIS_COMPAT_ALIAS)
    expect(THINGSVIS_COMPAT_PLATFORM).toBe(THINGSVIS_COMPAT_ALIAS)
  })
})
