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
export * from './device'
export * from './rdi'
export * from './plugin'
export * from './apikey'
export * from './dashboard-menu'
export * from './board'
export * from './telemetry-dead-letter'
export * from './rule_chain'
