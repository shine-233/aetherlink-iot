# 发布边界策略

本仓库按独立的 AetherLink IoT 源码工作区准备公开发布。本文档定义公开源码边界和发布检查清单；详细验证门槛见 `VALIDATION.md`，外部合约说明见 `COMPATIBILITY.md`。

## 公开源码边界

预期的公开源码候选范围包括：

- 根目录公开文档和仓库元数据。
- `frontend/`。
- `backend/`。
- `mqtt-broker/`。
- `automation_tests/`。
- `references/` 中用于描述当前公开状态、准入标准或工作计划的文件，不包括私人本地历史。

以下本地材料应被忽略，不应提交：

- `verification/` 证据归档。
- `audit_reports/` 本地审计和迁移历史。
- 根目录 `PROJECT_CLEANUP_PLAN.md`、`PROJECT_FOLDER_AUDIT_*`、`PROJECT_FOLDER_CONTENTS_*` 等固定工作区台账；它们包含历史绝对路径和自引用哈希，必须原位保留但不进入公开源码。
- 本地状态文档和带日期的内部笔记。
- `node_modules/` 等依赖目录。
- build、coverage、report、lock 和运行时输出目录。
- 本地 `.env`、`*.env`、`*.local`、key、cert 和运行时配置文件。

当前已确认的高风险本地边界还包括：

- `backend/configs/rsa_key/private_key.pem`：真实私钥，禁止公开；如曾入库应视为已泄露并重新生成。
- `backend/configs/rsa_key/public.pem`：建议与私钥一起移出公开仓库边界，改为部署时注入或本地保留。
- `backend/configs/conf-localdev.yml`：本地开发接线配置，不应进入公开仓库。
- `frontend/.env`、`frontend/.env.development`、`frontend/.env.production`：环境态文件，只保留模板，不公开真实版本。
- `backend/cmd/aetherlink-device-autotest/docs/`：更适合作为本地脱敏协议资料，不应默认纳入公开源码范围。
- `frontend/src/typings/components.d.ts`：前端自动生成文件，不应作为公开源码长期保留。

## 2026-08-15 上传前清理与当前 GitHub 状态

