# AetherLink 模拟集成与发布边界（2026-08-17）

## 先给结论

本仓库现在有一条可以重复运行的**离线模拟集成 lane**。它会实际执行以下检查：

- 模拟 API 登录和带令牌的会话读取；
- 模拟业务流程：创建设备、观察在线状态、读取 telemetry、执行成功/失败/重试命令、清理设备；
- 内存 MQTT router：online/offline、telemetry、command、ACK success/failure/retry；
- synthetic RDI protocol replay；
- 默认 Docker Compose 的服务图、healthcheck、secret fail-closed 插值和 Dockerfile 静态 dry-run；
- 本地 CycloneDX source SBOM 生成、源文件指纹和“外部 registry enrichment 未运行”边界。

入口和契约测试是：

```text
automation_tests/scripts/run_simulated_integration_lane.js
automation_tests/tests/00_simulated_integration_lane.test.js
```

运行：

```powershell
cd automation_tests
npm run test:simulated-integration
npm test
```

`test:simulated-integration` 默认只使用 Node.js 标准库，HTTP 服务只监听
`127.0.0.1` 的临时端口，token 只保存在进程内存中。可以加 `--report-dir`
保存 JSON 证据，但报告不会包含 token、密码或 cookie。

这条 lane 的绿色结论是：

```text
status = simulated_pass
```

不是：

```text
real-api-passed
real-mqtt-passed
real-rdi-passed
production-deployed
```

程序对 `--rdi-mode=real`、`--api-mode=real`、`--mqtt-mode=real` 和
`--deployment-mode=real` 都会 fail-closed，返回 `external_blocked`，不会猜测
用户的账号、设备、broker 或服务器。

## 模拟结果与真实证据的区别

| lane | 模拟通过证明什么 | 仍不能证明什么 |
|---|---|---|
| API login | 本地 HTTP 合同、认证头、会话读取和错误边界 | 真实 API 地址、真实账号、真实权限、真实数据库 |
| business E2E | 有前置准备、用户式业务动作、命令结果、telemetry 和 cleanup 断言 | 真实前端浏览器、真实 API、真实业务数据和真实租户 |
| MQTT broker | topic、在线/离线、telemetry、ACK success/failure/retry 合同 | 真实 GMQTT broker、TLS/ACL、网络断线、真实 firmware |
| synthetic RDI | synthetic PID、voucher 边界、协议 envelope 和 ACK replay | 物理设备、真实 PID、真实 voucher、固件和设备验收 |
| Compose dry-run | service graph、healthcheck、Dockerfile 和缺 secret 时拒绝启动 | Docker image build、容器启动、迁移、跨服务运行时行为 |
| source SBOM | 仓库清单/锁文件组件、格式和源文件 hash | 完整镜像 SBOM、所有 registry metadata、生产部署等价性 |

真实外部项应继续显示为：

```text
real API login       = not-proven
real business E2E    = not-proven
real MQTT broker     = not-proven
physical RDI/device  = not-proven
target deployment    = not-run
Docker Compose       = not-run when Docker is unavailable
```

## 两个 Secret Scanning 高级项到底做什么

### `secret_scanning_non_provider_patterns`

基础 Secret Scanning 更擅长识别 GitHub 已知供应商的密钥格式，例如某个云厂商
或服务商的 token。`non_provider_patterns` 用来补充扫描那些**不像某个已知供应商
token**、但仍可能是凭据的字符串，例如内部系统 API key、私有 webhook secret、
自定义签名密钥或项目自定义格式。

打开后的好处是漏检更少；代价是自定义字符串和测试 fixture 更容易产生误报，团队
需要维护 allowlist 和告警处理流程。它不能自动证明密钥有效，也不能替代轮换密钥。

### `secret_scanning_validity_checks`

发现一个疑似供应商密钥后，GitHub 会在供应商允许的范围内询问该密钥是否仍然有效。
这样可以区分“历史上看起来像密钥”和“现在仍可使用的密钥”，便于优先处理真正危险
的泄露。

它可能让密钥值或相关指纹被发送给供应商，受供应商支持范围、隐私政策、组织策略和
GitHub 计划能力限制；也可能产生 API 请求和速率/隐私考量。因此不是所有仓库都能
打开。

当前仓库实际回读状态是：

```text
secret_scanning                       = enabled
secret_scanning_push_protection       = enabled
secret_scanning_non_provider_patterns = disabled
secret_scanning_validity_checks       = disabled
```

曾经尝试通过 API PATCH，但重新 GET 后仍为 `disabled`。准确说法是“当前账号/仓库
能力没有让这两个选项生效”，不要在文档或发布说明中写成 enabled，也不要把它当成
代码运行失败。基础扫描和 push protection 已足够提供第一层保护；高级项在计划支持、
隐私评估和误报处理流程明确后再开更稳妥。

## 仓库是不是完整，别人能不能部署

要分成三个层次：

### 1. 源码和部署入口：基本具备

