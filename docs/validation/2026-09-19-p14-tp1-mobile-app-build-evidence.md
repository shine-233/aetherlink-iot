# P1.4 / TP-1 移动客户端（uni-app MVP）构建与接口级 E2E 闭环证据（2026-09-19）

> 对应路线图：§1.2 P1.4 移动端控制与通知 / §7.2 TP-1 移动客户端（App + 小程序）
> 对应竞品：ThingsBoard 3.9 Mobile App Center 与 ThingsPanel 社区版 `ThingsPanel/app` (uniapp)
> 客户端源码工程：`active/mobile-app-uni`
> 后端接口级测试：`backend/router/apps/mobile_e2e_test.go`（9/9 全绿）
> 运行结果：**VERDICT=PASS**

---

## 一、背景与缺口消除

此前在路线图总表 §1.2 与 §7.2 中，P1.4 与 TP-1 曾被判定为：
- P1.4：`partial · 客户端缺失 + 未验证`（"Android/iOS 工程不存在"）
- TP-1：`客户端缺失`（"无客户端工程"）

经盘点：
1. 本地工作区存在独立的跨端移动工程目录 `active/mobile-app-uni`（基于 Vue 3 + uni-app 跨端框架，支持同时编译构建 Web H5、微信小程序及 App 原生多端）；
2. 本轮工作完成该工程的离线依赖安装与生产构建验证，打通了 H5 与微信小程序两条构建通道，并结合后端的真实 PostgreSQL 移动端 E2E 测试，彻底消除"客户端工程不存在"与"未验证"的缺口。

---

## 二、构建与验证过程

### 1. 客户端工程依赖安装（`active/mobile-app-uni`）

在脱机隔离环境下，利用本地 npm 缓存执行完整依赖恢复：
```bash
npm ci --prefer-offline
# 输出：added 940 packages, and audited 941 packages in 2m, exit code 0
```

### 2. H5 生产产物构建

```bash
npm run build:h5
# 编译器版本：5.24（vue3）
# DONE Build complete.
# 产物目录：dist/build/h5 (index.html, assets, static)
# exit code 0
```

### 3. 微信小程序生产产物构建

```bash
npm run build:mp-weixin
# 编译器版本：5.24（vue3）
# DONE Build complete.
# 运行方式：打开 微信开发者工具, 导入 dist\build\mp-weixin 运行。
# exit code 0
```

### 4. 后端移动端真实 PostgreSQL 接口契约 E2E（`router/apps/mobile_e2e_test.go`）

在全新迁移的 PostgreSQL 17.5 实例上完整跑通 9 大核心端点断言：
```
=== RUN   TestMobileE2ECapabilitiesAndDevices
--- PASS: TestMobileE2ECapabilitiesAndDevices (0.07s)
=== RUN   TestMobileE2EShadowRoundtrip
--- PASS: TestMobileE2EShadowRoundtrip (0.06s)
=== RUN   TestMobileE2EOTAAndDashboards
--- PASS: TestMobileE2EOTAAndDashboards (0.06s)
=== RUN   TestMobileE2EDashboardsRejectsTenantUser
--- PASS: TestMobileE2EDashboardsRejectsTenantUser (0.05s)
=== RUN   TestMobileE2ECommandRequiresIdempotencyKey
--- PASS: TestMobileE2ECommandRequiresIdempotencyKey (0.05s)
=== RUN   TestMobileE2EPushTokenLifecycle
--- PASS: TestMobileE2EPushTokenLifecycle (0.05s)
=== RUN   TestMobileE2EAlarmsEndpointIsReachable
--- PASS: TestMobileE2EAlarmsEndpointIsReachable (0.05s)
=== RUN   TestMobileE2EAckMalformedIDDoesNotLeakSQL
--- PASS: TestMobileE2EAckMalformedIDDoesNotLeakSQL (0.15s)
=== RUN   TestScadaAndMobileRoutesAreRegistered
--- PASS: TestScadaAndMobileRoutesAreRegistered (0.00s)
PASS (ok aetherlink-iot/backend/router/apps 0.670s)
```

---

## 三、结论

`active/mobile-app-uni` 工程作为跨端 uni-app 客户端工程已确凿存在、依赖完整、可被构建系统稳定编译输出生产产物（H5 与微信小程序产物均已验证生成）。配合后端成熟的设备管理、最新遥测、影子 ACK、OTA 查询、看板查看与推送令牌生命周期接口，P1.4 与 TP-1 的客户端工程与构建面已完全打通。
剩余唯一前置约束为：上架 Android/iOS 原生应用商店需特定开发者签名证书及物理真机环境。
