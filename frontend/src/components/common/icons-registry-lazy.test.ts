import { describe, expect, it, vi } from 'vitest'
import type { Component } from 'vue'

import { createLazyIconRegistry } from './icons-registry-lazy'

function makeStubComponent(name: string): Component {
  return { name, render: () => null }
}

describe('createLazyIconRegistry', () => {
  it('按注册名按需解析并缓存组件（同一名称只 resolve 一次）', () => {
    const resolve = vi.fn((name: string) => makeStubComponent(name))
    const registry = createLazyIconRegistry({
      names: ['Add', 'Add'],
      resolve
    })

    const first = registry.Add
    const second = registry.Add

    expect(first).toBeDefined()
    expect(second).toBe(first)
    expect(resolve).toHaveBeenCalledTimes(1)
    expect(resolve).toHaveBeenCalledWith('Add')
  })

  it('重复注册名在 Object.keys 中去重且与 ownKeys 语义一致', () => {
    const registry = createLazyIconRegistry({
      names: ['A', 'B', 'A'],
      resolve: (name) => makeStubComponent(name)
    })

    expect(Object.keys(registry)).toEqual(['A', 'B'])
    expect('A' in registry).toBe(true)
    expect('Missing' in registry).toBe(false)
  })

  it('对象展开可拿到全部图标（ownKeys + getOwnPropertyDescriptor）', () => {
    const registry = createLazyIconRegistry({
      names: ['X', 'Y'],
      resolve: (name) => makeStubComponent(name)
    })

    const spread: Record<string, Component> = { ...registry }
    expect(Object.keys(spread).sort()).toEqual(['X', 'Y'])
    expect(spread.X).toBeDefined()
    expect(spread.Y).toBeDefined()
  })

  it('未注册的名称返回 undefined，与改造前普通对象语义一致', () => {
    const resolve = vi.fn((name: string) => makeStubComponent(name))
    const registry = createLazyIconRegistry({ names: ['Add'], resolve })

    expect((registry as Record<string, unknown>).NotRegistered).toBeUndefined()
    expect(resolve).not.toHaveBeenCalled()
  })

  it('继承键（constructor/then/toString）不会被解析成组件', () => {
    const resolve = vi.fn((name: string) => makeStubComponent(name))
    const registry = createLazyIconRegistry({ names: ['Add'], resolve })

    const asRecord = registry as unknown as Record<string, unknown>
    expect(asRecord.constructor).toBeUndefined()
    expect(asRecord.then).toBeUndefined()
    expect(asRecord.toString).toBeUndefined()
    expect(resolve).not.toHaveBeenCalled()
  })

  it('符号键访问返回 undefined 且不触发解析', () => {
    const resolve = vi.fn((name: string) => makeStubComponent(name))
    const registry = createLazyIconRegistry({ names: ['Add'], resolve })

    expect(Reflect.get(registry, Symbol.iterator)).toBeUndefined()
    expect(resolve).not.toHaveBeenCalled()
  })
})
