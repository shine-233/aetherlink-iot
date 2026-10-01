import type { GeneratedRoute } from '@elegant-router/types'

// 媒体库（media_files，TB-41）。views/media/library/index.vue 提供 ./files 上传资产的
// 网格预览、上传与引用感知删除工作台；菜单行由 backend/sql/129.sql 的 sys_ui_elements 种子驱动。
// 说明：gen-route 工具损坏（见 GENERATED_FILES.md），本文件为手工同步的 elegant-router 产物。
export const mediaRoutes: GeneratedRoute[] = [
  {
    name: 'media',
    path: '/media',
    component: 'layout.base',
    meta: {
      title: 'media',
      i18nKey: 'route.media'
    },
    children: [
      {
        name: 'media_library',
        path: '/media/library',
        component: 'view.media_library',
        meta: {
          title: 'media_library',
          i18nKey: 'route.media_library'
        }
      }
    ]
  }
]
