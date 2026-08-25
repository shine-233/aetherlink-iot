# GHCR package visibility readback（2026-08-17）

## 目的

独立确认 GitHub Packages 中三个 AetherLink 容器 package 的 metadata visibility，
而不是从公开仓库状态或 workflow 配置推测。

## 读取边界

本次只使用 GitHub CLI 的 GET 请求；没有修改 package、tag、repository 设置或删除
任何版本。当前 CLI token 的 scope 已回读为：

```text
gist, read:org, read:packages, read:project, repo, workflow
```

没有输出 token 值。最小新增权限是 `read:packages`；没有申请
`write:packages` 或 `delete:packages`。

## API 结果

请求：

```text
GET /users/shine-233/packages?package_type=container&per_page=100
GET /users/shine-233/packages/container/{package}
GET /users/shine-233/packages/container/{package}/versions?per_page=100
```

| Package | package_type | visibility | repository association | version tags |
|---|---|---|---|---|
| `aetherlink-iot-backend` | `container` | `public` | `shine-233/aetherlink-iot` | `0.1.6`, `latest`, `0.1` |
| `aetherlink-iot-frontend` | `container` | `public` | `shine-233/aetherlink-iot` | `0.1.6`, `latest`, `0.1` |
| `aetherlink-iot-mqtt-broker` | `container` | `public` | `shine-233/aetherlink-iot` | `0.1.6`, `latest`, `0.1` |

GitHub API 返回的 package version IDs：

```text
aetherlink-iot-backend    = 1140278942
aetherlink-iot-frontend   = 1140277229
aetherlink-iot-mqtt-broker = 1140276199
```

## 结论和限制

因此可以确认：

```text
GitHub package metadata visibility = public（三个 package）
```

这意味着 package 的 GitHub visibility 设置已经公开；它不是由仓库
`private=false` 单独推导出来的。

本机另做了匿名 `ghcr.io/token` probe，三个 registry scope 均返回 HTTP 403。该 probe
没有被记为“匿名 docker pull 通过”，因为它还可能受当前网络、registry endpoint 或
认证策略影响。它不推翻 GitHub Packages API 对 metadata visibility 的 `public` 回读，
但也不能把它写成“本机已经完成匿名 image pull 验收”。

真正的容器运行时验收仍然需要 Docker/Compose、镜像拉取、healthcheck、数据库迁移和
关键业务请求；package visibility 公开不等于部署等价性已经证明。
