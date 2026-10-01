import type { GeneratedRoute } from '@elegant-router/types'

// 统一调度器（scheduler_events，TB-48）。views/scheduler/calendar/index.vue 提供月视图
// 调度日历：三套存量调度 + 注册行的统一聚合展示（来源类型着色）与 scheduler_events
// 注册面管理；菜单行由 backend/sql/137.sql 的 sys_ui_elements 种子驱动
// （element_code scheduler / scheduler_calendar）。
// 说明：gen-route 工具损坏（见 GENERATED_FILES.md），本文件为手工同步的 elegant-router 产物。
export const schedulerRoutes: GeneratedRoute[] = [
  {
    name: 'scheduler',
    path: '/scheduler',
    component: 'layout.base',
    meta: {
      title: 'scheduler',
      i18nKey: 'route.scheduler'
    },
    children: [
      {
        name: 'scheduler_calendar',
        path: '/scheduler/calendar',
        component: 'view.scheduler_calendar',
        meta: {
          title: 'scheduler_calendar',
          i18nKey: 'route.scheduler_calendar'
        }
      }
    ]
  }
]
