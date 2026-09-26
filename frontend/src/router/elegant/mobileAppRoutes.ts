import type { GeneratedRoute } from '@elegant-router/types'

// 移动应用中心（mobile_app_bundles，TB-23）。views/mobile-app/app-center/index.vue 提供
// 安装包版本列表、上传登记、publish/archive 状态机流转与下载工作台；菜单行由
// backend/sql/133.sql 的 sys_ui_elements 种子驱动（element_code mobile-app / mobile-app_app-center）。
// 说明：gen-route 工具损坏（见 GENERATED_FILES.md），本文件为手工同步的 elegant-router 产物。
export const mobileAppRoutes: GeneratedRoute[] = [
  {
    name: 'mobile-app',
    path: '/mobile-app',
    component: 'layout.base',
    meta: {
      title: 'mobile-app',
      i18nKey: 'route.mobile-app'
    },
    children: [
      {
        name: 'mobile-app_app-center',
        path: '/mobile-app/app-center',
        component: 'view.mobile-app_app-center',
        meta: {
          title: 'mobile-app_app-center',
          i18nKey: 'route.mobile-app_app-center'
        }
      }
    ]
  }
]
