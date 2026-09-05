-- 用途：per-tenant API 限流固定窗口计数脚本（tenant_rate_limit.go 配套）。
-- KEYS[1] = 计数键（aetherlink:ratelimit:tenant:<subject>）
-- ARGV[1] = 窗口内允许的最大请求数（rpm）
-- ARGV[2] = 窗口长度（毫秒）
-- 返回：{当前计数, 剩余 TTL 毫秒}——剩余 TTL 供 Retry-After 计算，免去第二次往返。
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