仓库有源码、`docker-compose.yml`、Dockerfile、`.env.example`、启动脚本、迁移和
部署文档。别人可以按文档准备环境并尝试部署，核心服务边界是：

```text
PostgreSQL + Redis + 仓库内 MQTT broker + backend + frontend
```

但 `.env.example` 中的值是占位符，部署者必须自己生成真实密码、JWT key、MQTT
凭据和端口配置；不能把示例凭据直接当生产凭据。

### 2. 模拟集成：现在可以重复通过

上述模拟 lane 对软件合同和业务状态做了真实断言，并保留 cleanup 和 fail-closed
边界。这说明“程序在没有外部环境时的可测试部分”有证据。

### 3. 生产/真实环境完整性：尚未证明

因为当前机器没有 Docker CLI/daemon，也没有用户提供的真实 API 账号、MQTT broker、
物理 RDI 或目标服务器，所以不能把仓库称为“已经在生产等价环境完整验收”。更准确的
描述是：

> 其他人可以按文档尝试部署；源码和发布入口已具备，但本机完整 Compose 运行、真实
> 外部集成和目标服务器验收仍是待运行证据。

## GHCR 包公开 visibility 是什么，是否必须做

GitHub 仓库的公开状态和 GHCR 容器包的公开状态是两个独立字段：

```text
repository visibility != package visibility
```

GHCR visibility 回读要回答：别人是否可以匿名拉取这些镜像，以及 package 是否绑定
到了预期仓库。它对“别人能不能用发布镜像部署”很有用，但不影响源码本身能不能部署。

预期镜像是：

```text
ghcr.io/shine-233/aetherlink-iot-backend
ghcr.io/shine-233/aetherlink-iot-frontend
ghcr.io/shine-233/aetherlink-iot-mqtt-broker
```

当前本机 `gh` token 没有 `read:packages`，GitHub API 明确返回 403，因此这项不能
被判定为 public、private 或 internal；匿名 registry 的 401 也不能单独区分 package
不存在和 package 私有。要独立回读，需要同一轮仍在等待的命令完成：

```powershell
gh auth refresh --hostname github.com --scopes read:packages
gh api 'users/shine-233/packages?package_type=container&per_page=100'
```

只需要 `read:packages`，不需要为了回读 visibility 申请 `write:packages` 或
`delete:packages`。如果权限暂时拿不到，应记录为 `not independently verified`，
不能猜测结果。

## source SBOM 的外部 registry enrichment 是什么，是否必须做

当前 source SBOM 读取仓库里的 Go module 清单、Go checksum 和前端 lockfile，生成
CycloneDX，并记录源文件 hash。它回答：

```text
这份源码声明/锁定了哪些组件？输入文件有没有变化？
```

`registry enrichment` 是再去 npm、Go proxy、OSV/NVD 或其他 registry/漏洞服务，
为组件补充 package metadata、版本解析、license、PURL/CPE、漏洞和 registry URL。
它对供应链审计、许可证合规、漏洞优先级和 SBOM 消费者很有价值，但**不是源码能否
启动的必要条件**，也不是完整镜像 SBOM。

当前 lane 会验证：

```text
source SBOM format                = CycloneDX 1.6
scope                             = declared-and-locked-components
external.registry-enrichment      = not-run
deployment/image SBOM equivalence = not-proven
```

所以可以先发布源码和 source SBOM；若对外承诺“完整供应链 SBOM”，则还需要在有网络、
有 registry 凭据/服务策略的发布环境中再跑 enrichment，并记录输入、时间和失败边界。

## 完整部署等价性是什么，是否必须做

部署等价性不是“Compose 文件能解析”或“镜像存在”。它要证明同一版本在实际部署中
使用了对应的源码、镜像 digest、数据库迁移、配置、healthcheck、API、Redis、MQTT
和前端，并且关键业务行为一致。通常需要：

1. 固定 Git commit、release tag 和 image digest；
2. 在真实 Docker/Compose 环境构建或拉取镜像；
3. 使用干净数据库运行迁移并检查版本；
4. 通过 healthcheck、登录、关键业务命令和 telemetry；
5. 记录配置来源、端口、网络和 cleanup；
6. 在目标服务器或与之等价的隔离环境复现一次。

它对“现在就让别人下载源码尝试部署”不是硬性前置条件，但对下面这些说法是必须的：

```text
已在本机完整部署
release 与生产部署等价
镜像和源码完全一致
目标服务器验收通过
```

当前 Docker runtime 和目标服务器证据缺失，因此应保留 `deployment_equivalence =
not-proven`。这不是模拟 lane 失败，而是对证据范围诚实收口。

## 最终分层状态

```text
软件模拟链路                  = 可以通过
GitHub hosted CI              = 已通过既有门禁
source/release supply chain   = 已验证到 source/发布资产明确范围
GHCR package visibility       = 当前本机权限不足，未独立回读
真实 API/broker/device        = 需要真实输入和环境
生产部署等价性                = 不能由模拟替代
```
