/**
 * 文件用途：集中导出通用图标组件注册表。
 * 核心逻辑：合并核心（懒加载）、设备与可视化三组图标，并以稳定名称向外暴露。
 * 关键注意事项：导出名称可能被路由元信息或共享组件间接引用，删除或改名需先完成调用方迁移。
 * 重构建议：可按业务域或图标来源拆分注册表，再通过当前入口维持兼容导出。
 */
import type { Component } from 'vue'

import { coreIconNames } from './icons-registry-core-manifest'
import { coreIcons } from './icons-registry-core'
import { deviceIcons } from './icons-registry-device'
import { visualizationIcons } from './icons-registry-visualization'
import { createLazyIconRegistry } from './icons-registry-lazy'

// 设备与可视化图标数量有限，保持同步注册；核心图标走懒加载
const eagerIcons: Record<string, Component> = {
  ...deviceIcons,
  ...visualizationIcons
}

export const icons = createLazyIconRegistry({
  names: [...coreIconNames, ...Object.keys(eagerIcons)],
  resolve: (name) => eagerIcons[name] ?? coreIcons[name]
})
