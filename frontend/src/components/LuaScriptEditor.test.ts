import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import LuaScriptEditor from './LuaScriptEditor.vue'

vi.mock('vue-codemirror6', async () => {
  const { defineComponent, h } = await import('vue')

  return {
    default: defineComponent({
      name: 'CodeMirrorStub',
      inheritAttrs: false,
      props: {
        modelValue: { type: String, default: '' },
        disabled: Boolean,
        basic: Boolean
      },
      emits: ['update:modelValue'],
      setup(props, { attrs, emit }) {
        return () => h('textarea', {
          ...attrs,
          'data-testid': 'codemirror',
          value: props.modelValue,
          disabled: props.disabled,
          onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
        })
      }
    })
  }
})

describe('LuaScriptEditor', () => {
  it('uses the local plain-text editor without pretending JavaScript is Lua', async () => {
    const wrapper = mount(LuaScriptEditor, {
      props: {
        value: 'return 1',
        language: 'lua',
        height: 320,
        options: { readOnly: true, wordWrap: 'off' }
      }
    })
    await flushPromises()

    const editor = wrapper.get('[data-testid="codemirror"]')
    expect(editor.attributes('lang')).toBeUndefined()
    expect(editor.attributes('disabled')).toBeDefined()
    expect(wrapper.classes()).toContain('is-nowrap')
    expect(wrapper.find('.monaco-lua-editor__loading').exists()).toBe(false)
  })

  it('forwards value updates and preserves the previous exposed API honestly', async () => {
    const wrapper = mount(LuaScriptEditor, { props: { value: 'return 1' } })
    await flushPromises()

    await wrapper.get('[data-testid="codemirror"]').setValue('return 2')
    expect(wrapper.emitted('update:value')).toContainEqual(['return 2'])

    const editor = wrapper.vm as unknown as {
      getAction: (id: string) => undefined
      getValue: () => string
      setValue: (value: string) => void
    }
    expect(editor.getAction('editor.action.formatDocument')).toBeUndefined()
    expect(editor.getValue()).toBe('return 1')

    editor.setValue('return 3')
    expect(wrapper.emitted('update:value')).toContainEqual(['return 3'])
  })
})
