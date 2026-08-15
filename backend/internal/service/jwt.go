// 文件用途：维护 JWT token 在缓存中的读取和鉴权辅助服务。
// 核心逻辑：根据用户或 token 信息构造 Redis key，读取登录态并返回鉴权所需数据。
// 关键注意事项：token 缺失、过期或 Redis 异常应 fail-closed，不能把缓存错误误判为有效登录。
// 重构建议：抽出 token 存储接口，补齐 Redis 错误、过期、重复登录覆盖和审计边界测试。
package service

type JwtService struct{}

// 从redis中获取jwt的key