本轮已完成源码工作区的上传边界清理，并已将公开源码推送到 [AetherLink IoT GitHub 仓库](https://github.com/shine-233/aetherlink-iot)；没有把未完成的 r14 复测写成通过：

- 根 `_localrun`、前端构建/coverage、后端和 broker 二进制、运行态配置、认证状态、日志、截图、历史 verification 归档和本地审计台账已移动到仓库外的可恢复 quarantine；清单为本机父目录下的 `_aetherlink-github-cleanup-quarantine-20260815/github-cleanup-manifest-20260815.json`。
- 前一批 stale r14 运行物也保存在 `_aetherlink-cleanup-quarantine-20260815-r14-pre/quarantine-manifest.json`；两批均为 `permanentDelete=false`，回移前必须重新确认无进程引用并按清单校验。
- `node_modules` 没有上传，也没有搬走；它们是由 lockfile 重建的本地依赖树。`_localrun_instance_b/` 只保留公开 README 和 `instance-b.env.example` 模板。
- 追加清理批次已将 `automation_tests/verification/ready-check-command-draft-20260809.md`、dated deployment readiness/audit、迁移哈希快照和 repository inventory 共 7 个本地历史/生成文件移到 `../_aetherlink-github-cleanup-quarantine-20260815-r2/`；清单为其中的 `github-cleanup-manifest-20260815-r2.json`，逐项确认源路径不存在、哈希一致，`permanentDelete=false`。空的 r15 临时验证目录也已移除。
本轮未完成的 `_aetherlink-validation-20260815-r16` 未被写入发布证据，已在未重新测试的前提下整体移到仓库外 `../_aetherlink-github-cleanup-quarantine-20260815-r3/`，并通过逐文件 SHA-256 校验；该批次仍为 `permanentDelete=false`，不代表部署或 GitHub 上传已执行。
- `.gitignore` 现在明确排除 `automation_tests/verification/`、dated readiness/audit、migration hash 和 repository inventory；`verification/templates/`、模拟器源码和测试合同仍保留。
- tracked 文件的凭据边界扫描未发现明文数据库密码、私钥标记或带凭据的数据库 URI；真实密码只允许通过当前进程环境注入，不能写入文档、日志、manifest、dump 或提交。
- 清理后生成物候选扫描为 `0`；`check_supply_chain.js` 和 `release_preflight.js` 仍是静态/合同门禁，不等于 Docker、目标服务器、公网 MQTT、HTTPS/TLS 或真实设备验收。
- r14 后端全量 Go 测试因 `proxy.golang.org` 依赖下载超时未完成，broker 测试按用户要求停止；它们不能标记为通过。已有 r13 的本地证据仍按历史批次引用，并不替代当前目标环境验收。

当前发布标记为：`source_package_boundary=public-source`、`github_upload=executed`、`source_release=published`、`ghcr_release=workflow-pending`、`real_rdi_status=not-tested`、`target_deployment_status=pending`、`production_signoff=not-ready`。ThingsVis 仍是有源码和合同引用的 optional legacy compatibility provider，不因 Native 可运行而删除；`negative-menu` 是 ownership rejection 测试场景，不是待清理服务。

## GitHub 托管功能状态

当前已启用或已接入：

- Actions、SHA pinning required、只读默认 `GITHUB_TOKEN` 权限，以及源码 CI、Minimum quality gate、CodeQL、Dependency Review 和手动/夜间 integration workflow。
- Dependabot alerts、security updates、automated security fixes，以及 GitHub Actions、frontend/automation npm、三个 Go module 和三个 Docker 目录的版本更新配置。
- Secret Scanning、Push Protection、Issues、Discussions、Projects、Wiki、Issue Forms、PR 模板和 CODEOWNERS。
- `integration` environment；当前没有真实 API、账号、设备凭据或生产 secrets，因此手动/夜间 workflow 默认不会伪造 live 验收。
- `.github/workflows/container-release.yml` 已加入；它只在正式 tag 或手动指定已有 tag 时构建三个源码镜像，并要求 SBOM、provenance 和 digest attestation。首次 GHCR 发布及包可见性仍待托管运行验证。

当前尚未完成：GHCR 的首次镜像发布及其 digest/SBOM/attestation 托管验证，以及真实 API/E2E/设备和目标部署验收。`main` 分支保护、正式 Git tag/Source Release 已完成并已验证。Secret Scanning 的 non-provider patterns 与 validity checks 仍以 GitHub 当前设置为准，不能在文档中写成已启用。

## 兼容名称

公开品牌和默认入口已对齐到 AetherLink IoT。`COMPATIBILITY.md` 现在只用于说明未来 broker、ThingsVis 或 telemetry 合约变化应如何处理。

- broker plugin loading/config surfaces。
- ThingsVis embed/SSO identifiers and host keys。
- external telemetry gRPC service symbols。

简单规则如下：

- 不要把已退役的产品名重新引入当前文档、测试或配置。
- 未来的 wire/config/service 重命名应视为 breaking change。
- 任何未来合约重命名都应配套聚焦的 MQTT/ThingsVis/gRPC/API/E2E 验证，并记录明确的迁移说明。

首次提交前应重新运行最新扫描。不要把旧的公开候选计数复制到公开文档中作为当前结论；如需保留快照，应放在本地状态说明中。

## 验证门槛

在基于当前工作树重新生成验证证据之前，不要声明 release readiness。具体命令顺序由 `VALIDATION.md` 维护；本文档只保留发布门槛摘要。

当前发布门槛如下：

在 `automation_tests/` 中运行 `npm run preflight:release` 可先执行离线静态/契约门禁（供应链、生成物边界和部署 shell contracts）。它不等于 release readiness：外部漏洞数据、SBOM 和托管依赖审查保持 `not-run`，之后仍须完成真实 frontend/backend/broker build/tests，以及 `preflight:api-e2e` 和 E2E。

1. 通过环境变量或被忽略的本地配置提供真实的本地自动化凭据。
2. 按 `VALIDATION.md` 设置 preview-proxy 发布环境变量。
3. 在 `automation_tests/` 中运行 `npm run preflight:api-e2e`；它必须能够拒绝占位符、环境漂移、禁用 preview proxy、复用服务，以及 preview/backend 有限连通性失败。
4. preflight 通过后，运行 API automation 与 Playwright E2E，并归档报告。
5. 在验证摘要或私人本地审计说明中记录归档路径、退出码和剩余阻塞项。

历史归档证据可以用于说明测试组合，但不能作为清理后工作树的发布结论。

## 发布检查

在公开提交或 release snapshot 之前，应确认：

- `git ls-files --others --exclude-standard` 只包含预期的公开候选文件。
- 高风险后缀/路径扫描结果中不包含密钥、构建产物、报告、归档、依赖目录或私人运行时材料。
- 本地凭据仍在 Git 之外。
- 默认 SQL seed 数据中没有处于发布启用状态的默认登录账号。
- 按 `GENERATED_FILES.md` 保留生成文件。
- 按 `THIRD_PARTY_NOTICES.md` 保留第三方归属说明。
- `backend/configs/.instance_id` 必须保持未跟踪且被忽略；本地运行可生成该文件，但它不得进入公开快照。
- `frontend/src/typings/components.d.ts` 必须保持未跟踪且被忽略；本地构建可重新生成它，公开快照应依赖生成链而不是携带本地副本。

## 当前热点同步

- ThingsVis 宿主壳的最新落点是“父组件负责装配，生命周期与 transport 壳独立”：`ThingsVisAppFrame.vue` 当前实测约 352 行，`thingsvisAppFrameLifecycle.ts` 负责 iframe 初始化、`tv:ready` 调度、viewer/editor 分流与卸载清理，`thingsvisFrameTransportBridge.ts` 负责可信消息判定、`targetOrigin` 与 `tv:platform-data` 回推边界。
- broker 客户端契约的最新落点是“插件可见接口与协商选项分离”：`mqtt-broker/server/client.go` 当前只保留 `Client` 契约与连接状态常量，`mqtt-broker/server/client_options.go` 独立承载协商后的 `ClientOptions` 结构，`mqtt-broker/server/client_service.go` 独立承载 session 遍历、在线客户端读取与 session 终止语义。
- 自动化遥测主链的最新落点是“主调度留在主文件，条件与动作拆到旁路文件”：`automate_telemetry.go` 当前聚焦执行入口与场景编排，`automate_telemetry_device_condition.go` 负责设备条件求值，`automate_telemetry_action_dispatch.go` 负责动作分发与结果汇总。
- `edit-premise` 当前更接近文档化收尾而不是新一轮大拆分：`edit-premise.vue` 仍是前提编辑器宿主，但触发参数、条件组、事件参数条件和时间条件状态已经沉淀到 `premise-*.ts` helper 与 `PremiseScheduleConditionEditor.vue`。

## 上传 GitHub 前的维护路线

1. 先冻结当前热点拆分边界，避免在公开快照前把 `ThingsVisAppFrame`、broker client、`automate_telemetry` 或 `edit-premise` 再次回流成大文件。
2. 再完成文档同步，至少让根文档、目录 README、文件头注释和公开边界说明与当前工作树一致。
3. 接着复核公开候选，只保留源码、必要文档与有明确保留理由的生成物，持续排除本地凭据、验证归档、构建产物和一次性审计材料。
4. 最后按 `VALIDATION.md` 重建当前工作树的验证证据；没有这一轮新鲜证据，就不要整理 release snapshot 或声称上传就绪。

## 维护与审查建议

- 每次准备发布时，都应重新检查公开边界，因为未跟踪文件和本地报告很容易在并行工作中变化。
- 如果公开候选范围新增目录，应先判断它是源码、文档、测试资产、生成文件还是本地证据，再决定是否纳入。
- 发布说明中不要写“全部验证通过”，除非 `VALIDATION.md` 中的当前门槛已经基于当前工作树运行并归档。
- 对于 `backend/configs/rsa_key/*`、`backend/configs/conf-localdev.yml`、前端 `.env*` 和 `backend/cmd/aetherlink-device-autotest/docs/` 这类本地边界材料，优先策略是“保持忽略并留在私有环境”，而不是为了公开仓库一次性硬删导致本地环境失效。
