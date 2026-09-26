/**
 * 文件用途: service/api 的统一导出入口。
 * 核心逻辑: 聚合各业务域 API wrapper，供 views、store 和 composables 使用稳定导入路径。
 * 关键注意事项: 删除或重命名导出会影响大量页面隐式依赖，尤其是通过 `@/service/api` 汇总导入的旧代码。
 * 重构建议: 迁移到分域导入时保留兼容 re-export，并用 `rg "@/service/api"` 校验调用点。
 */
// Shared API barrel. Keep this import path stable for pages, stores, and
// composables that still consume `@/service/api` as a contract surface.
export * from './auth'
export * from './route'
export * from './system-data'
export {
  deleteDeviceTemplate,
  deviceTemplate as deviceTemplateModel,
  getDeviceListForSelect,
  getDeviceModel,
  getDeviceTemplateDetail,
  postDeviceModel,
  putDeviceModel
} from './device-template-model'
export * from './device-data-source'
export * from './roles'
export * from './protocol-plugin'
export * from './notification-services'
export * from './customer'
export * from './device'
export * from './rdi'
export * from './plugin'
export * from './apikey'
export * from './dashboard-menu'
export * from './board'
export * from './telemetry-dead-letter'
export * from './report'
export * from './rule_chain'
export * from './asset'
export * from './entity-relation'
export * from './entity_version'
export * from './plugin_registry' // PHASE-D-D9 插件框架 gRPC 网关
export * from './license'
export * from './edge-node'
export * from './solution'
// ROADMAP P2.2：anomaly 页面（views/visualization/anomaly）从 `@/service/api` 汇总导入
// TELEMETRY_ANOMALY_RULE_* / detectTelemetryAnomalies 等，但本 barrel 此前漏了这条
// re-export，导致 vue-tsc 报 11 个 TS2305「has no exported member」，页面也编译不过。
export * from './telemetry-analysis'
// TB-04 部件库（widget_bundles）：CRUD + 内置四部件种子导入 API。
export * from './widget-bundle'
// TB-17 租户 API 日配额：今日调用数/限额/剩余量（billing/api-quota）。
export * from './billing'
// TB-41 文件存储与媒体库（media_files）：通用上传 + 媒体列表/详情/删除 API。
export * from './media'
// TB-45 统一集成实体（integrations）：连接器实例 CRUD + 上下行转换器绑定。
export * from './integration'
// TB-46 用户组与组权限（GPE v1）：组 CRUD + 成员管理 + 组权限元素绑定（组共享授权）。
export * from './user-group'
// TB-23 移动应用中心（mobile_app_bundles）：上传登记 + 版本列表 + publish/archive 状态机。
export * from './mobile-app-bundle'
// TB-47 白标：租户翻译覆盖（CRUD）+ 自定义 CSS（读写）+ 登录后可读覆盖获取。
export * from './whitelabel'
// TB-48 统一调度器（scheduler_events）：三源聚合列表 + 注册面 CRUD。
export * from './scheduler'
