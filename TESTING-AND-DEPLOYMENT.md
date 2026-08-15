# AetherLink IoT 开源版本地 Dev 测试与部署验收指南

这份文档是开源用户拿到源码后的第一条验证路径。目标不是让读者背完整的内部验证流程，而是让任何人都能在一台本地机器上回答三个问题：

1. Docker Compose 能不能把核心服务启动起来？
2. 前端、后端、MQTT Broker 和自动化测试能不能在自己的环境中运行？
3. 如果失败，问题更像是环境、配置、测试脚本，还是程序本身？

## 先看结论标准

本项目的默认部署是“源码构建的单机 Docker Compose dev/private 环境”，包含 PostgreSQL、Redis、AetherLink MQTT Broker、Go 后端和 Vue 前端。默认本地可视化使用 Native visualization；ThingsVis、SMTP、Market、地图 SDK 和其他外部服务是可选能力，不应成为核心平台首次启动的前置条件。

“能打开网页”不等于“可以发布”。一次完整的本地验收至少要经过：

- doctor 配置与机器预检通过。
- Compose 核心容器运行并健康。
- 后端 /health、/ready、部署健康接口和 Broker metrics 可访问。
- /first-device 能完成首台设备闭环：创建设备、发送一条遥测、看到在线/最新数据/第一张图表，并下载成功证明。
- 源码测试、API 自动化和 Playwright E2E 的结果按实际执行情况归档。

如果只完成了前两项，应写成“本地栈可启动”；如果完成到首台设备，应写成“本地核心业务可验收”；只有完整测试和发布检查也通过，才可以写成“当前提交具备发布候选证据”。不要用历史报告、覆盖率数字或单个绿色页面替代新鲜运行证据。

## 1. 环境要求

### 只做 Compose 本地部署

最小要求是：

- Windows、Linux 或 macOS。
- Docker Desktop / Docker Engine，以及 Docker Compose v2；Docker daemon 必须已启动。
- 首次运行需要拉取基础镜像并构建本地 backend、frontend 和 mqtt-broker 镜像，因此需要网络和足够的磁盘空间。建议至少预留 8 GB 可用空间。
- 默认 light 档适合约 1C/2GB 起步的本地试用；普通私有部署建议 standard、约 2C/4GB；较大部署建议 production、约 4C/8GB。这些是资源预设，不是设备数或消息吞吐承诺。

先检查：

~~~powershell
docker --version
docker compose version
docker info
~~~

~~~sh
docker --version
docker compose version
docker info
~~~

如果 docker info 失败，先启动 Docker Desktop / Docker Engine；这属于环境问题，不是应用测试失败。

### 还要运行源码测试时

源码测试使用的版本边界来自当前仓库：

- Node.js >=18。
- 前端包管理器使用 pnpm@10.8.0，以 frontend/package.json 的 packageManager 字段为准。
- 后端和 Broker 的 go.mod 要求 Go 1.25.0。
- Playwright 默认使用 msedge channel；如果本机没有 Edge，可设置 PLAYWRIGHT_BROWSER_CHANNEL 或 PLAYWRIGHT_BROWSER_EXECUTABLE_PATH。

检查：

~~~powershell
node --version
pnpm --version
go version
~~~

~~~sh
node --version
pnpm --version
go version
~~~

Node、pnpm 或 Go 版本不满足时，不要先改测试断言；先把环境对齐。只做 Compose 部署的用户不需要在主机上安装 Go，但 Docker 仍会在目标机上构建本地服务镜像。

## 2. 第一次启动本地 dev 环境

所有命令都从项目根目录执行。

### 推荐入口：一键脚本

Windows PowerShell：

~~~powershell
.\start-aetherlink.ps1 -Help
.\start-aetherlink.ps1 -Doctor -PerformanceTier light
.\start-aetherlink.ps1 -PerformanceTier light -Open
~~~

Linux/macOS：

~~~sh
sh ./start-aetherlink.sh --help
sh ./start-aetherlink.sh --doctor --performance-tier light
sh ./start-aetherlink.sh --performance-tier light
~~~

