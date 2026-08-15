/**
 * 文件用途：定义 axios 请求封装内部共享常量。
 * 核心逻辑：集中维护请求 ID 请求头和后端错误码标识。
 * 关键注意事项：常量值会被请求实例和错误处理逻辑共同引用，修改会影响跨模块约定。
 * 重构建议：如常量继续增加，可按请求头、错误码和追踪字段分组导出。
 */
/** requestTs id key */
export const REQUEST_ID_KEY = 'X-Request-Id'

/** the backend error code key */
export const BACKEND_ERROR_CODE = 'BACKEND_ERROR'
