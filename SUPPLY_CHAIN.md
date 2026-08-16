# 供应链与依赖验证边界

本文件定义 AetherLink IoT 的依赖输入、离线默认检查和外部供应链能力。目标是让普通本地构建可复核，同时不把网络、SaaS 或额外扫描器伪装成私有部署的核心依赖。

## 本地默认（local-default）

运行：

```powershell
cd automation_tests
npm run preflight:supply-chain
```

该入口不联网、不安装工具，检查：

- Backend、MQTT Broker、设备 autotest 的 `go.mod` 与 `go.sum` 成对存在。
- Backend 和 Broker Docker builder 的 Go 版本不低于各自 module 的 `go` 指令。
- Frontend 的 `packageManager` 固定 pnpm 精确版本和完整性 hash。
- `pnpm-lock.yaml` 与 `pnpm-workspace.yaml` 同时存在。
- Frontend Docker build 使用 Corepack 和 `pnpm install --frozen-lockfile`。
- Frontend `package.json` 使用 SPDX `Apache-2.0` 声明，并核对 Frontend、Backend、MQTT Broker 的标准位置 `LICENSE` 文件包含对应 Apache-2.0/MIT 文本标识。

这些检查证明构建输入和组件级许可证声明边界没有被静默放松，不证明传递依赖许可证兼容，也不证明依赖当前不存在公开漏洞。

## 本地可运行的 SBOM（local source-manifest）

运行：

```powershell
cd automation_tests
npm run sbom:local
```

该命令只使用 Node.js 标准库，离线读取三份 `go.mod` 和 `frontend/pnpm-lock.yaml`，生成 CycloneDX 1.6-like JSON，组件哈希使用 SHA-256，并明确标注 `source-manifest-only`。它证明源码清单输入的可复核快照，不声称已经解析完整传递依赖、构建镜像或发布 subject；输出不应作为手写的静态发布结果提交。

## 可选外部（optional-external）

以下能力有价值，但需要额外工具、网络或发布流水线；缺少时不得阻止默认五服务栈启动：

- Go `govulncheck`：使用 Go 漏洞数据库分析实际可达调用；官方说明见 [Go Vulnerability Management](https://go.dev/doc/security/vuln/)。
- 依赖许可证 SCA：传递依赖的许可证探测和策略判断需要显式扫描工具链；本地检查只验证仓库组件声明，不输出兼容性假结论。
- 完整 resolved dependency SBOM：需要发布构建中的依赖解析和归档工具链；本地 `source-manifest-only` 不替代它。
- Docker image SBOM/attestation：需要镜像构建器、目标 image digest 和 attestation 发布/验证链路；可参考 [Docker SBOM attestations](https://docs.docker.com/build/attestations/sbom/)。
- CycloneDX/SPDX 发布格式和范围应按所选标准归档；CycloneDX 1.6 区分 build lifecycle 和 required/optional scope，见 [CycloneDX v1.6 JSON Reference](https://cyclonedx.org/docs/1.6/json/)。
- npm/pnpm advisory 检查：需要当前 registry/advisory 数据，应归档工具版本、时间和退出码。

上述 resolved/image SBOM 与漏洞、许可证、attestation 能力在默认本地 preflight 中保持 `optional-external / not-run`，不能输出假通过；正式 tag release 的 source-manifest SBOM、容器 BuildKit image SBOM 和 registry provenance/attestation 属于独立的托管发布证据。

## GitHub 托管检查（hosted-github）

仓库已经接入 [`.github/workflows/dependency-review.yml`](.github/workflows/dependency-review.yml)。它在 Pull Request 上调用 GitHub 的 Dependency Review action，依赖 GitHub dependency graph 和托管仓库权限，设置为发现 moderate、high 或 critical 风险时失败，并把摘要写入 PR。

这条 workflow 证明的是“本次 PR 的依赖变更经过 GitHub 托管检查”，不是本地 `preflight:supply-chain` 的替代品，也不等价于完整许可证兼容审计、运行时漏洞验证或镜像 SBOM。官方能力边界见 [GitHub Dependency Review](https://docs.github.com/en/code-security/supply-chain-security/understanding-your-software-supply-chain/about-dependency-review)。

本地入口仍保持离线和可复核；GitHub workflow 的排队、权限、dependency graph 或托管服务异常应记录为 hosted check 的失败/未运行，不得改写成本地通过。

## 工具链说明

- Backend 和 Broker module 当前声明 Go 1.25；Docker builder 使用更新的 Go 1.26.x。设备 autotest 是独立 Go 1.21 module，不参与服务镜像构建。
- Frontend Docker 使用 Node 22 和 Corepack；真正的 pnpm 版本由 `frontend/package.json` 的 `packageManager` 字段固定。
- Docker base image 当前固定版本 tag 但未固定 digest。digest 需要发布环境按目标平台解析和维护，属于发布流水线改进项，不应在无法拉取镜像的本地环境中猜写。

## 维护规则

- 修改 module、lockfile、Docker builder 或包管理器版本时，同步运行供应链契约和对应模块全量测试。
- 新增外部依赖先确认 license、NOTICE、运行必要性和本地替代；同步更新 `THIRD_PARTY_NOTICES.md`（如适用）。
- 本地 source-manifest SBOM 与完整 resolved/image SBOM 必须分层表述；漏洞扫描、完整 SBOM、attestation 和 hosted review 的 `not-run`/失败状态不得用本地清单快照或静态文档替代。发布 workflow 已有独立的 source/image SBOM 与 provenance 证据，但它们也不替代运行时部署验收。
