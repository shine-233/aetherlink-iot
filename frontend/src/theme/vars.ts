/**
 * 文件用途：维护前端主题设置、变量和默认外观配置。
 * 核心逻辑：定义主题色、暗色模式、布局偏好或 CSS 变量入口，供主题 store 和样式层使用。
 * 关键注意事项：主题默认值会影响首屏和本地存储恢复，修改时需要确认兼容旧配置。
 * 重构建议：可将静态配置、运行时推导和 CSS 变量写入拆分成更易测试的模块。
 */
/** Create color palette vars */
function createColorPaletteVars() {
  const colors: App.Theme.ThemeColorKey[] = ['primary', 'info', 'success', 'warning', 'error']
  const colorPaletteNumbers: App.Theme.ColorPaletteNumber[] = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950]

  const colorPaletteVar = {} as App.Theme.ThemePaletteColor

  colors.forEach(color => {
    colorPaletteVar[color] = `rgb(var(--${color}-color))`
    colorPaletteNumbers.forEach(number => {
      colorPaletteVar[`${color}-${number}`] = `rgb(var(--${color}-${number}-color))`
    })
  })

  return colorPaletteVar
}

const colorPaletteVars = createColorPaletteVars()

/** Theme vars */
export const themeVars: App.Theme.ThemeToken = {
  colors: {
    ...colorPaletteVars,
    nprogress: 'rgb(var(--nprogress-color))',
    container: 'rgb(var(--container-bg-color))',
    layout: 'rgb(var(--layout-bg-color))',
    inverted: 'rgb(var(--inverted-bg-color))',
    base_text: 'rgb(var(--base-text-color))'
  },
  boxShadow: {
    header: 'var(--header-box-shadow)',
    sider: 'var(--sider-box-shadow)',
    tab: 'var(--tab-box-shadow)'
  }
}
