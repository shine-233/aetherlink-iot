// 文件用途：集中定义跨模块共享的缓存策略常量。
// 核心逻辑：为设备信息与数据脚本缓存提供兜底过期时间，避免 TTL=0 的"永久缓存"
// 在任何写路径遗漏主动失效时造成不可恢复的脏读。
// 关键注意事项：主动失效（DelDeviceCache / InvalidateCache 等）仍是主机制，
// 本兜底 TTL 只是第二道防线；调整数值需同步 initialize/缓存说明.md 与相关测试。
// 重构建议：后续若新增其他永久缓存写入点，统一改用本常量。
package constant

import "time"

// CacheFallbackTTL 是设备/脚本等 JSON 缓存的兜底过期时间。
const CacheFallbackTTL time.Duration = 30 * time.Minute
