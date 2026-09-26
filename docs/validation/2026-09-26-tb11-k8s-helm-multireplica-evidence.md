# 2026-09-26 TB-11 实施证据（K8s/Helm 编排与多副本——部署面）

> 对应 `ROADMAP.md` §4.1 TB-11 行（部署架构与水平扩展）与 §5.2 批 3（部署与边缘）。
> 缺口编号沿用缺口体系（§1.5）。分支 `feature/roadmap-complete-tb-tp-parity`。
>
> **口径声明（重要）**：本证据文档由 TB-11 实施会话本人整理，与批次二证据文档的"更新会话转录"口径不同——本文全部"通过/全绿"结论均来自实施会话在本会话内实跑的命令（见 §3），未实跑的验收点一律写入 §4 residual，不以文档记载替代运行面。

## 1. 交付范围与改动文件

TB-11 的能力面在部署/编排域：本项**无 REST 契约面、无前端页面面**（`backend/internal/helmchart` 为离线校验库，未被 `router/`、`internal/api/`、`internal/service/` 任何文件 import，本会话 grep 实测 0 接线点；`frontend/` 无任何改动）。按任务书口径：第一阶段（后端面）交付 chart/override/校验单测/README 章节；第二阶段（前端与测试收口）确认无前端四件套与契约测试可做，做文档收尾（即本文）。

**改动/新增文件（共 18 个）**：

- `deploy/helm/aetherlink/Chart.yaml`——chart 元数据；appVersion 与 `backend/pkg/global/global.go` 的 `VERSION`（0.0.23）由单测双向钉死。
- `deploy/helm/aetherlink/values.yaml`——backend/broker/frontend/postgres/redis 五组工作负载默认值 + `postgresql.enabled`/`redis.enabled` 依赖开关 + `secrets` 密钥区（默认空串、模板 required fail-fast）。
- `deploy/helm/aetherlink/NOTES.txt`——安装后提示（迁移串行化、/ready 验证、多副本前提）。
- `deploy/helm/aetherlink/templates/statefulset-backend.yaml`——后端 StatefulSet：45 个 env 键、双探针（/health liveness + /ready readiness）+ startupProbe、资源限额、cap_drop ALL/no-new-privileges/runAsUser 10001（对齐 compose）、files/telemetry-spool/uplink-spool 三个 volumeClaimTemplates。
- `deploy/helm/aetherlink/templates/statefulset-mqtt-broker.yaml`——broker StatefulSet：20 个 env 键（含 `GMQTT_MQTT_SESSION_REVOCATIONS_BROKER_ID` 取 fieldRef metadata.name）、aetherlink.yml ConfigMap 挂载、/metrics 探针。
- `deploy/helm/aetherlink/templates/deployment-frontend.yaml`——无状态 Deployment + nginx.conf ConfigMap 覆盖挂载（/etc/nginx/nginx.conf subPath）。
- `deploy/helm/aetherlink/templates/service-backend.yaml`、`service-mqtt-broker.yaml`、`service-frontend.yaml`、`postgres.yaml`（STS+Service+headless）、`redis.yaml`（STS+Service+headless）。
- `deploy/helm/aetherlink/templates/secret-aetherlink.yaml`——chart Secret（`secrets.existingSecret` 非空时不渲染）。
- `deploy/helm/aetherlink/templates/configmap-broker-aetherlink.yaml`——aetherlink.yml 运行时配置（占位密钥经 GMQTT_* env 覆盖，对齐 compose 挂载 example 文件的既有口径）。
- `deploy/helm/aetherlink/templates/configmap-frontend-nginx.yaml`——frontend nginx 配置（仅 proxy_pass 目标换成 Release 内后端 Service DNS）。
- `deploy/docker-compose.replicas.yml`——backend+mqtt-broker 双副本 override 样板（`deploy.replicas: 2` + `ports: !override []`，注明三条硬前提：本地卷换共享存储/外部 PG、MQTT_BROKER_ID 单值无法按副本区分、端口发布冲突需 compose>=2.24）。
- `deploy/README.md`——新增 "Kubernetes (Helm) Deployment And Multi-Replica (TB-11)" 章节（安装步骤、首启迁移、多副本边界、compose override 前提）。
- `backend/internal/helmchart/render.go`——与 Helm 兼容的最小 text/template 渲染器（missingkey=zero，funcMap：quote/default/required/int/until，语义对齐 sprig）。
- `backend/internal/helmchart/manifests_check_test.go`——manifests 校验单测（见 §2/§3）。

## 2. 关键设计决策

