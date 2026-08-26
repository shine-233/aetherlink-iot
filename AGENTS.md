# AetherLink IoT 项目规则

## 定位

AetherLink IoT 是面向物联网设备接入、监控和私有部署的平台源码仓库。

## 启动与验证

- 单机部署：先运行根目录 `start-aetherlink.*` 的 doctor，再启动 Docker Compose；详见 `START-HERE.md` 与 `deploy/README.md`。
- 前端：`cd frontend && pnpm typecheck && pnpm test:run && pnpm build`。
- 后端：`cd backend && go test ./... && go build ./...`。
- Broker：`cd mqtt-broker && go test ./... && go build ./...`。
- 自动化：`cd automation_tests && npm run test:list`；真实 API/E2E 前先执行对应 preflight。

## 技术栈与目录

- `frontend/`：Vue 3、TypeScript、Vite、Vitest。
- `backend/`：Go、Gin、GORM、PostgreSQL。
- `mqtt-broker/`：Go、GMQTT。
- `automation_tests/`：Mocha、Playwright 和部署/覆盖契约。
- `references/` 保存现役基线与文档地图；`verification/` 仅保存验证证据；能力规划入口是根目录 `ROADMAP.md`。

## 现役边界

- 默认核心栈仅依赖 PostgreSQL、Redis、仓库内 MQTT Broker、Backend 和 Frontend。
- 本地 native board 是默认可视化 provider；ThingsVis 与 HTTP adapter 仅通过显式 optional profile/配置启用。
- Market、SMTP、地图 provider 属于外部可选能力；未配置时不得阻断核心启动，也不得泄露配置值。
- 外接模块必须保留稳定接口契约；能本地化的核心能力优先使用本地实现，不能本地化的能力返回明确的 optional/external-blocked 状态。
- 数据库迁移当前最高为 `53.sql` / `VERSION_NUMBER=53`；修改迁移前先核对 `backend/sql/` 与目标数据库的 `sys_version`。
- Broker 认证失败限速为插件配置面：`auth_ratelimit.max_failures_per_minute`（默认 30/分钟/IP）。
- 登录防爆破为账号+IP 双维度：账号沿用 `classified-protect.login-max-fail-times`；IP 维度用 `classified-protect.ip-login-max-fail-times` / `ip-login-fail-window-seconds`（默认 20 次/600 秒，负值关闭）。
- Casbin 路由覆盖审计默认 fail-fast（`casbin.route-audit-mode: fail-fast|warn|off`）：挂载在 CasbinRBAC 之后的新路由必须登记进资源表，否则后端拒绝启动。
- per-tenant API 限流挂载于 JWTAuth 之后全量业务路由：`api-rate-limit.requests-per-minute`（默认 600，<=0 关闭，env `GOTP_API_RATE_LIMIT_RPM`），超限返回 429+Retry-After；集群部署需替换为共享存储计数。
- devices.voucher 的 Redis 缓存键是跨服务 SHA-256 契约：`backend/pkg/utils/vouchercache.go` 必须与 `mqtt-broker/plugin/aetherlink/db.go` 的 `voucherCacheKey` 保持一致，任一侧变更需双端同步并更新两侧契约测试。
- 凭证缓存按设备失效通道是跨服务契约：backend `service.DeviceVoucherCacheInvalidationChannel` 与 broker `VoucherCacheInvalidationChannel` 均为 `aetherlink:device-voucher:cache-invalidate`，payload 为 `{"version":1,"device_id"}`；broker 写入 voucher 映射时同步维护反向索引（`aetherlink:voucher-cache-idx:v1:<deviceID>`）。任一侧变更需双端同步并更新两侧契约测试。
- 后端内部拨号 MQTT broker 一律经 `backend/pkg/utils/mqtt_broker_address.go` 的统一助手解析，禁止在业务代码里直连 localhost/127.0.0.1。
- 8082 指标端口无认证且永久 loopback 绑定（docker-compose.yml 契约测试锁定），不得改为跟随对外绑定地址。
- `deploy/gen-mqtt-certs.*` 仅生成开发/内网自签名证书；生产 TLS 必须使用正规 CA。

## 修改约定

- 改动状态以 `git status` 实时为准；批量修改前先确认工作树基线。禁止回退、覆盖、批量格式化或顺手重构无关文件。
- 不直接删除生成文件、第三方契约、审计底表或运行残留；先核对 `GENERATED_FILES.md`，删除候选在最终汇报后另行确认。根目录 `eslint-report*.json`、`tsc-errors.txt` 等可再生产物已于 2026-08 清理出库并加入 `.gitignore`，勿再提交同类产物。
- 接口、配置或行为变化必须同步注释、目标测试和权威文档。
- 静态契约或局部测试通过不等于发布就绪；未运行的 Docker、API、E2E 或真实运行验证必须标为 pending。
