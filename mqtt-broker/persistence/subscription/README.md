# Subscription Persistence

`mqtt-broker/persistence/subscription` contains MQTT subscription persistence and lookup.

## Folder Role

- `mem/` implements in-memory subscription stores and topic tries.
- `redis/` implements Redis-backed subscription stores.
- `test/` contains shared behavior tests.

## Review Notes

Subscription matching affects message delivery and wildcard behavior. Keep trie and Redis behavior aligned through shared tests.
