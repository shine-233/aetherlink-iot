import type { GeneratedRoute } from '@elegant-router/types'

// 模板市场（ROADMAP P1.6）。此前 views/market/browse/index.vue 已实现打包导入
// 的预览 / 验签 / 覆盖闸门，但从未挂进路由表——页面存在却无处可达，
// 后端整条链路在前端仍是死门。这里补上注册，与 imports.ts / transform.ts 同步。
export const marketRoutes: GeneratedRoute[] = [
  {
    name: 'market',
    path: '/market',
    component: 'layout.base',
    meta: {
      title: 'market',
      i18nKey: 'route.market'
    },
    children: [
      {
        name: 'market_browse',
        path: '/market/browse',
        component: 'view.market_browse',
        meta: {
          title: 'market_browse',
          i18nKey: 'route.market_browse'
        }
      }
    ]
  }
]
