/**
 * 文件用途：部件库页面的纯工具函数（部件定义计数与提交前 JSON 校验）。
 * 核心逻辑：从 index.vue 抽出，供列表列渲染、创建/编辑弹窗与内置预览抽屉共用。
 * 关键注意事项：部件定义会被画布渲染与命令下发直接消费，必须是对象数组且逐项含 type/version；
 *   这里只做前端快速反馈，后端 service 层复用 ValidateWidgetDefinition 做最终校验。
 */
import { $t } from '@/locales'

/** 部件定义数量：解析失败按 0 展示，交由编辑器兜底校验。 */
export function widgetCountOf(raw: string): number {
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.length : 0
  } catch {
    return 0
  }
}

/** 提交前本地校验：必须是对象数组且逐项含 type/version。返回 null 表示通过。 */
export function validateWidgetsJson(raw: string): string | null {
  const trimmed = raw.trim()
  if (!trimmed) return $t('page.widgetBundle.widgetsRequired')
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return $t('page.widgetBundle.widgetsInvalidJson')
  }
  if (!Array.isArray(parsed)) return $t('page.widgetBundle.widgetsNotArray')
  for (let i = 0; i < parsed.length; i += 1) {
    const item = parsed[i] as Record<string, unknown> | null
    if (!item || typeof item !== 'object' || !item.type || !item.version) {
      return `${$t('page.widgetBundle.widgetsItemInvalid')} #${i}`
    }
  }
  return null
}
