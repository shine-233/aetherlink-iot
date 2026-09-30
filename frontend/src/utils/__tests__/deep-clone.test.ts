/**
 * 文件用途：验证智能深拷贝工具对 Vue 响应式对象与常见内置类型的处理。
 * 核心逻辑：smartDeepClone 先解包 unref/toRaw 再 structuredClone（失败降级 JSON）；
 *   forceJSON 路径（safeDeepClone）只保留 JSON 可表达的结构。
 * 关键注意事项：structuredClone 在 Node 18+/现代浏览器可用；断言聚焦"无响应式代理泄漏"
 *   与"类型形状保持"两个契约，不重复 structuredClone 自身的语义测试。
 * 重构建议：若后续加入自定义克隆协议（如 toJSON 之外的钩子），需在本文件补契约用例。
 */
import { describe, expect, it } from 'vitest'
import { isReactive } from 'vue'
import { reactive, ref } from 'vue'
import { batchDeepClone, safeDeepClone, simpleDeepClone, smartDeepClone } from '../deep-clone'

describe('smartDeepClone', () => {
  it('普通对象与嵌套数组深拷贝且互不共享引用', () => {
    const source = { a: 1, nested: { list: [1, 2, { deep: true }] } }
    const cloned = smartDeepClone(source)

    expect(cloned).toEqual(source)
    expect(cloned).not.toBe(source)
    expect(cloned.nested).not.toBe(source.nested)
    expect(cloned.nested.list[2]).not.toBe(source.nested.list[2])

    cloned.nested.list[2].deep = false
    expect(source.nested.list[2].deep).toBe(true)
  })

  it('Vue reactive 对象被解包为普通对象（无代理泄漏）', () => {
    const source = reactive({ outer: { inner: 'value' } })
    const cloned = smartDeepClone(source)

    expect(cloned).toEqual({ outer: { inner: 'value' } })
    expect(isReactive(cloned)).toBe(false)
    expect(isReactive((cloned as { outer: unknown }).outer)).toBe(false)
  })

  it('Vue ref 输入解包为其原始值', () => {
    const source = ref({ count: 3 })
    const cloned = smartDeepClone(source)

    expect(cloned).toEqual({ count: 3 })
    expect(isReactive(cloned)).toBe(false)
  })

  it('Set 与 Map 成员被克隆', () => {
    const source = { tags: new Set(['a', 'b']), meta: new Map([['k', { v: 1 }]]) }
    const cloned = smartDeepClone(source)

    expect(cloned.tags).toBeInstanceOf(Set)
    expect(cloned.tags).not.toBe(source.tags)
    expect(cloned.meta).toBeInstanceOf(Map)
    expect(cloned.meta).not.toBe(source.meta)
    expect(cloned.meta.get('k')).toEqual({ v: 1 })
    expect(cloned.meta.get('k')).not.toBe(source.meta.get('k'))
  })

  it('Date 保持类型与时间值', () => {
    const when = new Date('2026-09-30T00:00:00Z')
    const cloned = smartDeepClone({ when })

    expect(cloned.when).toBeInstanceOf(Date)
    expect((cloned.when as Date).getTime()).toBe(when.getTime())
    expect(cloned.when).not.toBe(when)
  })

  it('null/undefined 原样返回', () => {
    expect(smartDeepClone(null)).toBe(null)
    expect(smartDeepClone(undefined)).toBe(undefined)
  })
})

describe('降级路径与便捷封装', () => {
  it('forceJSON（safeDeepClone）丢弃 Set 等非 JSON 结构的内容', () => {
    const source = { tags: new Set(['a']), when: new Date('2026-09-30T00:00:00Z') }
    const cloned = safeDeepClone(source)

    // JSON 路径下 Set 序列化为空对象，Date 序列化为 ISO 字符串
    expect(cloned.tags).toEqual({})
    expect(typeof cloned.when).toBe('string')
  })

  it('simpleDeepClone 与 smartDeepClone 行为一致', () => {
    const source = { list: [1, { x: 2 }] }
    expect(simpleDeepClone(source)).toEqual(source)
  })

  it('batchDeepClone 批量克隆数组且互不共享引用', () => {
    const items = [{ id: 1 }, { id: 2 }]
    const cloned = batchDeepClone(items)

    expect(cloned).toEqual(items)
    expect(cloned[0]).not.toBe(items[0])
    expect(cloned[1]).not.toBe(items[1])
  })
})
