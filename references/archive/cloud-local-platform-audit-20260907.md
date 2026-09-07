# 云端/本地物联网平台同步与下一阶段审计（2026-09-07）

## 结论摘要

- `active/aetherlink-iot` 的 `main` 与 `origin/main` 同步：`c0d6fd448c0070d0cc1d41763d48e6fac2082bef`，工作树干净，`git rev-list --left-right --count HEAD...origin/main = 0 0`。
- `research/repos/thingsboard` 的 `master` 与 GitHub 同步：`d96082970bc3737dc7cccdb9d1078f0f89ea6ac9`。
- `research/repos/thingspanel-frontend-community` 的 `master` 与 GitHub 同步：`12131665e6192d5722a8cc19b159f214bd0ab306`。
- `research/repos/ThingsPanel-Go` 不同步：本地 `f9029efe3e9e8b35d70fdec1eb4e49ae5bef3618`，远端 `main` 为 `b64606102d823c28265c470ed3d275f1758b0337`，远端领先 1 提交、7 个文件。差异是版本注入/发布工作流/Docker 构建门禁，不是业务功能；已在临时浅克隆验证远端源码可下载且干净。

## 公开平台核对

ThingsBoard 官网/GitHub公开能力包括设备与资产/客户实体关系、MQTT/CoAP/HTTP、多租户、遥测与实时看板、SCADA、Rule Engine、告警和通知、边缘/网关生态、水平扩展与容错。依据：<https://thingsboard.io/>、<https://github.com/thingsboard/thingsboard>、<https://github.com/thingsboard/thingsboard-gateway>。

ThingsPanel 官网/GitHub公开能力包括 MQTT/Modbus/HTTP 等协议、设备模板/批量注册、网关与子设备、看板/3D/SCADA、OTA、场景自动化、告警通知、多租户 RBAC、插件/规则引擎、OpenAPI/数据网关和移动端。依据：<https://thingspanel.io/>、<https://github.com/ThingsPanel/ThingsPanel-Go>、<https://github.com/ThingsPanel/thingspanel-frontend-community>。

## 本地剩余缺口

以下均以当前源码或验证文档为准，页面/README 存在不等于端到端完成：

1. 真实发布门禁仍需重跑：目标服务器部署、HTTPS/TLS、公网 MQTT、backup/restore、ThingsVis 外部集成、real-RDI。
2. 设备影子离线命令的真实 broker ACK E2E、CSV 预注册浏览器上传/导出 E2E 尚未闭环。
3. 已发现的明确 TODO/未实现路径：Flow 停止、OTA 进度消费、场景执行/过期时间、模板市场升级；另有 HTTP `warn_status` 预留字段与少数生成 gRPC 空实现。
4. 产品化尾项：模板市场浏览/按行业打包下载、移动端小程序/APP 发布、边缘模板运维；RBAC 还需按产品授权逐角色收紧。
5. 质量债：剩余 DAL 零测目录、约 231 个前端列表空态、约 734 行 CJK/i18n、全面 aria 巡检、性能 benchmark 与部分大文件拆分。

## 是否继续

建议继续，但采用“证据门禁 + 垂直切片”推进：先同步并验证 ThingsPanel-Go 发布提交，再补真实 E2E/部署证据和明确 TODO，之后才扩展竞品差异能力。未通过真实栈门禁时，不把静态测试或历史报告表述为发布就绪。
