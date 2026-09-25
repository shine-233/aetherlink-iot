// 文件用途：billing 路由区的 elegant-router 手工同步生成文件（TB-17 配额页；gen-route 工具损坏勿用）。
// 核心逻辑：billing 顶级目录（layout.base）+ billing_api-quota 子页面（view.billing_api-quota）。
// 关键注意事项：与 routes.ts/imports.ts/transform.ts/typings/elegant-router.d.ts 四处条目联动，
// 漏一处即 typecheck 或运行时路由解析失败；菜单行由 backend/sql/124.sql 的 sys_ui_elements 驱动。
// 重构建议：gen-route 工具修复后由工具重新生成本文件，保持同构。
import type { GeneratedRoute } from '@elegant-router/types'

export const billingRoutes: GeneratedRoute[] = [
  {
    name: 'billing',
    path: '/billing',
    component: 'layout.base',
    meta: {
      title: 'billing',
      i18nKey: 'route.billing'
    },
    children: [
      {
        name: 'billing_api-quota',
        path: '/billing/api-quota',
        component: 'view.billing_api-quota',
        meta: {
          title: 'billing_api-quota',
          i18nKey: 'route.billing_api-quota'
        }
      }
    ]
  }
]