第一次没有 .env 时，初始化脚本会根据本地模式生成配置和本地密钥；它不会覆盖已经存在的 .env 或 Docker volume。若你希望手工控制配置，也可以先复制模板：

~~~powershell
Copy-Item .env.example .env
~~~

~~~sh
cp .env.example .env
~~~

手工编辑 .env 时至少要替换所有 change_me_* 值，给 GOTP_JWT_KEY 设置不少于 32 个字符的随机 secret，并保持以下成对配置一致：

| 目的 | 配置 | 必须保持一致 |
| --- | --- | --- |
| PostgreSQL | POSTGRES_PASSWORD、GOTP_DB_PSQL_PASSWORD | 两者相同 |
| Redis | REDIS_PASSWORD、GOTP_DB_REDIS_PASSWORD | 两者相同 |
| MQTT root | MQTT_ROOT_PASSWORD、GOTP_MQTT_PASS | 两者相同 |
| 浏览器/OTA 地址 | AETHERLINK_PUBLIC_URL、GOTP_OTA_DOWNLOAD_ADDRESS | 指向同一地址 |
| 设备 MQTT 地址 | AETHERLINK_MQTT_ACCESS_ADDRESS、GOTP_MQTT_ACCESS_ADDRESS | 指向同一 host:port |

真实密码、JWT secret、第三方 token 和公网配置不能提交到 Git。env.example 只能保留占位符。

### 直接使用 Compose（适合排查）

在真正启动前先验证 Compose 配置能展开：

~~~powershell
docker compose --env-file .env.example config --quiet
~~~

如果使用自己的 .env，把命令中的 .env.example 换成 .env。配置展开失败通常是 YAML、变量或密码占位符问题；此时还没有进入业务测试。

启动本地 dev 栈：

~~~powershell
docker compose --env-file .env up -d --build
docker compose ps
~~~

脚本入口仍然是首选，因为它会写入 resource tier、运行 doctor、等待健康检查并生成 verification/startup-<timestamp>/manifest.json。docker compose up -d --build 更适合定位单个容器或构建问题。

## 3. 启动后的最小健康检查

默认本地端口如下：

| 服务 | 本机地址 | 用途 |
| --- | --- | --- |
| 前端 | http://127.0.0.1:8080/ | 浏览器控制台 |
| 后端 liveness | http://127.0.0.1:9999/health | 进程是否存活 |
| 后端 readiness | http://127.0.0.1:9999/ready | 数据库、Redis、MQTT 等必需依赖是否就绪 |
| 部署健康 | http://127.0.0.1:9999/api/v1/deployment/health | 核心和可选能力的结构化状态 |
| Broker metrics | http://127.0.0.1:8082/metrics | MQTT Broker 运行指标 |
| MQTT | 127.0.0.1:1883 | 本机设备/测试客户端接入 |

Windows PowerShell 检查：

~~~powershell
docker compose ps
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/ | Select-Object StatusCode
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:9999/health | Select-Object StatusCode, Content
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:9999/ready | Select-Object StatusCode, Content
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:9999/api/v1/deployment/health | Select-Object StatusCode, Content
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8082/metrics | Select-Object StatusCode
.\deploy\verify.ps1
~~~

Linux/macOS 检查：

~~~sh
docker compose ps
curl -fsS http://127.0.0.1:8080/
curl -fsS http://127.0.0.1:9999/health
curl -fsS http://127.0.0.1:9999/ready
curl -fsS http://127.0.0.1:9999/api/v1/deployment/health
curl -fsS http://127.0.0.1:8082/metrics
sh ./deploy/verify.sh
~~~

/health 只证明进程存活，/ready 才是核心依赖就绪门槛。部署健康接口中的 ThingsVis、SMTP、Market 或地图状态可能是 disabled、configuration-required、external-blocked；只要默认核心检查健康，这些可选能力不会阻止 Native 核心启动，但必须在报告中如实标记。

查看失败原因：

~~~powershell
docker compose ps -a
docker compose logs --tail=200 postgres redis mqtt-broker backend frontend
~~~

~~~sh
docker compose ps -a
docker compose logs --tail=200 postgres redis mqtt-broker backend frontend
~~~

