import { describe, expect, it } from 'vitest'
import { darkTheme } from 'naive-ui'
import { ensureNaiveDarkTheme, naiveDarkThemeRef } from '../index'

describe('ensureNaiveDarkTheme', () => {
  it('lazily loads naive-ui darkTheme once and caches it', async () => {
    expect(naiveDarkThemeRef.value).toBeNull()
    const [first, second] = await Promise.all([ensureNaiveDarkTheme(), ensureNaiveDarkTheme()])
    expect(first).toBe(darkTheme)
    expect(second).toBe(darkTheme)
    expect(naiveDarkThemeRef.value).toBe(darkTheme)
    expect(await ensureNaiveDarkTheme()).toBe(darkTheme)
  })
})
