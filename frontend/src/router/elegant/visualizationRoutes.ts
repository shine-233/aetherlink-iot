import type { GeneratedRoute } from '@elegant-router/types';

export const dashboardRoutes: GeneratedRoute[] = [
  {
      name: 'dashboard',
      path: '/dashboard',
      component: 'layout.base',
      meta: {
        title: 'dashboard',
        i18nKey: 'route.dashboard'
      },
      children: [
        {
          name: 'dashboard_workspace',
          path: '/dashboard/workspace',
          component: 'view.dashboard_workspace',
          meta: {
            title: 'dashboard_workspace',
            i18nKey: 'route.dashboard_workspace'
          }
        },
        {
          name: 'dashboard_rdi-overview',
          path: '/dashboard/rdi-overview',
          component: 'view.dashboard_rdi-overview',
          meta: {
            title: 'dashboard_rdi-overview',
            i18nKey: 'route.dashboard_rdi-overview'
          }
        },
        {
          name: 'dashboard_workbench',
          path: '/dashboard/workbench',
          component: 'view.dashboard_workbench',
          meta: {
            title: 'dashboard_workbench',
            i18nKey: 'route.dashboard_workbench'
          }
        }
      ]
    }
];

export const visualizationRoutes: GeneratedRoute[] = [
  {
      name: 'visualization',
      path: '/visualization',
      component: 'layout.base',
      meta: {
        title: 'visualization',
        i18nKey: 'route.visualization'
      },
      children: [
        {
          name: 'visualization_thingsvis',
          path: '/visualization/thingsvis',
          component: 'view.visualization_thingsvis',
          meta: {
            title: 'visualization_thingsvis',
            i18nKey: 'route.visualization-thingsvis'
          }
        },
        {
          name: 'visualization_thingsvis-dashboards',
          path: '/visualization/thingsvis-dashboards',
          component: 'view.visualization_thingsvis-dashboards',
          meta: {
            title: 'visualization_thingsvis-dashboards',
            i18nKey: 'route.visualization-thingsvis-dashboards',
            hideInMenu: true
          }
        },
        {
          name: 'visualization_thingsvis-editor',
          path: '/visualization/thingsvis-editor',
          component: 'view.visualization_thingsvis-editor',
          meta: {
            title: 'visualization_thingsvis-editor',
            i18nKey: 'route.visualization-thingsvis-editor',
            hideInMenu: true
          }
        },
        {
          name: 'visualization_thingsvis-menu-dashboard',
          path: '/visualization/thingsvis-menu-dashboard',
          component: 'view.visualization_thingsvis-menu-dashboard',
          meta: {
            title: 'visualization_thingsvis-menu-dashboard',
            i18nKey: 'route.visualization-thingsvis-menu-dashboard'
          }
        },
        {
          name: 'visualization_thingsvis-preview',
          path: '/visualization/thingsvis-preview',
          component: 'view.visualization_thingsvis-preview',
          meta: {
            title: 'visualization_thingsvis-preview',
            i18nKey: 'route.visualization-thingsvis-preview',
            constant: true
          }
        },
        {
          name: 'visualization_report',
          path: '/visualization/report',
          component: 'view.visualization_report',
          meta: {
            title: 'visualization_report',
            i18nKey: 'route.visualization-report',
            roles: ['SYS_ADMIN', 'TENANT_ADMIN']
          }
        },
        {
          // ROADMAP P2.2：anomaly 页面（views/visualization/anomaly）此前只存在于
          // 文件系统，路由表里没有条目，浏览器实测直接落到 not-found。
          // 这里补齐注册，与 imports.ts / transform.ts / typings 同步。
          name: 'visualization_anomaly',
          path: '/visualization/anomaly',
          component: 'view.visualization_anomaly',
          meta: {
            title: 'visualization_anomaly',
            i18nKey: 'route.visualization-anomaly',
            roles: ['SYS_ADMIN', 'TENANT_ADMIN']
          }
        },
        {
          name: 'visualization_native-boards',
          path: '/visualization/native-boards',
          component: 'view.visualization_native-boards',
          meta: {
            title: 'visualization_native-boards',
            i18nKey: 'route.visualization-native-boards',
            roles: ['SYS_ADMIN', 'TENANT_ADMIN']
          }
        },
        {
          name: 'visualization_native-board',
          path: '/visualization/native-board',
          component: 'view.visualization_native-board',
          meta: {
            title: 'visualization_native-board',
            i18nKey: 'route.visualization-native-board',
            hideInMenu: true
          }
        },
        {
          name: 'visualization_native-board-editor',
          path: '/visualization/native-board-editor',
          component: 'view.visualization_native-board-editor',
          meta: {
            title: 'visualization_native-board-editor',
            i18nKey: 'route.visualization-native-board-editor',
            hideInMenu: true,
            roles: ['SYS_ADMIN', 'TENANT_ADMIN']
          }
        },
        {
          name: 'visualization_scada-editor',
          path: '/visualization/scada-editor',
          component: 'view.visualization_scada-editor',
          meta: {
            title: 'visualization_scada-editor',
            i18nKey: 'route.visualization-scada-editor',
            roles: ['SYS_ADMIN', 'TENANT_ADMIN']
          }
        },
        {
          // 工业画布编辑器（符号库 + 拖拽）。与 scada-editor 是互补的两条路径：
          // scada-editor 负责遥测链路状态与控制命令确认；本页负责工业符号与画布拖拽。
          name: 'visualization_scada',
          path: '/visualization/scada',
          component: 'view.visualization_scada',
          meta: {
            title: 'visualization_scada',
            i18nKey: 'route.visualization-scada',
            roles: ['SYS_ADMIN', 'TENANT_ADMIN']
          }
        }
      ]
    }
];
