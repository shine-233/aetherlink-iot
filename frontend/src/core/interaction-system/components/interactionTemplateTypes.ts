/**
 * 文件用途：交互模板的共享类型定义。
 * 核心逻辑：模板选择器与模板预览组件使用同一份模板结构。
 */
import type { Component } from 'vue'

import type { InteractionConfig } from './interactionPreviewTypes'

export interface InteractionTemplate {
  id: string
  name: string
  description: string
  category: string
  icon: Component
  color: string
  config: InteractionConfig[]
  tags?: string[]
}
