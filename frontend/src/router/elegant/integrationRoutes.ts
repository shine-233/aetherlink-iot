import type { GeneratedRoute } from '@elegant-router/types'

// 统一集成（TB-45 Integration 纳管管线）。views/integration/list/index.vue 提供
// 连接器实例 CRUD、上下行转换器绑定与设备绑定工作台；菜单行由 backend/sql/130.sql 的
// sys_ui_elements 种子驱动（element_code=integration / integration_list）。
export const integrationRoutes: GeneratedRoute[] = [
  {
    name: 'integration',
    path: '/integration',
    component: 'layout.base',
    meta: {
      title: 'integration',
      i18nKey: 'route.integration'
    },
    children: [
      {
        name: 'integration_list',
        path: '/integration/list',
        component: 'view.integration_list',
        meta: {
          title: 'integration_list',
          i18nKey: 'route.integration_list'
        }
      }
    ]
  }
]
