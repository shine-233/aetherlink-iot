/**
 * 文件用途: request 子模块统一导出入口。
 * 核心逻辑: 对外暴露共享 HTTP 客户端和相关类型，供 service/api 与运行时数据源复用。
 * 关键注意事项: 导出路径是跨目录依赖点，调整时需检查 service、data-architecture 和测试 mock。
 * 重构建议: 保留 barrel export，新增 request helper 时优先导出类型明确的小函数。
 */
export * from './request'
