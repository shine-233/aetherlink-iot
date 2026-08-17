# GitHub 能力与门禁深度审计（2026-08-17）

本记录只描述当前可回读的 GitHub 状态和当前仓库源码；它不把“开关打开”、静态 YAML、历史告警数量或局部绿色检查写成完整产品验收。远端仓库为 [`shine-233/aetherlink-iot`](https://github.com/shine-233/aetherlink-iot)，本次最终审计的 `main` 为 `6511f90b1bf4c4219467c0b217d735f8f25e7e30`；正式 v0.1.6 tag 固定在该提交，不把旧 release tag 的提交冒充为当前 main。

## 结论速览

| 能力 | 当前事实 | 深度判断 |
|---|---|---|
| Actions/CI | 10 个 active workflow（含 Dependabot Updates/Dependency Graph），最近 main push 的 Source CI、Container build、CodeQL、Dependency review、Minimum quality gate 均有成功 run | 不是只写 YAML；PR/main 的主要静态、构建、测试和安全门禁真实运行过 |
| Actions 权限 | selected actions、SHA pinning required、默认 workflow token 为 read；当前 `uses:` 均为 40 位 SHA | 配置较强；仍须持续审计 Dependabot 对 action SHA 的更新 |
| Source CI | frontend lint/typecheck/build/unit、backend/broker/device Go test+build、automation contract tests | 对源码层有效；不等于 Compose 启动、外部 API 或物理设备验收 |
| Minimum quality gate | 真实执行 supply-chain、generated-artifact 和 deploy contract 脚本；现在固定 Node 22 | 这是离线/静态门禁，不冒充 runtime gate |
| CodeQL | Actions、JavaScript/TypeScript、Go 三条分析；`security-extended`；当前 open=0，历史 resolved=113 且全部 state=`fixed` | 当前 ref 的 CodeQL 门禁真实检查分析 commit/ref 和 open alerts；历史数量不是当前未修复告警 |
| Dependency Review | PR 和 main push 都运行，moderate 起步，runtime/development/unknown scope 都阻断 | 真实检查变更依赖；不替代全量依赖漏洞库存 |
| Dependabot | 12 个维护输入覆盖 Actions、2 个 npm、3 个 Go module、3 个 Dockerfile、3 个 Compose 输入；安全更新和 automated security fixes enabled | 配置真实；55 个历史 Dependabot PR 当前为 25 merged、30 closed-unmerged、0 open；未来 major 仍需逐项兼容性处置 |
| Secret Scanning | 基础扫描、Push Protection、Private vulnerability reporting enabled；non-provider patterns 和 validity checks disabled | 基础能力真实；两个高级项不能声称已开启 |
| Issues | `has_issues=true`；Bug/Feature Issue Form；`blank_issues_enabled=false`；当前公开 Issue 数为 0 | 配置真实，内容为空不是配置假象 |
| Discussions | `has_discussions=true`；6 个 GitHub 分类；当前讨论数为 0 | 功能真实可用，尚未有内容 |
| Projects | 用户项目 [AetherLink IoT Engineering](https://github.com/users/shine-233/projects/1) 存在，13 个字段、30 个条目 | 不是只有 `has_projects=true` 开关；该项目是 user-owned，所以 repository GraphQL 的 repo-owned projects 列表为 0 并不矛盾 |
| Release/GHCR/SBOM | v0.1.6 source/container workflow 成功，资产 checksum、CycloneDX SBOM、source/image provenance 与 attestation 已复核 | 发布链路真实；v0.1.6 固定在当前 main，但仍不能冒充目标部署、真实 API、生产 MQTT 或真实设备验收 |
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
Dependabot fixed alerts: 110
Dependabot PRs: 55 total = 25 merged + 30 closed-unmerged + 0 open
```

最后 6 个历史 open PR（#55、#57、#60、#63、#67、#70）的逐项处置见 [`dependabot-pr-disposition-20260817.md`](dependabot-pr-disposition-20260817.md)。它们现在均已关闭：#55/#57 由 #80 替代，#60 由 #79/#82/#83 拆分替代，#63/#67/#70 分别由 #78/#77/#76 替代；关闭不等于盲目合并，原数据库、GORM/Go、前端 major、lockfile 和脚本工具链风险均已通过替代 PR 的具体门禁或明确保留理由处理。

## Issue、Discussion、Project

当前仓库本身：

- Issue 功能已启用；Bug report 和 Feature request 两个 Issue Form 已提交到源码；空白 Issue 被禁用；当前没有公开 Issue。
- Discussion 功能已启用；GitHub 返回 6 个默认分类；当前没有 Discussion 帖子。没有帖子不代表入口未启用。
- Projects 功能开关已启用，且 user-owned Project #1 `AetherLink IoT Engineering` 实际存在，拥有 13 个字段和 30 个条目。由于它是用户项目，不会出现在 repository-owned `repository.projectsV2` 列表中；应以 `gh project list --owner shine-233` 和项目页面为准。

## Release、GHCR、SBOM、provenance

### 当前正式版本 v0.1.6

当前 `main` 与正式发布目标均为 `6511f90b1bf4c4219467c0b217d735f8f25e7e30`。v0.1.6 通过 tag-only 方式触发，让 `release.yml` 自己创建 Release，避免预创建 Release 与 source workflow 的创建步骤冲突。

- [v0.1.6 Release](https://github.com/shine-233/aetherlink-iot/releases/tag/v0.1.6) 已创建；[Source release 32008549565](https://github.com/shine-233/aetherlink-iot/actions/runs/32008549565)、[Container image release 32008549465](https://github.com/shine-233/aetherlink-iot/actions/runs/32008549465)、[Container build 32008549455](https://github.com/shine-233/aetherlink-iot/actions/runs/32008549455) 和 [Minimum quality gate 32008549458](https://github.com/shine-233/aetherlink-iot/actions/runs/32008549458) 均为 `success`。
- Release assets：`aetherlink-iot-v0.1.6.tar.gz` SHA-256=`8ab7fd332230f567ef23bb377bfde1d413935ea5435d41043c3ec4e3531db4c9`；`source-sbom.json` SHA-256=`520b01e4fb0fab8f1c2ce649bbfa949b407ff6c5a224a24ab54d1ecced570901`；`SHA256SUMS.txt` asset SHA-256=`f141fb9e26a375da913660d1cf52e606d0d14e3c6916ab4bb9b7881ae1aa98c7`。
- `source-sbom.json` 是 CycloneDX 1.6，包含 `1373` 个 components；source archive、SBOM、checksum 三个资产的 `gh attestation verify` 均成功，身份为 `Source release@refs/tags/v0.1.6`，且有 Rekor timestamp。
- GHCR v0.1.6 manifest digests：backend=`sha256:54c7fad54e1d7e82ecc71aec716c7620fbba8956db597729dde2004af93e7fc0`、frontend=`sha256:23fae3bcd2199c70f10e176add2af82dad47626013149c68b203890a969a9b44`、MQTT broker=`sha256:e1898a3160781e40319b4f108536809fc7538e20aacd2eb0666715996c5fd1e0`；三者的 `0.1.6`、`0.1`、`latest` 标签均指向对应 digest。
- 三个 GHCR digest 的 `gh attestation verify` 均成功，predicate 为 SLSA provenance v1，身份为 `Container image release@refs/tags/v0.1.6`，均来自 run `32008549465`，并有 Rekor timestamp。
- v0.1.4 曾经的失败 tag push 没有公开 Release/资产；v0.1.5 的 source run 因预创建同名 Release 失败，但 container run 成功推送了 `0.1.5` 镜像标签。v0.1.5 不应被当作完整 source release；v0.1.6 才是当前完整发布证据。

历史版本 [v0.1.3](https://github.com/shine-233/aetherlink-iot/releases/tag/v0.1.3) 仍固定在 `a61ff1fa5f28533478631f5e4df037ba9fd8eabe`；当前正式发布使用 v0.1.6，固定在 `6511f90b1bf4c4219467c0b217d735f8f25e7e30`。v0.1.3 的历史资产证据仍保留在下方，但不能代替当前 v0.1.6。

- Source release run [31993375439](https://github.com/shine-233/aetherlink-iot/actions/runs/31993375439) 成功；
- Container release run [31993375430](https://github.com/shine-233/aetherlink-iot/actions/runs/31993375430) 成功；
- source archive、`source-sbom.json` 和 `SHA256SUMS.txt` 可下载；archive 重算 SHA-256 为 `98b62eea7281258c1a40f0bae6d30718740e24e7db1c3f4a8426785b61ffc0e8`，source SBOM 重算 SHA-256 为 `aa63a6da5dfbb7ee04cd1fa15e59ec14428e23dc138e601f5c5adf65b478b0c7`；
- 三个 source 资产的 `gh attestation verify` 成功；source SBOM 为 CycloneDX 1.6，共 1320 个 components，其中 1316 个为 declared-or-locked，go.sum declaration relationship 为 533；
- 三个 GHCR image digest 的 attestation 成功：backend=`sha256:b5fc893a43b0cfec66bd90769d459c7434fab6af0f543b37be1c4dbe5c1dbc80`、frontend=`sha256:7908c4ef70e133a8c3fa66c987af043262ab3eaf0c88b0885cc8419432bff807`、MQTT broker=`sha256:24116f1c10ca1ac7fff123bef53e5abcd1799f0eb99daf5f9345d0510c8dbc29`。

v0.1.3 的 source SBOM 已包含 declared/locked 依赖证据，但它不包含随后合并的 #80/#81/#83/#85；v0.1.2 的 source SBOM 仍明确标记 `completeness=source-manifest-only`，作为历史资产保留。两者都不能用来证明目标服务器或真实设备已部署。当前 main 的完整 source/container 发布证据以 v0.1.6 小节为准。

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

1. Dependabot 当前没有 open PR 或 open alert，但历史 closed-unmerged 记录仍保留，未来大版本不能仅凭开关或绿灯合并。
2. v0.1.6 已创建并完成 source/container hosted 验证，固定在包含 #80/#81/#83/#85 的当前 main；release 成功也不等于真实环境部署验收。
3. integration environment 为空，所以真实 API/E2E/MQTT/物理设备没有运行。
4. Secret Scanning 的 non-provider patterns 和 validity checks 仍 disabled；没有可靠权限/计划证据前不能声称已开启。
5. main 没有要求第二位 reviewer，CODEOWNERS 也不是 required review；这是治理强度不足，不影响已配置的 status gates，但不能称作多人审批门禁。
