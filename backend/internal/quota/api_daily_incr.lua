-- 用途：TB-17 API 日配额同日累加脚本（internal/quota/meter.go 配套）。
-- KEYS[1] = 当日计数键（aetherlink:quota:api_daily:<date>:<tenantID>）
-- ARGV[1] = DB 种子值（该租户当日已落库的 api_calls，用于 Redis 丢失后对齐）
-- ARGV[2] = 键 TTL（毫秒，至次日 UTC 零点 + 缓冲）
-- 返回：累加后的当日计数。
-- 语义：键不存在时先写入种子值再 INCR（等价 DB 对齐 + 1）；已存在则直接 INCR。
-- 计数键与 TTL 原子维护：无 TTL 的残留键（历史版本/异常）就地补 TTL，防泄漏与跨日污染。
local cur = redis.call('GET', KEYS[1])
if not cur then
    redis.call('SET', KEYS[1], ARGV[1])
end
local count = redis.call('INCR', KEYS[1])
if redis.call('PTTL', KEYS[1]) < 0 then
    redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return count
