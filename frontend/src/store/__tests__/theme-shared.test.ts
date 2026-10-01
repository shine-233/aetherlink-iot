/**
 * 文件用途：验证主题暗色切换对 <html> 的副作用。
 * 核心逻辑：调用 toggleCssDarkMode 后断言 dark class 与 color-scheme 同步，保证原生滚动条和表单控件跟随主题。
 */
import { afterEach, describe, expect, it } from 'vitest'
import { toggleCssDarkMode } from '../modules/theme/shared'

describe('toggleCssDarkMode', () => {
  afterEach(() => {
    document.documentElement.classList.remove('dark')
    document.documentElement.style.colorScheme = ''
  })

  it('adds the dark class and dark color-scheme', () => {
    toggleCssDarkMode(true)

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })

  it('removes the dark class and restores light color-scheme', () => {
    toggleCssDarkMode(true)
    toggleCssDarkMode(false)

    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.style.colorScheme).toBe('light')
  })
})
