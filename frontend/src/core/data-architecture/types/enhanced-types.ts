/**
 * 文件用途: 增强版数据架构类型的兼容入口。
 * 核心逻辑: 原 1118 行的单一类型文件已按内聚主题拆分为 enhanced/ 下的子模块，此处仅做 barrel 重导出。
 * 关键注意事项: 该文件承接新旧配置兼容，修改版本字段或适配类型需要配套迁移测试；请勿在此新增类型定义。
 * 重构建议: 新增类型请放入 enhanced/ 对应主题模块，再通过本文件导出。
 */
export * from './enhanced/data-item'
export * from './enhanced/placeholder'
export * from './enhanced/component-mapping'
export * from './enhanced/configuration'
export * from './enhanced/converters'
export * from './enhanced/guards'
