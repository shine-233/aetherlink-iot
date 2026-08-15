/**
 * 文件用途: 验证动态参数存储的独立公开契约。
 * 核心逻辑: 覆盖类型推断、两种键形式、作用域查询、严格过期边界和清理。
 * 关键注意事项: 过期测试固定 Date.now()，不依赖真实时间。
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DynamicParameterStore, type DynamicParameterStorage } from './DynamicParameterStore'

describe('DynamicParameterStore', () => {
  let store: DynamicParameterStore

  beforeEach(() => {
    store = new DynamicParameterStore()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it.each([
    ['text', 'string'],
    [42, 'number'],
    [true, 'boolean'],
    [{ enabled: true }, 'object'],
    [[1, 2], 'array'],
    [null, 'object']
  ] as const)('infers %j as %s', (value, expectedType) => {
    store.store('component-1', 'parameter', value)

    expect(store.get('component-1')).toBeNull()
    expect(store.getAll()).toEqual({
      'component-1:parameter': {
        name: 'parameter',
        value,
        type: expectedType,
        scope: 'component'
      }
    })
  })

  it('stores scoped values and returns values for scoped queries', () => {
    store.store('component-1', 'first', 'one')
    store.store('component-1', 'second', 2)
    store.store('component-2', 'first', 'other')

    expect(store.get('component-1', 'first')).toBe('one')
    expect(store.getAll('component-1')).toEqual({ first: 'one', second: 2 })
  })

  it('stores a complete parameter under its direct storage key', () => {
    const parameter: DynamicParameterStorage = {
      name: 'theme',
      value: { dark: true },
      type: 'object',
      scope: 'global',
      dependencies: ['palette']
    }

    store.store('theme-key', parameter)

    expect(store.get('theme-key')).toBe(parameter)
    expect(store.getAll()).toEqual({ 'theme-key': parameter })
    expect(store.getAll('global')).toEqual({})
  })

  it('does not expire a parameter when Date.now() equals expiresAt', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1000)
    const parameter: DynamicParameterStorage = {
      name: 'boundary',
      value: 'present',
      type: 'string',
      scope: 'session',
      expiresAt: 1000
    }
    store.store('boundary-key', parameter)

    expect(store.get('boundary-key')).toBe(parameter)
    expect(store.getAll()).toEqual({ 'boundary-key': parameter })
  })

  it('removes an expired parameter when it is read', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1001)
    store.store('expired-key', {
      name: 'expired',
      value: 'stale',
      type: 'string',
      scope: 'session',
      expiresAt: 1000
    })

    expect(store.get('expired-key')).toBeNull()
    expect(store.getAll()).toEqual({})
  })

  it('removes expired parameters during all-parameter queries', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1001)
    store.store('component-1', {
      name: 'expired',
      value: 'stale',
      type: 'string',
      scope: 'component',
      expiresAt: 1000
    })
    store.store('component-1', 'active', 'fresh')

    expect(store.getAll()).toEqual({
      'component-1:active': {
        name: 'active',
        value: 'fresh',
        type: 'string',
        scope: 'component'
      }
    })
    expect(store.get('component-1')).toBeNull()
  })

  it('clears every stored parameter', () => {
    store.store('component-1', 'first', 'one')
    store.store('direct-key', {
      name: 'second',
      value: 2,
      type: 'number',
      scope: 'global'
    })

    store.clear()

    expect(store.getAll()).toEqual({})
    expect(store.get('component-1', 'first')).toBeNull()
    expect(store.get('direct-key')).toBeNull()
  })
})
