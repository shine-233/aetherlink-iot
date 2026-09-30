import type { Component } from 'vue'

/**
 * 文件用途：提供「按需解析」的图标注册表构造能力。
 *
 * 核心逻辑：用 Proxy 暴露一个形如 `icons.Foo` 的对象，但只有在属性真正被访问
 * 时才调用 resolve 创建组件，从而把图标组件的加载从「模块初始化即全量注册」
 * 推迟到「真正渲染时」。
 *
 * 关键注意事项：
 * - ownKeys / getOwnPropertyDescriptor 必须实现，否则 `Object.keys(icons)` 与
 *   对象展开（`{ ...icons }`）拿不到任何图标，图标选择器会变空。
 * - 解析结果需要缓存，保证同一个图标多次访问拿到同一个组件实例，避免重复挂载。
 */
export interface LazyIconRegistryOptions {
  /** 注册表中可用的图标名称（决定 Object.keys 的结果） */
  names: readonly string[]
  /** 按需解析单个图标 */
  resolve: (name: string) => Component
}

export function createLazyIconRegistry(options: LazyIconRegistryOptions): Record<string, Component> {
  const { resolve } = options
  const names = [...new Set(options.names)]
  const nameSet = new Set(names)
  const cache = new Map<string, Component>()

  const getIcon = (name: string) => {
    const cached = cache.get(name)
    if (cached) return cached

    const component = resolve(name)
    cache.set(name, component)
    return component
  }

  return new Proxy({} as Record<string, Component>, {
    get(_target, key) {
      if (typeof key !== 'string') return undefined
      return getIcon(key)
    },
    has(_target, key) {
      return typeof key === 'string' && nameSet.has(key)
    },
    ownKeys() {
      return names
    },
    getOwnPropertyDescriptor(_target, key) {
      if (typeof key !== 'string' || !nameSet.has(key)) return undefined
      return {
        enumerable: true,
        configurable: true,
        value: getIcon(key)
      }
    }
  })
}
