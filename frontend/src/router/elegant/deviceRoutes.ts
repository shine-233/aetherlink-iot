import type { GeneratedRoute } from '@elegant-router/types';

export const deviceRoutes: GeneratedRoute[] = [
  {
      name: 'device',
      path: '/device',
      component: 'layout.base',
      meta: {
        title: 'device',
        i18nKey: 'route.device'
      },
      children: [
        {
          name: 'device_command-center',
          path: '/device/command-center',
          component: 'view.device_command-center',
          meta: {
            title: 'device_command-center',
            i18nKey: 'route.device_command-center'
          }
        },
        {
          name: 'device_config',
          path: '/device/template',
          component: 'view.device_config',
          meta: {
            title: 'device_config',
            i18nKey: 'route.device_config',
            keepAlive: true
          }
        },
        {
          name: 'device_config-detail',
          path: '/device/config-detail',
          component: 'view.device_config-detail',
          meta: {
            title: 'device_config-detail',
            i18nKey: 'route.device_config-detail'
          }
        },
        {
          name: 'device_config-edit',
          path: '/device/config-edit',
          component: 'view.device_config-edit',
          meta: {
            title: 'device_config-edit',
            i18nKey: 'route.device_config-edit'
          }
        },
        {
          name: 'device_details',
          path: '/device/details',
          component: 'view.device_details',
          meta: {
            title: 'device_details',
            i18nKey: 'route.device_details'
          }
        },
        {
          name: 'device_details-child',
          path: '/device/details-child',
          component: 'view.device_details-child',
          meta: {
            title: 'device_details-child',
            i18nKey: 'route.device_details-child'
          }
        },
        {
          name: 'device_grouping',
          path: '/device/grouping',
          component: 'view.device_grouping',
          meta: {
            title: 'device_grouping',
            i18nKey: 'route.device_grouping'
          }
        },
        {
          name: 'device_grouping-details',
          path: '/device/grouping-details',
          component: 'view.device_grouping-details',
          meta: {
            title: 'device_grouping-details',
            i18nKey: 'route.device_grouping-details'
          }
        },
        {
          name: 'device_manage',
          path: '/device/manage',
          component: 'view.device_manage',
          meta: {
            title: 'device_manage',
            i18nKey: 'route.device_manage'
          }
        },
        {
          name: 'device_service-access',
          path: '/device/service-access',
          component: 'view.device_service-access',
          meta: {
            title: 'device_service-access',
            i18nKey: 'route.device_service-access'
          }
        },
        {
          name: 'device_service-details',
          path: '/device/service-details',
          component: 'view.device_service-details',
          meta: {
            title: 'device_service-details',
            i18nKey: 'route.device_service-details'
          }
        },
        {
          name: 'device_share',
          path: '/device/share',
          component: 'view.device_share',
          meta: {
            title: 'device_share',
            i18nKey: 'route.device_share'
          }
        },
        {
          name: 'device_shared-with-me',
          path: '/device/shared-with-me',
          component: 'view.device_shared-with-me',
          meta: {
            title: 'device_shared-with-me',
            i18nKey: 'route.device_shared-with-me'
          }
        },
        {
          name: 'device_template',
          path: '/device/thingsmodel',
          component: 'view.device_template',
          meta: {
            title: 'device_template',
            i18nKey: 'route.device_template'
          }
        }
      ]
    }
];

export const deviceAppRoutes: GeneratedRoute[] = [
  {
      name: 'device-details-app',
      path: '/device-details-app',
      component: 'layout.base$view.device-details-app',
      meta: {
        title: 'device-details-app',
        i18nKey: 'route.device-details-app'
      }
    }
];

export const productRoutes: GeneratedRoute[] = [
  {
      name: 'product',
      path: '/product',
      component: 'layout.base',
      meta: {
        title: 'product',
        i18nKey: 'route.product',
        icon: 'carbon:package',
        order: 4
      },
      children: [
        {
          name: 'product_update-ota',
          path: '/product/update-ota',
          component: 'view.product_update-ota',
          meta: {
            title: 'product_update-ota',
            i18nKey: 'route.product_update-ota'
          }
        },
        {
          name: 'product_update-package',
          path: '/product/update-package',
          component: 'view.product_update-package',
          meta: {
            title: 'product_update-package',
            i18nKey: 'route.product_update-package'
          }
        }
      ]
    }
];
