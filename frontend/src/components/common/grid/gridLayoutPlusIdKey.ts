import type { GridLayoutPlusItem } from './gridLayoutPlusTypes'

/**
 * 文件用途：网格项主键协议（idKey）相关的纯函数。
 * 核心逻辑：内部统一使用 `i` 作为主键，对外事件按 idKey 补别名，保证任意主键协议兼容。
 * 关键注意事项：不要在这里改动原始项引用语义，仅补充别名字段。
 */

/** 将内部布局规范化为「每项都带 i」的结构，并在使用自定义主键时同步写回别名字段 */
export function normalizeLayout(layout: GridLayoutPlusItem[], idKey: string): GridLayoutPlusItem[] {
  const key = idKey || 'i'

  return (layout || []).map((item) => {
    const itemRecord = item as unknown as Record<string, unknown>
    const currentId = itemRecord[key] ?? itemRecord.i
    const normalized: GridLayoutPlusItem = { ...item, i: currentId as string }

    if (key !== 'i') {
      ;(normalized as unknown as Record<string, unknown>)[key] = normalized.i
    }

    return normalized
  })
}

/** 对外派发布局前补齐 idKey 别名字段 */
export function withIdKey(items: GridLayoutPlusItem[], idKey: string): GridLayoutPlusItem[] {
  const key = idKey || 'i'
  if (key === 'i') return items

  return items.map((item) => ({
    ...(item as unknown as Record<string, unknown>),
    [key]: item.i
  })) as unknown as GridLayoutPlusItem[]
}

/** 为新建项写入 idKey 别名字段，保证双字段一致 */
export function applyIdKey(item: GridLayoutPlusItem, idKey: string): GridLayoutPlusItem {
  const key = idKey || 'i'
  if (key === 'i') return item

  ;(item as unknown as Record<string, unknown>)[key] = item.i
  return item
}
