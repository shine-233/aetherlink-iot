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

当前发布标记为：`source_package_boundary=public-source`、`github_upload=executed`、`source_release=v0.1.2-published-and-verified`、`source_sbom=v0.1.2-source-manifest-only`、`next_source_sbom=declared-and-locked-pending`、`ghcr_release=v0.1.2-published-and-attested`、`real_rdi_status=not-tested`、`target_deployment_status=pending`、`production_signoff=not-ready`。ThingsVis 仍是有源码和合同引用的 optional legacy compatibility provider，不因 Native 可运行而删除；`negative-menu` 是 ownership rejection 测试场景，不是待清理服务。

## GitHub 托管功能状态

当前已启用或已接入：

- Actions、SHA pinning required、只读默认 `GITHUB_TOKEN` 权限，以及源码 CI、Minimum quality gate、CodeQL（GitHub Actions、Go、JavaScript/TypeScript）、Dependency Review 和手动/夜间 integration workflow。
- Dependabot alerts、security updates、automated security fixes，以及 GitHub Actions、frontend/automation npm、三个 Go module 和三个 Docker 目录的版本更新配置。
- Secret Scanning、Push Protection、Issues、Discussions、Projects、Wiki、Issue Forms、PR 模板和 CODEOWNERS。
- `integration` environment；当前没有真实 API、账号、设备凭据或生产 secrets，因此手动/夜间 workflow 默认不会伪造 live 验收。
- `.github/workflows/release.yml` 和 `.github/workflows/container-release.yml` 只在正式 tag push 时发布。`v0.1.2` 的 source/container hosted runs 已成功完成，source release 确实附带 checksum、source SBOM 和 source attestation，三个容器镜像也完成了 BuildKit SBOM、maximum-detail provenance 和 digest attestation；具体资产、digest 和复核命令见下节。注意：`v0.1.2` 的 source SBOM 是本次 Go `go.sum` 深度增强之前生成的 `source-manifest-only` 版本，不能冒充下一版的 `declared-and-locked-components` 输出。

### 2026-08-17 继续收口记录

- GitHub 当前历史 Dependabot PR 共 38 个：14 个已合并、24 个关闭未合并、0 个 open；不存在当前仍等待逐个合并的“28 个普通 open PR”。逐项分类、当前 manifest/lock 证据和后续动作见 [`references/dependabot-pr-disposition-20260817.md`](references/dependabot-pr-disposition-20260817.md)。
- 新增 `.github/workflows/container-ci.yml`：PR、`main` push、正式 tag 和手动运行都会对 backend/frontend/MQTT broker 三个生产 Dockerfile 做 `linux/amd64` build-only。该 job 不登录 GHCR、不 push、不申请 `packages: write`；三个 check 也已接入两个 tag release workflow 的 required-check 轮询。它仍不是 Compose 启动、迁移、API/E2E 或真实设备验收。
- `.github/dependabot.yml` 现在对 `backend/cmd/aetherlink-device-autotest` 同时覆盖普通 minor/patch 和 security updates；合同测试还会防止新增维护 manifest 后没有 Dependabot entry。
- `integration-nightly.yml` 现在有显式 `Integration result` 汇总 job。缺少 environment 配置时配置门禁失败，下游 live API/E2E/device job 不会被当成通过；任何 skipped、失败或未运行都会以 fail-closed 结果结束。当前 integration environment 仍没有真实变量/secrets，所以本轮没有启动真实 API、账号、MQTT 或设备验收。
- 对两个 Secret Scanning 高级选项执行了一次真实 GitHub API PATCH：请求返回成功，但回读仍为 `secret_scanning_non_provider_patterns=disabled`、`secret_scanning_validity_checks=disabled`；基础 Secret Scanning、Push Protection、Dependabot security updates 保持 enabled。当前 API 的仓库 `plan` 不公开给本次 token，因此只能确认“写请求没有改变状态”，不能把 disabled 归因到某个具体套餐名称。

### v0.1.2 Source Release 端到端证据

本次复核使用当前 GitHub 公开资产和托管运行，而不是只读 workflow YAML：

