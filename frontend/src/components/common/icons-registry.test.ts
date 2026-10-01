import { describe, expect, it } from 'vitest'

import { icons } from './icons-registry'

describe('icons 合并注册表（核心懒加载 + 设备/可视化同步注册）', () => {
  it('注册表包含核心、设备、可视化三组图标名', () => {
    const names = Object.keys(icons)

    // 核心图标清单为 592 个（见 icons-registry-core-manifest.ts），设备/可视化另行追加
    expect(names.length).toBeGreaterThan(592)
    expect(names).toContain('Add')
    expect(names).toContain('BatteryCharging')
    expect(names).toContain('Wifi')
  })

  it('按名称解析核心图标返回组件（走分组懒加载）', async () => {
    const component = icons.AddOutline
    expect(component).toBeDefined()
  })

  it('未注册的图标名返回 undefined 而不是抛错组件', () => {
    expect((icons as Record<string, unknown>).NoSuchIcon).toBeUndefined()
    expect((icons as Record<string, unknown>).constructor).toBeUndefined()
  })
})
