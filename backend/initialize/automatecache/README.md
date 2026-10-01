# automatecache

This directory holds the Redis read/write logic for the automation trigger cache. It was moved here from `initialize/automate_cache.go`, which now keeps only type aliases and `NewAutomateCache()` for compatibility.

- `cache.go` / `cache_ops.go`: the `Cache` type. It is **stateless**: the one/multiple dimension is passed per call and is never written back to a field. The old singleton wrote `c.device` on every call, which raced once concurrent callers stopped being serialized by the Automate global lock.
- `types.go`: cached structures and result constants. The JSON field names are the on-wire format of existing cache entries and must not change.
- `device_one.go` / `device_multiple.go`: dimension adapters (key prefix + trigger condition type).

The key format `automate:v3:{one|multiple}:{_|_group_|_action_}:{id}` matches existing production cache entries. `TestCacheKeyFormatIsStable` locks it.
