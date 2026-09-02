# P1/P2 加固批次 · 交接状态（2026-08-26）

分支：`improve/p1p2-hardening`（基于 improve/p2p3-batch @ 59bccd9，已推送）
批次提交：`2c3ddab` 后端安全加固第一批（24 文件，+489/-54）

## 已完成（本会话验证过）

- [x] LIKE 通配转义：`dal.EscapeLikePattern/ContainsLikePattern`（like_escape.go）+ 全部 15 处拼接点接入 + 单测
- [x] 刷新吊销旧会话（F1）：`RefreshToken(claims, previousTokenDigest)`；api 层经 `middleware.SelectJWTAuthToken`（新导出）算摘要
- [x] `<email>_token` 键改存 TokenDigest 且 TTL 对齐会话超时（F2）；`deleteLoginToken` 已删除
- [x] IP 维度登录防爆破（F3）：`login_lock.go` 新增 4 方法；配置键 `classified-protect.ip-login-max-fail-times/ip-login-fail-window-seconds`（默认 20/600s 启用，-1 关闭）；conf.yml 与 conf.example.yml 已同步
- [x] Casbin 路由覆盖审计：`router/casbin_audit.go` 基线快照差集 + fail-fast；模式 `casbin.route-audit-mode: fail-fast|warn|off`（yml 待补键，见下）
- [x] 测试：like_escape_test / casbin_route_audit_test(service) / casbin_audit_test(router) / login_lock IP 用例

## 验证状态

- `go build ./...` 通过；`go test ./router/ ./internal/api/ ./internal/dal/` 全绿
- `internal/service` 仅 1 个**既有失败**（与本批无关，基线复现）：`TestCheckDBMigrationsRequiresCurrentMigrationVersion`
- CI：push 已触发 Source CI，结果未等待

## 下一步（新会话按序执行）

1. **补 yml**：conf.yml/conf.example.yml 顶层加 `casbin: { route-audit-mode: fail-fast }`（代码默认即 fail-fast，写键是为了可发现性）
2. **跑全量**：`cd backend && go test ./... && go build ./...`；前端不动
3. **A5 http_client**：`third_party/others/http_client/request_method.go` 的 `Post/Delete` 是零调用死代码→直接删；`PostJson` 保留（Notification 正确 Close、DisconnectDevice 调用方 runtime.go:111 正确 defer Close）；更新文件头注释第 3 行
4. **A6 裸查 DAL 加固**：`GetDeviceByID`(device_query_reads.go:291)/`GetDevicesByIDs`(:316) 改名 `*Unscoped`+doc 强制约定，~26 个调用点在 internal/service（编译器驱动）；`DeleteDeviceConfig(id)`(device_config.go:86)/`DeleteDeviceGroup(id)`(device_groups.go:87) 增加 `ForTenant(id,tenantID)` 版本并改 2 个 service 调用点传 claims.TenantID
5. **前端 B 批**：B1 three/@tresjs/core/@tresjs/cientos/@types/three 从 devDependencies 移到 dependencies（pnpm install 刷 lockfile）；B2 vite.config.ts manualChunks 加 vendor-three(three,@tresjs/*)+vendor-motion(motion-v)；B3 ~~china-region.json~~ **已是动态导入（index.vue:109、ProvinceCityDistrictSelector.vue:88），无需做**；封面图 554KB 压缩可选
6. **前端 B5**：apply/form.vue、personal-center/change-information.vue、management/user table-action-modal 补 loading/error/empty 态 + 组件测试
7. **部署 C1**：doctor.sh/.ps1 在 server 模式且绑定非 loopback 且未配 TLS 证书时强警告（`AETHERLINK_STRICT_TLS=1` 升级为阻断）
8. **文档同步**：COMPATIBILITY.md（casbin 审计对存量库的升级影响）、VALIDATION.md（Redis 行为变更标 pending）、AGENTS.md casbin 审计一行
9. **明确不做（记录理由）**：13 个零测试目录全量补齐/巨型文件拆分/1014 处硬编码 hex 迁移/JWT 默认改 HttpOnly cookie/Device3DPanel 接线——需独立 lane 或产品决策

## 环境备忘

- worktree：`C:\Users\Zz\AppData\Local\Temp\opencode\final-wt`（占用 improve/p2p3-batch 分支的是它，主仓库在 sec/p0-guardrails）
- 禁止用 PS 的 Get-Content/Set-Content 往返改文件（UTF8+BOM 毁编码，已踩坑），一律用 edit 工具
