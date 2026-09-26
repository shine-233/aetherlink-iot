/**
 * 文件用途：白标（TB-47）翻译覆盖前端合并逻辑的单元测试（纯函数，无 DOM / 无网络）。
 * 核心逻辑：锁定 applyTranslationOverrides / pickOverridesForFolder /
 *   whitelabelFolderForLocale 的关键契约——覆盖值恒胜出、入参不被改写、
 *   非字符串/空值覆盖被剔除、平铺键保持平铺（不拆点号嵌套）。
 * 关键注意事项：本仓库语言目录为平铺点号键形态，vue-i18n 靠字面量平铺键回退解析，
 *   若有人把合并改成"拆点号深合并"，这里的多条用例会立即变红。
 * 重构建议：语言目录结构变化时先改本文件的负向用例预期，再动实现。
 */
import { describe, expect, it } from 'vitest'
import {
  applyTranslationOverrides,
  pickOverridesForFolder,
  whitelabelFolderForLocale
} from '../whitelabel-override'

describe('whitelabel-override merge logic (TB-47)', () => {
  it('applyTranslationOverrides: 覆盖值胜出且不改写入参', () => {
    const messages = {
      'page.customer.title': 'Customer',
      'page.device.title': 'Device',
      custom: { nested: 'keep' }
    }
    const overrides = { 'page.customer.title': '客户中心' }

    const merged = applyTranslationOverrides(messages, overrides)

    expect(merged['page.customer.title']).toBe('客户中心')
    expect(merged['page.device.title']).toBe('Device')
    expect(merged.custom).toEqual({ nested: 'keep' })
    // 合并不得改写共享的静态目录引用
    expect(messages['page.customer.title']).toBe('Customer')
  })

  it('applyTranslationOverrides: 平铺点号键保持平铺（不拆成嵌套对象）', () => {
    const merged = applyTranslationOverrides(
      { 'route.mobile-app': 'Mobile' },
      { 'route.mobile-app_app-center': 'App Center' }
    )
    // vue-i18n 依赖 message[key] 字面量平铺键回退解析，拆点号会破坏解析
    expect(Object.prototype.hasOwnProperty.call(merged, 'route.mobile-app_app-center')).toBe(true)
    expect((merged as Record<string, any>)['route']).toBeUndefined()
  })

  it('applyTranslationOverrides: 非字符串与空串覆盖被剔除（fail-open 到静态目录）', () => {
    const merged = applyTranslationOverrides(
      { 'page.a.title': 'A' },
      { 'page.a.title': '', 'page.b.title': 42 as unknown as string, 'page.c.title': 'C' }
    )
    expect(merged['page.a.title']).toBe('A')
    // 非字符串覆盖被剔除且不污染目录（该键原本不存在）
    expect(merged['page.b.title']).toBeUndefined()
    expect(merged['page.c.title']).toBe('C')
  })

  it('applyTranslationOverrides: 空覆盖返回等价目录', () => {
    const messages = { 'page.a.title': 'A' }
    const merged = applyTranslationOverrides(messages, {})
    expect(merged).toEqual(messages)
    expect(merged).not.toBe(messages)
  })

  it('pickOverridesForFolder: 按 lang 目录取覆盖，形态非法返回空对象', () => {
    const translations = {
      'zh-cn': { 'page.customer.title': '客户中心' },
      'en-us': { 'page.customer.title': 'Customer Center', broken: null }
    }
    expect(pickOverridesForFolder(translations, 'zh-cn')).toEqual({ 'page.customer.title': '客户中心' })
    expect(pickOverridesForFolder(translations, 'en-us')).toEqual({ 'page.customer.title': 'Customer Center' })
    expect(pickOverridesForFolder(translations, 'fr-fr')).toEqual({})
    expect(pickOverridesForFolder(null, 'zh-cn')).toEqual({})
    expect(pickOverridesForFolder('oops', 'zh-cn')).toEqual({})
    expect(pickOverridesForFolder({ 'zh-cn': 'oops' }, 'zh-cn')).toEqual({})
  })

  it('whitelabelFolderForLocale: 四语言映射正确，未知 locale 返回 null', () => {
    expect(whitelabelFolderForLocale('zh-CN')).toBe('zh-cn')
    expect(whitelabelFolderForLocale('en-US')).toBe('en-us')
    expect(whitelabelFolderForLocale('es-ES')).toBe('es-es')
    expect(whitelabelFolderForLocale('fr-FR')).toBe('fr-fr')
    expect(whitelabelFolderForLocale('de-DE')).toBeNull()
  })
})
