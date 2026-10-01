import type { GeneratedRoute } from '@elegant-router/types'

// 客户管理（ThingsBoard customers 对标，122.sql）。views/customer/list/index.vue 提供
// 客户档案增删改查与设备分配工作台；菜单行由 backend/sql/122.sql 的 sys_ui_elements 种子驱动。
export const customerRoutes: GeneratedRoute[] = [
  {
    name: 'customer',
    path: '/customer',
    component: 'layout.base',
    meta: {
      title: 'customer',
      i18nKey: 'route.customer'
    },
    children: [
      {
        name: 'customer_list',
        path: '/customer/list',
        component: 'view.customer_list',
        meta: {
          title: 'customer_list',
          i18nKey: 'route.customer_list'
        }
      }
    ]
  }
]
