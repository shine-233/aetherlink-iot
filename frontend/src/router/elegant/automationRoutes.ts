import type { GeneratedRoute } from '@elegant-router/types';

export const alarmRoutes: GeneratedRoute[] = [
  {
      name: 'alarm',
      path: '/alarm',
      component: 'layout.base',
      meta: {
        title: 'alarm',
        i18nKey: 'route.alarm'
      },
      children: [
        {
          name: 'alarm_notification-group',
          path: '/alarm/notification-group',
          component: 'view.alarm_notification-group',
          meta: {
            title: 'alarm_notification-group',
            i18nKey: 'route.alarm_notification-group'
          }
        },
        {
          name: 'alarm_notification-record',
          path: '/alarm/notification-record',
          component: 'view.alarm_notification-record',
          meta: {
            title: 'alarm_notification-record',
            i18nKey: 'route.alarm_notification-record'
          }
        },
        {
          name: 'alarm_rdi-overview',
          path: '/alarm/rdi-overview',
          component: 'view.alarm_rdi-overview',
          meta: {
            title: 'alarm_rdi-overview',
            i18nKey: 'route.alarm_rdi-overview'
          }
        },
        {
          name: 'alarm_warning-message',
          path: '/alarm/warning-message',
          component: 'view.alarm_warning-message',
          meta: {
            title: 'alarm_warning-message',
            i18nKey: 'route.alarm_warning-message'
          }
        }
      ]
    }
];

export const automationRoutes: GeneratedRoute[] = [
  {
      name: 'automation',
      path: '/automation',
      component: 'layout.base',
      meta: {
        title: 'automation',
        i18nKey: 'route.automation'
      },
      children: [
        {
          name: 'automation_linkage-edit',
          path: '/automation/linkage-edit',
          component: 'view.automation_linkage-edit',
          meta: {
            title: 'automation_linkage-edit',
            i18nKey: 'route.automation_linkage-edit'
          }
        },
        {
          name: 'automation_scene-edit',
          path: '/automation/scene-edit',
          component: 'view.automation_scene-edit',
          meta: {
            title: 'automation_scene-edit',
            i18nKey: 'route.automation_scene-edit'
          }
        },
        {
          name: 'automation_scene-linkage',
          path: '/automation/scene-linkage',
          component: 'view.automation_scene-linkage',
          meta: {
            title: 'automation_scene-linkage',
            i18nKey: 'route.automation_scene-linkage'
          }
        },
        {
          name: 'automation_scene-manage',
          path: '/automation/scene-manage',
          component: 'view.automation_scene-manage',
          meta: {
            title: 'automation_scene-manage',
            i18nKey: 'route.automation_scene-manage'
          }
        }
      ]
    }
];