1. **backend/broker 选 StatefulSet 而非字面 Deployment**：①telemetry/uplink spool 是 per-replica 耐久层，跨副本共享目录会被并发回放写坏，`volumeClaimTemplates` 仅 StatefulSet 支持；②`OrderedReady` 串行化首次启动，保证 1 副本完成全链迁移（`initialize/pg_init.go` `CheckVersion` 1..VERSION_NUMBER，空库自动建 sys_version 后逐号执行，故 chart 无需 postgres entrypoint 初始化脚本）并通过 `/ready` 后才放行下一副本。frontend 无状态用 Deployment。
2. **多副本 broker 身份**：`GMQTT_MQTT_SESSION_REVOCATIONS_BROKER_ID` 经 `fieldRef: metadata.name` 取 StatefulSet Pod 名（`<release>-mqtt-broker-<ordinal>`，跨重启稳定，满足 `mqtt-broker/plugin/aetherlink/config.go` broker_id 正则/非空 fail-fast 校验）；backend 的 `GOTP_MQTT_SESSION_REVOCATIONS_REQUIRED_BROKER_IDS` 渲染为空格分隔的全部副本 ID——`internal/service/mqtt_session_revocation.go:162` 经 viper `GetStringSlice` 读取（空白切分；逗号会产生单条含逗号条目、被 `^[A-Za-z0-9._:-]+$` 规范化拒绝）。吊销 ACK 按 distinct broker_id 计数（同文件 `mqttSessionRevocationRequiredAcksComplete`），副本集扩张后语义仍成立。broker `GMQTT_PERSISTENCE_TYPE=redis`（会话/订阅/QoS 队列共享 Redis）为 chart 强制默认，多副本禁改 memory。
3. **对齐由单测机械强制**（不靠人工口径）：backend env 45 键 / broker env 20 键与根 `docker-compose.yml` 对应服务的 `environment` 键集合精确相等；容器端口取自 compose `ports` 解析（backend 9999、broker 1883+8082、frontend 8080、postgres 5432；redis 6379 为 compose 完全不发布端口时的必要差异，测试内注明）；探针路径与 compose healthcheck 字符串绑定（/ready、/health、/metrics）；`Chart.yaml appVersion == global.VERSION`；frontend nginx ConfigMap 与 `frontend/nginx.conf` 做 location 集合 diff（仅 proxy_pass 目标随 Release 名替换，禁止残留 `http://backend:9999`）。
4. **无迁移、未改 VERSION_NUMBER**：chart 不含 SQL 面；数据库 schema 完全由 backend 启动迁移链承担（对齐 compose 单节点既有行为）。

## 3. 实测记录（本会话实跑命令与结果）

- `go build ./...`（backend/）→ exit 0。
- `go test ./internal/helmchart/ -count=1 -v` → 5 用例全 PASS：
  - `TestRenderedHelmManifestsParseAndCarryRequiredKeys`（主校验：values.yaml+Chart.yaml 加载、全部 11 个模板文件渲染、逐文档 yaml.Unmarshal、kind 数量 pin（4 STS+1 Deploy+9 Svc+1 Secret+2 CM）、Secret 五键非空、broker CM 占位标记、nginx location 集合 diff、工作负载结构必填键（replicas/selector/探针三件套/资源限额/安全上下文）、env 键集合与端口对齐 compose、secretKeyRef 口径、单副本 required_broker_ids）
  - `TestMultiReplicaScenarioInjectsBrokerIdentities`（双副本渲染：backend `GOTP_MQTT_SESSION_REVOCATIONS_REQUIRED_BROKER_IDS` == 全部两个 Pod ID；broker id 走 fieldRef metadata.name；files 默认 accessModes 钉住 RWO——多副本 RWX 属部署侧显式决定）
  - `TestEmptySecretsFailFast`（secrets 全空渲染必须失败且原因来自 required）
  - `TestDependencySwitches`（postgresql/redis 关闭后不渲染对应对象，backend/broker 数据面地址切 external.*）
  - `TestComposeReplicasOverrideDeclaresDualReplicaPremises`（override 文件 replicas==2、ports 为空、前提标注关键词存在）
- `go test ./internal/dal/ ./internal/api/ ./initialize/ -count=1` → 全 ok（含静态守卫 `TestTenantScopeQueryAudit`、`TestRequestPathContextBackgroundBudget`、`TestMigrationFilesMatchVersionNumber`）。
- `gofmt -l internal/helmchart/` → 无输出（经 `gofmt -w` 修正一次后复验）；`go vet ./internal/helmchart/` → 通过。
- 第二阶段（2026-09-26）复跑：`go test ./internal/helmchart/ -count=1 -v` 5 用例 PASS；`go build ./...` exit 0；三守卫包 ok（确认并行会话改动未破坏本项交付）。
- 无 helm/kubectl/docker 二进制（`helm version`/`kubectl version --client`/`docker --version` 均不存在）——凡依赖真实工具链的验证见 §4。

## 4. residual 与明确不做（诚实边界）

1. **【明确不做】真实集群安装验证与 broker federation 接线演练（需 K8s 环境，residual）**：未做 `helm install`/`helm template`/`helm lint` 实测、未做 pod 调度与探针真实行为验证、未做后端+broker 双副本"会话不粘本地卷、吊销跨 broker 收敛"演练；federation 插件（`mqtt-broker/plugin/federation/`，Serf/gRPC）保持默认关闭，chart 不接线。chart 渲染面由 §3 单测覆盖，但**渲染面通过不等于安装面通过**。
2. 契约测试：本项无 REST 契约面（helmchart 包零 HTTP 接线，grep 实测），按任务书明确不创建。
3. 前端面：本项无前端页面/路由/菜单种子/四语言改动（部署编排无操作 UI），前端四件套不适用。
4. Ingress/NodePort 对外暴露、TLS(8883) 启用、TimescaleDB 镜像变体、chart 打包进 `deploy/package.sh` 产物：属部署侧显式决定或另立项，未做。
5. 全量 `go test ./...` / 全量 vitest / OpenAPI 重生成按约定由编排方统一执行，本会话未跑（本项未产生 OpenAPI 变更面）。
