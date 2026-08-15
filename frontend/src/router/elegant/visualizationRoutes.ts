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
        }
      ]
    }
];
