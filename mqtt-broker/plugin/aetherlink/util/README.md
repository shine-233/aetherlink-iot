# AetherLink Plugin Utilities

`mqtt-broker/plugin/aetherlink/util` contains topic validation helpers used by the AetherLink broker plugin.

## Directory Responsibilities

- Validate device publish topics before the AetherLink plugin accepts upstream data.
- Validate device subscribe topics before the plugin permits downstream subscriptions.
- Keep the allowed topic contract small, deterministic, and easy to review against product MQTT docs.

## File Relationships

- `check_pub_topic.go` owns the publish allowlist and `ValidateTopic`.
- `check_sub_topic.go` owns the subscribe allowlist and `ValidateSubTopic`.
- `check_pub_topic_test.go` and `check_sub_topic_test.go` pin accepted and rejected examples for authorization-sensitive topic rules.
- Both validators currently implement their own pattern matching logic; behavior should stay aligned when wildcard policy changes.

## Key Files

- `check_pub_topic.go`: Publish topic patterns, MQTT `+` wildcard handling, and empty-segment rejection.
- `check_sub_topic.go`: Subscribe topic patterns, `{device_number}` placeholder handling, and wildcard/device-ID safeguards.
- `*_test.go`: Focused regression tests for valid topics, malformed topics, and empty topic levels.

## Review Suggestions

- Compare every allowlist addition with the external AetherLink MQTT topic contract.
- Confirm wildcard semantics do not permit empty device IDs, `#`, or over-broad subscriptions.
- Add table-driven tests before changing topic patterns.
- Consider refactoring publish and subscribe matching into a shared matcher once the topic contract stabilizes.
