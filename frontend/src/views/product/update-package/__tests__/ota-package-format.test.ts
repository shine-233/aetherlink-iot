/**
 * 文件用途: 升级包展示格式化工具（ota-package-format）的单元测试。
 * 核心逻辑: 覆盖时间格式化、类型标签、URL 归一化、可选值与文件名提取的边界分支。
 * 关键注意事项: $t 返回 key 本身，断言 i18n key 不变即代表文案口径未漂移。
 * 重构建议: 工具继续增多时按函数分组拆 describe。
 */
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import {
  formatOptional,
  formatTime,
  normalizePackageUrl,
  packageFileName,
  packageTypeLabel
} from '../ota-package-format'

describe('ota-package-format', () => {
  it('formats time or falls back to dash', () => {
    expect(formatTime(undefined)).toBe('-')
    expect(formatTime('')).toBe('-')
    expect(formatTime('2024-01-01')).toBe('2024-01-01 00:00:00')
  })

  it('maps package type to the unchanged i18n keys', () => {
    expect(packageTypeLabel(1)).toBe('page.product.update-package.diff')
    expect(packageTypeLabel(2)).toBe('page.product.update-package.full')
    expect(packageTypeLabel(undefined)).toBe('page.product.update-package.full')
  })

  it('normalizes relative package URLs', () => {
    expect(normalizePackageUrl('./pkg.bin')).toBe('/pkg.bin')
    expect(normalizePackageUrl('/pkg.bin')).toBe('/pkg.bin')
    expect(normalizePackageUrl('')).toBe('')
    expect(normalizePackageUrl()).toBe('')
  })

  it('formats optional values with dash fallback', () => {
    expect(formatOptional(null)).toBe('-')
    expect(formatOptional(undefined)).toBe('-')
    expect(formatOptional('')).toBe('-')
    expect(formatOptional('test')).toBe('test')
    expect(formatOptional(0)).toBe('0')
  })

  it('extracts the file name from package URLs', () => {
    expect(packageFileName('/path/to/file.bin')).toBe('file.bin')
    expect(packageFileName('\\path\\to\\win.bin')).toBe('win.bin')
    expect(packageFileName(null)).toBe('-')
  })
})
