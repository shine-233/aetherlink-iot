# tests

## 目录职责

保存设备自动测试的外部集成测试入口，按直连设备和网关设备分目录组织。

## 文件关系

`direct` 使用 `config-community.yaml` 和直连 topic。`gateway` 使用 `config-gateway-community.yaml` 和网关嵌套 topic。两个目录都依赖 `internal/device`、`internal/platform` 和 `internal/utils`。

## 重点文件

| 目录 | 说明 |
| --- | --- |
| `direct` | 直连遥测、属性、事件、命令和控制闭环测试。 |
| `gateway` | 网关遥测、属性、事件、命令、控制和多层组合测试。 |

## 审查建议

这些测试需要 `external_integration` build tag 和 `AUTOTEST_EXTERNAL=1`。默认 `go test` 不应把跳过状态误判为业务闭环已验证。