## 4. 首台设备闭环：判断“核心功能真的能用”

健康检查通过后，打开：

~~~text
http://localhost:8080/first-device
~~~

按页面完成以下步骤：

1. 第一次运行时创建超级管理员，并创建租户管理员。
2. 进入“接入第一台设备”工作区。
3. 检查部署状态。
4. 生成第一台设备。
5. 使用页面提供的浏览器测试/发送测试动作，或使用页面生成的 MQTT/HTTP 命令发送一条遥测。
6. 确认设备在线、最新遥测出现、第一张图表能显示。
7. 下载首台设备成功证明，并保留它的路径。

如果首个管理员页面打不开，可以在后端健康后使用一次性 CLI 入口：

~~~powershell
.\deploy\first-admin.ps1
~~~

~~~sh
sh ./deploy/first-admin.sh
~~~

CLI 只会在 GET /api/v1/tenant/setup-state 表明尚未初始化超级管理员时调用初始化接口；不要为了让测试变绿而重复创建管理员。完成后仍然要回到 /first-device 做设备和遥测闭环。

这一步是最小业务验收。只看到登录页、路由能打开或 HTTP 返回 200，不足以证明设备接入、遥测持久化和图表链路可用。

## 5. 源码级测试与构建

下面这组命令适合开源贡献者在自己的 checkout 中执行。依赖安装会产生可再生目录，失败后优先检查网络、磁盘和版本，不要删除源码。

### 前端

~~~powershell
Set-Location frontend
pnpm install --frozen-lockfile
pnpm typecheck
pnpm test:run
pnpm build
Set-Location ..
~~~

~~~sh
cd frontend
pnpm install --frozen-lockfile
pnpm typecheck
pnpm test:run
pnpm build
cd ..
~~~

pnpm typecheck 是 TypeScript/Vue 类型门槛，pnpm test:run 是 Vitest，pnpm build 证明当前前端可以生成 frontend/dist。构建产物是可再生的，不应提交。

### 后端和 MQTT Broker

~~~powershell
Set-Location backend
go test ./...
go build ./...
Set-Location ..\mqtt-broker
go test ./...
go build ./...
Set-Location ..
~~~

~~~sh
cd backend
go test ./...
go build ./...
cd ../mqtt-broker
go test ./...
go build ./...
cd ..
~~~

如果 Go 版本低于 1.25.0、网络无法下载模块或本机工具链缺失，先记录为环境阻塞；如果环境满足而测试断言或运行逻辑失败，再按程序问题排查。

### 部署脚本契约测试

Windows 至少运行：

~~~powershell
.\deploy\tests\doctor-pure-rules-contract.test.ps1
.\deploy\tests\verify-health-contract.test.ps1
~~~

Linux/macOS 或 WSL 可运行对应 Shell 套件：

~~~sh
sh ./deploy/tests/doctor-pure-rules-contract.test.sh
sh ./deploy/tests/verify-health-contract.test.sh
sh ./deploy/tests/backend-readiness-contract.test.sh
sh ./deploy/tests/network-segmentation-contract.test.sh
sh ./deploy/tests/redis-memory-contract.test.sh
~~~

这些契约测试检查脚本的纯规则和配置边界，不能替代真实 Docker、数据库、Broker、浏览器和设备运行。

## 6. API 自动化和 Playwright E2E

自动化测试必须连接真实的本地后端；不要把 mock API、占位账号、只返回 HTML 的错误代理或历史 reports 目录当作发布证据。

### 安装与列出模块

~~~powershell
Set-Location automation_tests
npm ci
npm run test:list
Set-Location ..
~~~

~~~sh
cd automation_tests
npm ci
npm run test:list
cd ..
~~~

test:list 只列出模块，不执行测试。它适合先确认当前 checkout 的 API/E2E 数量和模块发现是否正常。

### 本地便利预检

确认 frontend/dist 已由前面的 pnpm build 生成、后端监听 9999 后：

~~~powershell
Set-Location automation_tests
npm run preflight:local
Set-Location ..
~~~

