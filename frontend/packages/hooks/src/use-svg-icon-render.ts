/**
 * 文件用途：生成 SVG 图标组件的渲染函数配置。
 * 核心逻辑：接收图标组件，返回可按 fontSize 和 color 渲染的函数。
 * 关键注意事项：传入组件需要兼容 Vue h 函数渲染，样式值由调用方负责合法性。
 * 重构建议：可扩展尺寸、类名和可访问性属性配置，避免调用方重复封装。
 */
import { h } from 'vue'
import type { Component } from 'vue'

/**
 * Svg icon render hook
 *
 * @param SvgIcon Svg icon component
 */
export default function useSvgIconRender(SvgIcon: Component) {
  interface IconConfig {
    /** Iconify icon name */
    icon?: string
    /** Local icon name */
    localIcon?: string
    /** Icon color */
    color?: string
    /** Icon size */
    fontSize?: number
  }

  type IconStyle = Partial<Pick<CSSStyleDeclaration, 'color' | 'fontSize'>>

  /**
   * Svg icon VNode
   *
   * @param config
   */
  const SvgIconVNode = (config: IconConfig) => {
    const { color, fontSize, icon, localIcon } = config

    const style: IconStyle = {}

    if (color) {
      style.color = color
    }
    if (fontSize) {
      style.fontSize = `${fontSize}px`
    }

    if (!icon && !localIcon) {
      return undefined
    }

    return () => h(SvgIcon, { icon, localIcon, style })
  }

  return {
    SvgIconVNode
  }
}
