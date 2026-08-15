/**
 * 文件用途：定义业务域通用常量。
 * 核心逻辑：集中维护设备、产品、告警或业务枚举的共享取值。
 * 关键注意事项：常量值可能与后端字典或接口契约绑定，不能仅按文案修改。
 * 重构建议：可逐步用后端字典生成类型化常量，减少前后端漂移。
 */
/**
 * 文件：业务枚举常量。
 * 作用：维护管理后台、服务管理、规则引擎、数据服务等业务选项。
 * 依赖：依赖国际化函数和通用 option 转换工具生成 UI 下拉选项。
 * 维护：接口枚举或国际化 key 变化时同步更新映射和调用方测试。
 */

import { transformObjectToOption, transformRecordToOption } from '@/utils/common/options'
import { $t } from '@/locales'

export const enableStatusRecord: Record<Api.Common.EnableStatus, App.I18n.I18nKey> = {
  '1': 'page.manage.common.status.enable',
  '2': 'page.manage.common.status.disable'
}

export const enableStatusOptions = transformRecordToOption(enableStatusRecord)

export const userGenderRecord: Record<Api.SystemManage.UserGender, App.I18n.I18nKey> = {
  '1': 'page.manage.user.gender.male',
  '2': 'page.manage.user.gender.female'
}

export const userGenderOptions = transformRecordToOption(userGenderRecord)

export const menuTypeRecord: Record<Api.SystemManage.MenuType, App.I18n.I18nKey> = {
  '1': 'page.manage.menu.type.directory',
  '2': 'page.manage.menu.type.menu'
}

export const menuTypeOptions = transformRecordToOption<OptionTypes>(menuTypeRecord as any)

export const menuIconTypeRecord: Record<Api.SystemManage.IconType, App.I18n.I18nKey> = {
  '1': 'page.manage.menu.iconType.iconify',
  '2': 'page.manage.menu.iconType.local'
}

export const menuIconTypeOptions = transformRecordToOption(menuIconTypeRecord)

export const userRoleLabels: Record<Api.Auth.RoleType, string> = {
  SYS_ADMIN: $t('page.login.pwdLogin.superAdmin'),
  TENANT_ADMIN: $t('page.login.pwdLogin.admin'),
  TENANT_USER: $t('page.login.pwdLogin.user')
}

export const userRoleOptions = transformRecordToOption(userRoleLabels)

/** 路由管理 - 组件类型 */
export const routeComponentTypeLabels: Record<'basic' | 'blank' | 'multi' | 'self' | 'base' | 'custom', string> = {
  basic: 'basic',
  blank: 'blank',
  multi: 'multi',
  self: 'self',
  base: 'base',
  custom: ''
}

export const routeComponentTypeOptions = transformObjectToOption(routeComponentTypeLabels)

/** 路由管理 - 路由类型 */
export const routerTypeLabels: Record<CustomRoute.routerTypeKey, string> = {
  1: $t('card.menu'),
  3: $t('card.route')
}
export const routeTypeOptions = transformObjectToOption(routerTypeLabels)
/** 路由管理 - 访问标识 */
export const routerSysFlagLabels: Record<CustomRoute.routerSysFlagKey, string> = {
  SYS_ADMIN: $t('card.systemAdmin'),
  TENANT_ADMIN: $t('card.tenantAdmin')
}

export const routeSysFlagOptions = transformObjectToOption(routerSysFlagLabels)

/** 应用管理 - 服务管理 - 服务管理 - 设备类型 */
export const serviceManagementDeviceTypeLabels: Record<ServiceManagement.DeviceTypeKey, string> = {
  1: $t('generate.direct-connected-device'),
  2: $t('generate.gatewayDevice')
}

export const serviceManagementDeviceTypeOptions = transformObjectToOption(serviceManagementDeviceTypeLabels)

/** 应用管理 - 服务管理 - 服务管理 - 协议类型 */
export const serviceManagementProtocolTypeLabels: Record<ServiceManagement.ProtocolTypeKey, string> = {
  1: 'MODBUS_RTU'
}
export const serviceManagementProtocolTypeOptions = transformObjectToOption(serviceManagementProtocolTypeLabels)

/** 规则引擎状态状态 */
export const ruleEngineStatusLabels: Record<RuleEngine.StatusKey, string> = {
  1: $t('card.started'),
  2: $t('card.paused')
}
export const ruleEngineStatusOptions = transformObjectToOption(ruleEngineStatusLabels)

/** 数据服务-签名方式 */
export const dataServiceSignModeLabels: Record<DataService.SignModeKey, string> = {
  1: 'MD5',
  2: 'HAS256'
}

export const dataServiceSignModeOptions = transformObjectToOption(dataServiceSignModeLabels)

/** 数据服务-接口支持标志 */
export const dataServiceFlagLabels: Record<DataService.FlagKey, string> = {
  1: $t('card.httpInterface'),
  2: $t('card.httpwsInterface')
}
export const dataServiceFlagOptions = transformObjectToOption(dataServiceFlagLabels)

/** 数据服务-状态 */
export const dataServiceStatusLabels: Record<DataService.StatusKey, string> = {
  1: $t('card.started'),
  2: $t('card.stopped')
}
export const dataServiceStatusOptions = transformObjectToOption(dataServiceStatusLabels)

/** 用户状态 */
export const userStatusLabels: Record<UserManagement.UserStatusKey, string> = {
  F: 'freeze',
  N: 'normal'
}
export const userStatusOptions = transformObjectToOption(userStatusLabels)

/** 系统管理 - 常规设置 - 数据清理 清理类型 */
export const dataClearSettingEnabledTypeLabels: Record<GeneralSetting.EnabledTypeKey, string> = {
  1: $t('page.manage.common.status.enable'),
  2: $t('page.manage.common.status.disable')
}
export const dataClearSettingEnabledTypeOptions = transformObjectToOption(dataClearSettingEnabledTypeLabels)

/** 系统管理 - 常规设置 - 数据清理 清理内容 */
export const dataClearSettingCleanupTypeLabels: Record<GeneralSetting.CleanupTypeKey, string> = {
  1: $t('card.deviceData'),
  2: $t('card.operationLog')
}

export const signModeOptions = [
  {
    label: 'MD5',
    value: 'MD5'
  },
  {
    label: 'HAS256',
    value: 'HAS256'
  }
]

export const packageOptions = [
  { label: $t('page.product.update-package.diff'), value: 1 },
  { label: $t('page.product.update-package.full'), value: 2 }
]

export const memberNotificationLabels: Record<CustomRoute.routerSysFlagKey, string> = {
  EMAIL: $t('card.emailNotice'),
  APP: $t('card.appNotice'),
  WECHAT: $t('card.wechatNotice')
}

export const MemberNotificationOptions = transformObjectToOption(memberNotificationLabels)

export const notificationOptions = [
  {
    label: $t('card.memberNotice'),
    value: 'MEMBER'
  },
  {
    label: $t('card.emailNotice'),
    value: 'EMAIL'
  },
  {
    label: 'webhook',
    value: 'WEBHOOK'
  }
]

export const enumDataType: Record<'Number' | 'String' | 'Boolean', string> = {
  Number: 'Number',
  String: 'String',
  Boolean: 'Boolean'
}

export const enumDataTypeOption = transformObjectToOption(enumDataType)