- tag `v0.1.2` 指向 `1c76f346e2136442b217ec3501e7231987d569da`；Source release run [31964048006](https://github.com/shine-233/aetherlink-iot/actions/runs/31964048006) 与 Container image release run [31964048070](https://github.com/shine-233/aetherlink-iot/actions/runs/31964048070) 均为 `push` 事件、`success`，且三个 container matrix job 全部成功。
- [v0.1.2 Release](https://github.com/shine-233/aetherlink-iot/releases/tag/v0.1.2) 的三个资产均可下载：`aetherlink-iot-v0.1.2.tar.gz`、`source-sbom.json`、`SHA256SUMS.txt`。
- 下载后 SHA-256 已重新计算并与 `SHA256SUMS.txt` 匹配：archive=`d99b67e0a0864ad2a26aafd98fd272f6e578a40fc968ab16eeff02ebb3b240a1`，source SBOM=`a3e9b0dc529af4b3ef324fee20e52c50f0ce13d621624021bb09931fbc6368ea`。
- archive 目录条目（去掉根目录）为 `4215`，Git tree recursive 结果为 `4215`，两组路径差异为 `0`，且 GitHub tree 未截断。
- 三个 source 资产的 `gh attestation verify` 均通过，并强制校验仓库 `shine-233/aetherlink-iot`、workflow `.github/workflows/release.yml`、`refs/tags/v0.1.2` 和 source SHA `1c76f346...`；每个结果都有 `1` 个 verified transparency timestamp。
- 该版本 `source-sbom.json` 的元数据明确为 `completeness=source-manifest-only`，外部 dependency resolution、registry enrichment 和 container attestation 均为 `not-run`；这是真实资产的范围声明，不是缺陷被隐藏。当前分支正在把 Go `go.sum` 校验条目加入下一版的 `declared-and-locked-components` 模式，合并并重新打 tag 后才会产生新的 hosted artifact。
- GHCR 的 `0.1.2`、`0.1`、`latest` 三个 tag 均存在并解析到同一版本 digest：backend=`sha256:fbdcf639be9c0326f0e020c039ef15fc28710d953031484a63a9a09ccfa461a8`、frontend=`sha256:9871f07ae786abc8b46c7f20bf757c97df58c4fa360b7561de882850fa679afa`、MQTT broker=`sha256:96ab7e85a3d24ade54d2c743a51f962c895fda8d78956813fe01a43ab86e5c09`；三个镜像的 `gh attestation verify` 也均以 `exit code 0` 通过，并指向 `container-release.yml@refs/tags/v0.1.2`。

### GHCR 首次发布证据（历史 v0.1.0 手动运行）

2026-08-16 的托管运行 [31925704773](https://github.com/shine-233/aetherlink-iot/actions/runs/31925704773) 成功发布了三个公开 GHCR package：

- [aetherlink-iot-backend](https://github.com/users/shine-233/packages/container/package/aetherlink-iot-backend)：`sha256:b7c4cf7dfbfdd717eff369583aa69abb61ffbc7de9311ec652ec231f8041d0ca`
- [aetherlink-iot-frontend](https://github.com/users/shine-233/packages/container/package/aetherlink-iot-frontend)：`sha256:3f7dfab2e9782ceeab8553c5053dbbff4c2df60cd0e70820d70126cc8fd423e2`
- [aetherlink-iot-mqtt-broker](https://github.com/users/shine-233/packages/container/package/aetherlink-iot-mqtt-broker)：`sha256:58e906a0218d2574d85291897dae696f9934156ef5a2273e728be5f41552758b`

托管日志确认三个 job 都完成了镜像构建和推送，BuildKit 的 SBOM 与 `provenance: mode=max` 参数生效，并且每个实际镜像 digest 都完成了 registry attestation 创建和上传。GitHub API 也能返回三个 digest 对应的 Sigstore bundle。

本机使用 GitHub API 下载的 bundle 对三个 digest 分别运行了 `gh attestation verify`，并强制校验了仓库 `shine-233/aetherlink-iot`、签名 workflow `.github/workflows/container-release.yml`、源 ref `refs/heads/main` 和源提交 `57c7a40c58ebc65e4da87381a3bc45c02e141539`；三次均通过，且包含 Rekor 时间戳。这是镜像 digest、签名身份和托管构建证据，不是目标服务器、外部 API、Broker 或真实设备验收。

历史证据：该次运行来自旧版 workflow，并通过 `workflow_dispatch` 手动指定了 `tag=v0.1.0`；workflow 的 checkout 使用了这个 tag 作为构建源码输入，但是 attestation 证书和 SLSA provenance 的上下文记录为 `refs/heads/main` 与提交 `57c7a40`，而不是 tag-triggered 的 `refs/tags/v0.1.0`。这不代表当前 `.github/workflows/container-release.yml` 仍支持手动 tag 输入；当前发布 workflow 只接受正式的 `v*.*.*` tag push。该历史镜像只应表述为“已发布的 tag-selected source 镜像”，不能宣称它是由 `v0.1.0` tag 事件触发的 provenance。若需要严格的 tag provenance，应使用后续新的 patch tag 触发当前 tag workflow；不可变的 `v0.1.0` Source Release 不回写。

当前尚未完成：真实 API/E2E/设备和目标部署验收；`integration` environment 当前没有任何 secrets 或 variables，因此 workflow 会在配置门禁处 fail-closed。`main` 分支保护、正式 Git tag/Source Release、v0.1.2 source/container 发布及其资产/attestation 已完成并已验证。Secret Scanning 的 non-provider patterns 与 validity checks 当前仍为 `disabled`，不能在文档中写成已启用；其是否为具体计划限制也没有足够证据可断言。

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