preflight:local 会临时启动 9725 preview proxy，检查前端构建、代理、部署健康 JSON 和后端健康，然后关闭这个临时代理。它使用 local-lite 配置，不检查完整发布账号，也不等于业务 E2E 通过。

### 准备本地测试账号

~~~powershell
Set-Location automation_tests
npm run prepare:local-accounts
. .\.local\automation-env.ps1
Set-Location ..
~~~

~~~sh
cd automation_tests
npm run prepare:local-accounts
set -a
. ./.env.local
set +a
cd ..
~~~

如果数据库里已经存在超级管理员，脚本需要通过环境变量得到现有管理员账号；它不会猜密码，也不应该在源码里写真实密码。生成的 .local 文件、.env.local 和浏览器认证状态必须保持在被忽略目录。

### 严格 API/E2E 预检

发布式 API/E2E 需要一个真实的 preview proxy：前端 9725 的 API 请求必须代理到真实后端 9999，返回 JSON，而不是把 /api/v1/* 当作前端 HTML。准备好账号、构建产物和后端后运行：

~~~powershell
Set-Location automation_tests
$env:FRONTEND_URL = 'http://127.0.0.1:9725'
$env:PREVIEW_URL = 'http://127.0.0.1:9725'
$env:API_TARGET = 'http://127.0.0.1:9999'
$env:API_BASE_URL = 'http://127.0.0.1:9999/api/v1'
$env:PLAYWRIGHT_USE_PREVIEW_PROXY = '1'
$env:PLAYWRIGHT_REUSE_EXISTING_SERVER = '0'
npm run preflight:api-e2e
Set-Location ..
~~~

严格预检检查账号占位符、preview 端口、API target、代理模式和复用旧服务器设置，并请求 preview HTML、代理后的部署健康 JSON 和 backend health JSON。它不会替你启动完整 Docker 栈、登录、执行浏览器操作或证明业务正确；预检不通过时不要直接把后续失败归因于程序。

### API 和 E2E 全量运行

~~~powershell
Set-Location automation_tests
node run_tests.js --parallel --workers 2 --archive
node run_tests.js --include-e2e --parallel --workers 2 --archive
Set-Location ..
~~~

第一条只跑 API 自动化；第二条按统一 runner 先跑 API，再跑 E2E，并把本轮 reports/文件复制到 verification/automation-run-<timestamp>/。共享 automation_tests/reports/ 是临时输出，后续运行可能覆盖它；要引用结果，引用带 archive-manifest.json 的 verification/归档，并记录命令、退出码、commit、URL、账号来源和阻塞项。

E2E 默认使用 Edge。如果 Edge 不在默认位置：

~~~powershell
$env:PLAYWRIGHT_BROWSER_CHANNEL = 'chrome'
# 或：$env:PLAYWRIGHT_BROWSER_EXECUTABLE_PATH = 'C:\path\to\browser.exe'
~~~

### 每个页面的真实浏览器截图

如果需要对当前运行环境逐页检查，使用 live page audit，而不是只检查路由表。

前提是后端、9725 preview proxy、真实租户管理员认证状态已准备好：

~~~powershell
Set-Location automation_tests
$env:LIVE_PAGE_BASE_URL = 'http://127.0.0.1:9725'
$env:E2E_AUTH_STATE = (Resolve-Path '.\e2e\.auth\tenant-admin.json').Path
$env:LIVE_PAGE_OUTPUT_DIR = (Join-Path (Resolve-Path '..\verification').Path 'live-page-audit-current')
node .\scripts\live_page_audit_current.cjs
Set-Location ..
~~~

~~~sh
cd automation_tests
LIVE_PAGE_BASE_URL=http://127.0.0.1:9725 \
E2E_AUTH_STATE="$PWD/e2e/.auth/tenant-admin.json" \
LIVE_PAGE_OUTPUT_DIR="$PWD/../verification/live-page-audit-current" \
node ./scripts/live_page_audit_current.cjs
cd ..
~~~

这个脚本会对页面目录和补充路由逐页打开，记录最终 URL、可见文本、console/page/network 错误，并为每个页面写一张截图和 JSON/HTML 报告。它会创建并清理一个临时 Native board fixture；如果清理失败，报告必须标为失败。visual-page-sweep.js 是 mock 数据布局检查工具，适合看页面结构，但不能替代这里的 live audit。

页面结果要区分：passed、redirected、forbidden、runtime-error、optional-disabled 和 external-blocked。ThingsVis 在默认配置下属于 optional，不应为了让截图全绿而伪造外部服务响应；Native 页面才是默认核心验收面。

## 7. 服务器/私有化部署

本地 dev 通过后，再切换到服务器地址。浏览器和设备都必须能到达配置里的地址，不能把 localhost 留给远程设备。

Windows：

~~~powershell
$serverArgs = @{
  Server = $true
  Doctor = $true
  PublicUrl = 'http://192.168.1.10:8080'
  MqttAddress = '192.168.1.10:1883'
  BindAddress = '0.0.0.0'
  PerformanceTier = 'standard'
}
.\start-aetherlink.ps1 @serverArgs

$serverArgs.Doctor = $false
.\start-aetherlink.ps1 @serverArgs
~~~

Linux/macOS：

~~~sh
sh ./start-aetherlink.sh --doctor \
  --server \
  --public-url http://192.168.1.10:8080 \
  --mqtt-address 192.168.1.10:1883 \
  --bind-address 0.0.0.0 \
  --performance-tier standard

sh ./start-aetherlink.sh \
  --server \
  --public-url http://192.168.1.10:8080 \
  --mqtt-address 192.168.1.10:1883 \
  --bind-address 0.0.0.0 \
  --performance-tier standard
~~~

部署到公网或跨网段环境时，还需要自行处理：

- 防火墙只开放确实需要的端口；PostgreSQL 和 Redis 默认不映射到主机，不要额外暴露。
- 直接 Compose 默认是 HTTP 和明文 MQTT 1883，不自动提供 HTTPS/MQTTS。生产环境应在反向代理和 Broker override 中配置 TLS、证书和端口，并重新验证地址。
- AETHERLINK_PUBLIC_URL 是操作员访问的浏览器地址；AETHERLINK_MQTT_ACCESS_ADDRESS 是设备使用的 host:port。
- -Server/--server 模式会拒绝 localhost、loopback 和占位地址；这是防止“服务器启动但设备永远连不上”的配置门禁。
- 首次部署要留意数据库迁移、Docker volume、磁盘、Redis AOF、Broker 持久化和遥测 spool。单独的 PostgreSQL dump 不覆盖 Redis、Broker、上传文件或 spool volume。

服务器模式仍然要打开 AETHERLINK_PUBLIC_URL/first-device 完成首台设备闭环；仅从服务器本机访问首页不代表远程浏览器和设备都能工作。

## 8. ThingsVis、SSO 和外部能力的边界

默认核心平台不依赖 ThingsVis：Native visualization 是默认 provider。ThingsVis Server/Studio、HTTP adapter 和 ThingsVis SSO 是可选集成，只有在启用对应 optional Compose profile、配置 secret、外部地址和真实账号后，才执行 ThingsVis 专项 E2E。

因此：

- 没配置 ThingsVis 时，部署健康结果显示 disabled 或 configuration-required 是预期状态，不是核心部署失败。
- 已配置但外部服务不可达时要记录 external-blocked，不能改成“通过”。
- ThingsVis SSO 覆盖属于可选集成的专项验收，至少要验证登录跳转、回调/凭证交换、iframe 或 viewer/editor 加载、可信消息来源和登出/过期行为；它不属于 Native 核心的最小部署门槛。
- SMTP、Market、地图和真实第三方服务同样不能用本地 fake 或 route.fulfill 伪装成生产成功。

## 9. 失败分类：环境、配置、测试还是程序

按照下面的顺序分类，能减少反复重跑：

| 现象 | 首先归类 | 下一步 |
| --- | --- | --- |
| docker、node、pnpm、go 找不到；Docker daemon 不可达；磁盘/内存不足 | 环境问题 | 安装或启动工具，释放空间，重新运行同一命令 |
| Compose config 失败、密码变量为空、地址仍是 localhost、端口被占用 | 配置/环境问题 | 修正 .env、端口或 firewall，再跑 doctor |
| 镜像构建失败、Go/Node 依赖下载失败、Docker registry 超时 | 环境/外部依赖问题 | 看网络、代理、磁盘、锁文件和 registry；不要先改业务代码 |
| 容器启动但 /ready 或部署健康失败，日志出现迁移、连接、权限或 panic | 程序或部署配置问题 | 保存 docker compose logs、响应体和 startup manifest，定位对应服务 |
| API 返回状态/响应体与断言不一致，手工请求也复现 | 程序契约问题 | 检查 API、数据库副作用、权限和测试期望，必要时修代码和测试 |
| 只有测试失败，但手工路径通过；测试使用旧账号、旧端口、旧 proxy 或 stale report | 测试/环境问题候选 | 清掉本轮临时报告和旧进程，重新生成账号/代理并复现；不要放宽断言掩盖问题 |
| 只失败 ThingsVis/SMTP/Market/地图，核心 Native 和首台设备正常 | 可选外部能力阻塞 | 标记 optional/external-blocked，只有配置该能力的发行目标才阻断 |

每次报告失败时至少保留：执行命令、退出码、当前 commit、Node/Go/Docker 版本、.env 是否为新生成（不要保存 secret）、容器状态、相关日志、接口响应和截图。这样别人才能在自己的机器上复现，而不是只看到一句“测试没过”。

## 10. 通过/失败判定清单

### “我的机器可以部署”

- [ ] doctor 无 error。
- [ ] docker compose ps 中 PostgreSQL、Redis、mqtt-broker、backend、frontend 均运行并健康。
- [ ] 首页、/health、/ready、部署健康接口和 Broker metrics 均返回预期结果。
- [ ] /first-device 完成创建设备、发送遥测、显示图表和下载成功证明。

### “我的源码可以参与发布验证”

- [ ] 前端 typecheck、Vitest、build 通过。
- [ ] backend 和 mqtt-broker 的 go test ./...、go build ./... 通过。
- [ ] npm run preflight:local 通过。
- [ ] 真实本地账号、严格 preflight:api-e2e 和 node run_tests.js --include-e2e --archive 通过，或每个未通过项都有明确阻塞分类。
- [ ] 页面 live audit 的错误、重定向、无权限和 optional 状态已人工审阅，截图归档可打开。
- [ ] 新鲜报告放在 verification/<timestamp>/，没有把 automation_tests/reports/ 的旧文件当作当前结果。

任何未勾选项都意味着结论需要降级。例如“源码单测绿但 Docker 未启动”只能叫“静态/源码检查通过”；“首页和登录页能打开但首台设备失败”不能叫“全部功能可用”。

## 11. 停止服务、清理和重新开始

普通停止不删除数据：

~~~powershell
docker compose stop
docker compose down
~~~

重新构建服务但保留 volume：

~~~powershell
docker compose up -d --build
~~~

docker compose down -v 会删除 PostgreSQL、Redis、Broker、上传文件和 spool 等 Compose volume，是破坏性重置。执行前必须确认备份和恢复演练，不要把它当成普通“清缓存”命令：

~~~powershell
.\deploy\backup.ps1
docker compose down -v
~~~

开发测试产生的 frontend/dist、frontend/node_modules/.vite、automation_tests/reports、Playwright test-results 和 _localrun 是可再生输出，但当前 verification/归档、部署包、数据库/Redis/MQTT 数据和正在使用的运行目录不是默认清理目标。清理前应先确认路径、消费者、归档保留要求和活动进程。

## 12. 与其他入口的关系

- 快速启动和首次设备路径：START-HERE.md。
- 现有部署脚本与备份/迁移细节：deploy/README.md。
- 多层验证策略、覆盖率和发布证据边界：VALIDATION.md。
- 自动化脚本、账号和预检说明：automation_tests/README.md。

本文件是给开源用户的一次性验收入口；如果它与脚本行为不一致，应以当前脚本和新鲜运行结果为准，并在同一轮修正文档，而不是继续沿用过期命令。
