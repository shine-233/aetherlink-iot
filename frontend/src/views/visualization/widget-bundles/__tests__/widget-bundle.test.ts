/**
 * 文件用途：覆盖部件库页面纯工具函数（widgetCountOf / validateWidgetsJson）。
 * 核心逻辑：计数容错（非数组/坏 JSON 记 0）与提交前校验（空串、坏 JSON、非数组、逐项 type/version）。
 * 关键注意事项：断言文案用 i18n key（locales 已 mock 成透传 key），不绑定具体语言译文。
 */
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

import { validateWidgetsJson, widgetCountOf } from '../modules/widget-bundle'

describe('widget-bundle helpers', () => {
  describe('widgetCountOf', () => {
    it('counts items of a valid JSON array', () => {
      expect(widgetCountOf('[{"type":"gauge"},{"type":"chart"}]')).toBe(2)
    })

    it('returns 0 for non-array JSON', () => {
      expect(widgetCountOf('{"type":"gauge"}')).toBe(0)
    })

    it('returns 0 for invalid JSON', () => {
      expect(widgetCountOf('not-json')).toBe(0)
    })
  })

  describe('validateWidgetsJson', () => {
    it('rejects empty input', () => {
      expect(validateWidgetsJson('   ')).toBe('page.widgetBundle.widgetsRequired')
    })

    it('rejects invalid JSON', () => {
      expect(validateWidgetsJson('[{')).toBe('page.widgetBundle.widgetsInvalidJson')
    })

    it('rejects non-array payloads', () => {
      expect(validateWidgetsJson('{"type":"gauge"}')).toBe('page.widgetBundle.widgetsNotArray')
    })

    it('rejects items missing type or version with the item index', () => {
      expect(validateWidgetsJson('[{"type":"gauge"}]')).toBe('page.widgetBundle.widgetsItemInvalid #0')
      expect(validateWidgetsJson('[{"type":"gauge","version":"1"},{"version":"1"}]')).toBe(
        'page.widgetBundle.widgetsItemInvalid #1'
      )
      expect(validateWidgetsJson('[null]')).toBe('page.widgetBundle.widgetsItemInvalid #0')
    })

    it('accepts an object array where every item has type and version', () => {
      expect(validateWidgetsJson('[{"type":"gauge","version":"1"}]')).toBeNull()
    })
  })
})
