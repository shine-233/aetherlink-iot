import type { GeneratedRoute } from '@elegant-router/types';

export const systemIntroRoutes: GeneratedRoute[] = [
  {
      name: '403',
      path: '/403',
      component: 'layout.blank$view.403',
      meta: {
        title: '403',
        i18nKey: 'route.403',
        constant: true
      }
    },
  {
      name: '404',
      path: '/404',
      component: 'layout.blank$view.404',
      meta: {
        title: '404',
        i18nKey: 'route.404',
        constant: true
      }
    },
  {
      name: '500',
      path: '/500',
      component: 'layout.blank$view.500',
      meta: {
        title: '500',
        i18nKey: 'route.500',
        constant: true
      }
    }
];

export const applicationRoutes: GeneratedRoute[] = [
  {
      name: 'apply',
      path: '/apply',
      component: 'layout.base',
      meta: {
        title: 'apply',
        i18nKey: 'route.apply'
      },
      children: [
        {
          name: 'apply_plugin',
          path: '/apply/plugin',
          component: 'view.apply_plugin',
          meta: {
            title: 'apply_plugin',
            i18nKey: 'route.apply_plugin'
          }
        },
        {
          name: 'apply_service',
          path: '/apply/service',
          component: 'view.apply_service',
          meta: {
            title: 'apply_service',
            i18nKey: 'route.apply_service'
          }
        }
      ]
    }
];

export const authRoutes: GeneratedRoute[] = [
  {
      name: 'home',
      path: '/home',
      component: 'layout.base$view.home',
      meta: {
        title: 'home',
        i18nKey: 'route.home',
        icon: 'mdi:monitor-dashboard',
        order: 1
      }
    },
  {
      name: 'login',
      path: '/login/:module(pwd-login|code-login|register|register-email|register-super-admin|reset-pwd|bind-wechat)?',
      component: 'layout.blank$view.login',
      props: true,
      meta: {
        title: 'login',
        i18nKey: 'route.login',
        constant: true
      }
    }
];

export const adminRoutes: GeneratedRoute[] = [
];

export const managementRoutes: GeneratedRoute[] = [
  {
      name: 'management',
      path: '/management',
      component: 'layout.base',
      meta: {
        title: 'management',
        i18nKey: 'route.management'
      },
      children: [
        {
          name: 'management_api',
          path: '/management/api',
          component: 'view.management_api',
          meta: {
            title: 'management_api',
            i18nKey: 'route.management_api'
          }
        },
        {
          name: 'management_auth',
          path: '/management/auth',
          component: 'view.management_auth',
          meta: {
            title: 'management_auth',
            i18nKey: 'route.management_auth'
          }
        },
        {
          name: 'management_entity-version',
          path: '/management/entity-version',
          component: 'view.management_entity-version',
          meta: {
            title: 'management_entity-version',
            i18nKey: 'route.management_entity-version'
          }
        },
        {
          name: 'management_notification',
          path: '/management/notification',
          component: 'view.management_notification',
          meta: {
            title: 'management_notification',
            i18nKey: 'route.management_notification'
          }
        },
        {
          name: 'management_role',
          path: '/management/role',
          component: 'view.management_role',
          meta: {
            title: 'management_role',
            i18nKey: 'route.management_role'
          }
        },
        {
          name: 'management_setting',
          path: '/management/setting',
          component: 'view.management_setting',
          meta: {
            title: 'management_setting',
            i18nKey: 'route.management_setting'
          }
        },
        {
          name: 'management_user',
          path: '/management/user',
          component: 'view.management_user',
          meta: {
            title: 'management_user',
            i18nKey: 'route.management_user'
          }
        }
      ]
    }
];

export const personalRoutes: GeneratedRoute[] = [
  {
      name: 'personal-center',
      path: '/personal-center',
      component: 'layout.base$view.personal-center',
      meta: {
        title: 'personal-center',
        i18nKey: 'route.personal-center'
      }
    }
];

export const systemManagementRoutes: GeneratedRoute[] = [
  {
      name: 'system-management-user',
      path: '/system-management-user',
      component: 'layout.base',
      meta: {
        title: 'system-management-user',
        i18nKey: 'route.system-management-user'
      },
      children: [
        {
          name: 'system-management-user_equipment-map',
          path: '/system-management-user/equipment-map',
          component: 'view.system-management-user_equipment-map',
          meta: {
            title: 'system-management-user_equipment-map',
            i18nKey: 'route.system-management-user_equipment-map'
          }
        },
        {
          name: 'system-management-user_system-log',
          path: '/system-management-user/system-log',
          component: 'view.system-management-user_system-log',
          meta: {
            title: 'system-management-user_system-log',
            i18nKey: 'route.system-management-user_system-log'
          }
        }
      ]
    }
];
