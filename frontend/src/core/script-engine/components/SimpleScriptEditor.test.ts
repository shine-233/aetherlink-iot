import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import SimpleScriptEditor from './SimpleScriptEditor.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'en-US' } } })
}))

vi.mock('@/store/modules/theme', () => ({
  useThemeStore: () => ({ darkMode: false })
}))

vi.mock('vue-codemirror6', () => {
  throw new Error('editor chunk unavailable')
})

describe('SimpleScriptEditor fallback', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('keeps a local plain-text input when the CodeMirror chunk cannot load', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const wrapper = mount(SimpleScriptEditor, {
      props: {
        modelValue: 'return data',
        placeholder: '请求前处理脚本',
        height: '300px',
        showTemplates: false
      }
    })

    await flushPromises()
    await wrapper.vm.$nextTick()

    const fallback = wrapper.get('[data-editor-fallback="plain-text"]')
    expect(fallback.element).toBeInstanceOf(HTMLTextAreaElement)
    expect(fallback.attributes('placeholder')).toBe('请求前处理脚本')
    expect(fallback.attributes('style')).toContain('height: 300px')
    expect((fallback.element as HTMLTextAreaElement).value).toBe('return data')

    await fallback.setValue('return { ok: true }')
    expect(wrapper.emitted('update:modelValue')).toContainEqual(['return { ok: true }'])
  })
})
