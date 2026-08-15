# backend/router/publicfiles

## 目录定位

`backend/router/publicfiles` 是公开文件下载路由的路径解析子包，用于把 `/files/*filepath` 的 URL 路径安全映射到后端本地 `./files` 目录。

## 文件用途

- `resolver.go`：清洗 URL 路径，拒绝空路径、反斜杠、盘符、`..` 和越界路径，最后返回 `./files` 下的绝对路径。
- `resolver_test.go`：验证正常子路径可解析，并覆盖 Windows/POSIX 风格的路径逃逸输入。

## 依赖关系

本目录只依赖 Go 标准库，由 `backend/router/router_init.go` 的 `/files/*filepath` 路由调用。它不直接读取文件内容，只负责返回安全的本地路径给 Gin `c.File`。

## 审查发现

- 路径解析属于安全敏感逻辑，不能用简单字符串拼接替代。
- 当前实现同时检查 URL 路径和本地绝对路径相对关系，能覆盖常见目录穿越输入。
- 测试覆盖了基础逃逸场景，但尚未覆盖 URL 编码后的特殊字符，是否由 Gin 解码需要在路由层确认。

## 重构建议

后续可补充 URL 编码、重复分隔符、大小写盘符和符号链接边界测试；如果支持可配置文件根目录，应把根目录作为参数注入并保留相对路径校验。

## 验证建议

修改本目录后运行 `cd backend; go test ./router/publicfiles -count=1`。如果同时改动 `/files/*filepath` 路由，还应运行 `cd backend; go test ./router -run TestRouterContract -count=1`。
