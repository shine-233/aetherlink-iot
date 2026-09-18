-- clustered_rate_limit.lua
-- 多窗口原子集群限流评测脚本（对标 ThingsBoard 复合限流：例如 "100:1,1000:60"）
--
-- KEYS: 包含 N 个窗口的 Redis 计数键集合 [key1, key2, ...]
-- ARGV: 成对传入限制值与窗口毫秒数 [limit1, windowMs1, limit2, windowMs2, ...]
--
-- 返回值列表: {allowed (1 or 0), retry_after_seconds, violated_limit, violated_window_ms}
-- - allowed = 1: 所有窗口均未超限，原子递增并放行
-- - allowed = 0: 至少一个窗口超限，不递增任何计数，拦截并返回建议重试等待秒数

local numKeys = #KEYS
local maxRetryAfterMs = 0
local violated = false
local violatedLimit = 0
local violatedWindowMs = 0

-- 阶段 1：只读预检所有窗口是否已达限额
for i = 1, numKeys do
    local key = KEYS[i]
    local limit = tonumber(ARGV[(i - 1) * 2 + 1])
    local windowMs = tonumber(ARGV[(i - 1) * 2 + 2])

    local current = tonumber(redis.call('GET', key) or "0")
    if current >= limit then
        violated = true
        local ttl = redis.call('PTTL', key)
        if ttl < 0 then
            ttl = windowMs
        end
        if ttl > maxRetryAfterMs then
            maxRetryAfterMs = ttl
            violatedLimit = limit
            violatedWindowMs = windowMs
        end
    end
end

if violated then
    local retryAfterSec = math.ceil(maxRetryAfterMs / 1000)
    if retryAfterSec <= 0 then
        retryAfterSec = 1
    end
    return {0, retryAfterSec, violatedLimit, violatedWindowMs}
end

-- 阶段 2：所有窗口均满足条件，原子递增各窗口计数并维护 TTL
for i = 1, numKeys do
    local key = KEYS[i]
    local windowMs = tonumber(ARGV[(i - 1) * 2 + 2])

    local count = redis.call('INCR', key)
    if count == 1 then
        redis.call('PEXPIRE', key, windowMs)
    end
end

return {1, 0, 0, 0}
