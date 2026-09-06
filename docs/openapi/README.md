# OpenAPI 契约(PHASE-D-D8a)

## 生成

```
cd backend
go run ./cmd/openapigen -out docs/openapi/openapi.json
```

- 路由枚举复用 `router.RouterInit()`(路由注册不触库,无 DB 依赖);
- 输出稳定可 diff:路径字典序、方法固定次序、键自动字典序、无时间戳;
- 动态路径参数 `:name` → OpenAPI path 参数;tag = `api/v1` 后首个模块段;
- 鉴权:`/api/v1/*` 声明 Bearer JWT 或 `X-API-Key`(OpenAPI Key)双方案;
- 当前覆盖:330 paths(生成时统计);请求/响应 schema 为 MVP 推断骨架(泛化 object),
  精细化 schema 随 handler swagger 注解逐域补充。

## 已知缺口

- 请求体 schema 未逐端点建模(骨架为空 object);
- 非受保护路径(health/metrics 等)不携带 security,为预期行为;
- `HEAD/OPTIONS` 不入契约。
