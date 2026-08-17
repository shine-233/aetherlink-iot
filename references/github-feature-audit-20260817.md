# GitHub 能力与门禁深度审计（2026-08-17）

本记录只描述当前可回读的 GitHub 状态和当前仓库源码；它不把“开关打开”、静态 YAML、历史告警数量或局部绿色检查写成完整产品验收。远端仓库为 [`shine-233/aetherlink-iot`](https://github.com/shine-233/aetherlink-iot)，审计时 `main` 为 `0dc124c89e9acb1d5189678c239bc88937c67b38`，本地工作树 clean。

## 结论速览

| 能力 | 当前事实 | 深度判断 |
|---|---|---|
| Actions/CI | 10 个 active workflow（含 Dependabot Updates/Dependency Graph），最近 main push 的 Source CI、Container build、CodeQL、Dependency review、Minimum quality gate 均有成功 run | 不是只写 YAML；PR/main 的主要静态、构建、测试和安全门禁真实运行过 |
| Actions 权限 | selected actions、SHA pinning required、默认 workflow token 为 read；当前 `uses:` 均为 40 位 SHA | 配置较强；仍须持续审计 Dependabot 对 action SHA 的更新 |
| Source CI | frontend lint/typecheck/build/unit、backend/broker/device Go test+build、automation contract tests | 对源码层有效；不等于 Compose 启动、外部 API 或物理设备验收 |
| Minimum quality gate | 真实执行 supply-chain、generated-artifact 和 deploy contract 脚本；现在固定 Node 22 | 这是离线/静态门禁，不冒充 runtime gate |
| CodeQL | Actions、JavaScript/TypeScript、Go 三条分析；`security-extended`；当前 open=0，历史 resolved=113 且全部 state=`fixed` | 当前 ref 的 CodeQL 门禁真实检查分析 commit/ref 和 open alerts；历史数量不是当前未修复告警 |
| Dependency Review | PR 和 main push 都运行，moderate 起步，runtime/development/unknown scope 都阻断 | 真实检查变更依赖；不替代全量依赖漏洞库存 |
| Dependabot | 12 个维护输入覆盖 Actions、2 个 npm、3 个 Go module、3 个 Dockerfile、3 个 Compose 输入；安全更新和 automated security fixes enabled | 配置真实；当前仍有 10 个普通更新 PR 需要逐项兼容性处置 |
| Secret Scanning | 基础扫描、Push Protection、Private vulnerability reporting enabled；non-provider patterns 和 validity checks disabled | 基础能力真实；两个高级项不能声称已开启 |
| Issues | `has_issues=true`；Bug/Feature Issue Form；`blank_issues_enabled=false`；当前公开 Issue 数为 0 | 配置真实，内容为空不是配置假象 |
| Discussions | `has_discussions=true`；6 个 GitHub 分类；当前讨论数为 0 | 功能真实可用，尚未有内容 |
| Projects | 用户项目 [AetherLink IoT Engineering](https://github.com/users/shine-233/projects/1) 存在，13 个字段、30 个条目 | 不是只有 `has_projects=true` 开关；该项目是 user-owned，所以 repository GraphQL 的 repo-owned projects 列表为 0 并不矛盾 |
| Release/GHCR/SBOM | v0.1.2 source/container workflow 成功，资产 checksum、source attestation、image digest attestation 已复核 | 发布链路真实；v0.1.2 source SBOM 是 source-manifest-only，不能冒充完整依赖解析/部署验收 |
| 分支保护 | main strict、14 个 required contexts、enforce admins、linear history、conversation resolution、禁止 force push/deletion | 门禁真实；审批数为 0、CODEOWNERS 非 required 是明确的策略弱点，不是隐藏的质量证据 |
| integration environment | 当前 secrets/variables=0；新增 protected-branches policy；配置为空时 workflow 已实测 fail-closed | 外部 API/账号/设备仍未运行，阻断是诚实的 |

## Actions 与 CI

仓库当前 active workflow 包括：

- Source CI；
- Container build；
- Minimum quality gate；
- Dependency review；
- CodeQL；
- Manual and nightly integration；
- Source release；
- Container image release；
- Dependabot Updates；
- Dependency Graph。

仓库 Actions API 当前返回：

```text
allowed_actions=selected
sha_pinning_required=true
default_workflow_permissions=read
can_approve_pull_request_reviews=false
```

所有当前 workflow 的 `uses:` 行都通过了 40 位 SHA pinning 检查。默认权限为 read，但发布 job 在需要时显式提升到 `contents: write`、`packages: write`、`id-token: write` 和 `attestations: write`；构建、测试和 CodeQL job 没有拿发布权限。

### 不足边界

- Container CI 当前是三个生产 Dockerfile 的 `linux/amd64` build-only；它没有证明 Compose 启动、数据库迁移、健康检查、API/E2E 或真实 MQTT 设备链路。
- Minimum quality gate 是离线脚本和部署合同门禁；它没有启动服务，也没有联网查询漏洞数据库。
- Integration workflow 已经配置了 fail-closed，但没有真实 environment inputs 时必须失败，不能把 skipped 写成通过。

## CodeQL 与依赖安全

CodeQL 当前分析三种语言/工作流：

```text
actions
javascript-typescript
go
```

Go lane 实际构建 `backend`、`mqtt-broker` 和 `backend/cmd/aetherlink-device-autotest`，不是只初始化 CodeQL 后跳过 build。当前 `security-extended` 查询集生效。

当前 GitHub API 事实：

```text
CodeQL open alerts: 0
CodeQL resolved alerts: 113
resolved state breakdown: fixed=113
```

历史 CodeQL 告警主要来自 `go/log-injection`、`js/path-injection`、`js/remote-property-injection` 等规则；它们现在都记录为 fixed，而不是 dismissed。这个事实证明当前结果没有被“批量关闭原因”伪装，但不代表 CodeQL 能覆盖业务授权、真实部署、设备协议或所有第三方漏洞。

Dependabot 当前 API 事实：

```text
Dependabot open alerts: 0
Dependabot fixed alerts: 109
Dependabot PRs: 55 total = 21 merged + 24 closed-unmerged + 10 open
```

10 个 open PR 的逐项处置见 [`dependabot-pr-disposition-20260817.md`](dependabot-pr-disposition-20260817.md)。它们不能只因为 CI 绿色就整体合并：其中包含数据库大版本、多数据库镜像、前端 major、lockfile 冲突、Release 工具链和真实业务兼容性风险。

## Issue、Discussion、Project

当前仓库本身：

- Issue 功能已启用；Bug report 和 Feature request 两个 Issue Form 已提交到源码；空白 Issue 被禁用；当前没有公开 Issue。
- Discussion 功能已启用；GitHub 返回 6 个默认分类；当前没有 Discussion 帖子。没有帖子不代表入口未启用。
- Projects 功能开关已启用，且 user-owned Project #1 `AetherLink IoT Engineering` 实际存在，拥有 13 个字段和 30 个条目。由于它是用户项目，不会出现在 repository-owned `repository.projectsV2` 列表中；应以 `gh project list --owner shine-233` 和项目页面为准。

## Release、GHCR、SBOM、provenance

已复核的正式版本为 [v0.1.2](https://github.com/shine-233/aetherlink-iot/releases/tag/v0.1.2)：

- Source release run [31964048006](https://github.com/shine-233/aetherlink-iot/actions/runs/31964048006) 成功；
- Container release run [31964048070](https://github.com/shine-233/aetherlink-iot/actions/runs/31964048070) 成功；
- source archive、`source-sbom.json` 和 `SHA256SUMS.txt` 可下载，checksum 与下载后重算值一致；
- 三个 source 资产的 `gh attestation verify` 成功；
- 三个 GHCR image digest 的 attestation 成功。

v0.1.2 的 source SBOM 明确标记 `completeness=source-manifest-only`。下一次 tag release 才会生成包含 declared-and-locked Go checksum 条目的新资产；不能修改不可变的 v0.1.2 资产，也不能用 SBOM 证明目标服务器或真实设备已部署。

## 分支保护与环境

`main` 分支当前要求 14 个 status context：frontend/backend/broker/device/automation、CodeQL 三语言、CodeQL alert gate、Dependency review、三个 container build 和 Offline release preflight。保护同时开启：

```text
strict=true
enforce_admins=true
required_linear_history=true
required_conversation_resolution=true
allow_force_pushes=false
allow_deletions=false
required_approving_review_count=0
require_code_owner_reviews=false
```

最后两项和审批数为 0 是可见的策略弱点。当前仓库是单维护者仓库，直接改成需要另一位审批会让维护者无法自行合并；因此本次不擅自改变审批策略，但必须把它记录为治理缺口。

`integration` environment 当前没有 variables/secrets，也没有真实 protection reviewers；本轮已增加 `protected_branches=true` 的 deployment branch policy，使未来注入 secrets 后只能从受保护分支使用该 environment。真实 API、MQTT、账号、generic emulator 联调仍需外部环境。

## 当前不能声称完成的事项

1. 10 个开放 Dependabot 普通升级 PR 尚未全部合并；它们已有处置记录，但高风险项仍需单独兼容性证据。
2. 没有创建未经用户指定版本号的新 Release；v0.1.2 已验证，但不是当前 main 的新产品版本。
3. integration environment 为空，所以真实 API/E2E/MQTT/物理设备没有运行。
4. Secret Scanning 的 non-provider patterns 和 validity checks 仍 disabled；没有可靠权限/计划证据前不能声称已开启。
5. main 没有要求第二位 reviewer，CODEOWNERS 也不是 required review；这是治理强度不足，不影响已配置的 status gates，但不能称作多人审批门禁。
