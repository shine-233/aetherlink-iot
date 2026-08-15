/**
 * 文件用途：为 PageTab 组件生成主题相关 CSS 变量。
 * 核心逻辑：把主色转换为浅色、暗色和透明度颜色，供不同页签状态复用。
 * 主要逻辑：createTabCssVars 接收 activeColor，组合颜色工具输出 PageTabCssVars。
 * 关键注意事项：调用方应传入合法 CSS 颜色值，避免颜色转换结果不可用。
 * 重构建议：建议为颜色转换失败和暗色模式组合补充纯函数测试。
 */
import { addColorAlpha, transformColorWithOpacity } from '@aetherlink/utils'
import type { PageTabCssVars, PageTabCssVarsProps } from '../../types'

/** The active color of the tab */
export const ACTIVE_COLOR = '#1890ff'

function createCssVars(props: PageTabCssVarsProps) {
  const cssVars: PageTabCssVars = {
    '--soy-primary-color': props.primaryColor,
    '--soy-primary-color1': props.primaryColor1,
    '--soy-primary-color2': props.primaryColor2,
    '--soy-primary-color-opacity1': props.primaryColorOpacity1,
    '--soy-primary-color-opacity2': props.primaryColorOpacity2,
    '--soy-primary-color-opacity3': props.primaryColorOpacity3
  }

  return cssVars
}

export function createTabCssVars(primaryColor: string) {
  const cssProps: PageTabCssVarsProps = {
    primaryColor,
    primaryColor1: transformColorWithOpacity(primaryColor, 0.1, '#ffffff'),
    primaryColor2: transformColorWithOpacity(primaryColor, 0.3, '#000000'),
    primaryColorOpacity1: addColorAlpha(primaryColor, 0.1),
    primaryColorOpacity2: addColorAlpha(primaryColor, 0.15),
    primaryColorOpacity3: addColorAlpha(primaryColor, 0.3)
  }

  return createCssVars(cssProps)
}
