# 验证证据归档

本目录保存本地验证归档，例如覆盖率、API 自动化、Playwright E2E、后端 Go 测试、预览验证和发布前检查结果。

## 目录定位

- 按时间戳或主题保存一次验证会话的输出。
- 将历史证据与当前发布政策分离，避免旧报告覆盖当前结论。
- 只说明“当时命令产生了这些文件”，不自动说明“当前代码仍然通过”。

## 文件用途

以下名称是归档约定；对应目录可能不存在，不代表当前工作树已产生该类证据。

- `automation-run-*`：通常是自动化运行器归档。
- `preview-*`、`device-e2e-*`、`system-focused-*`、`write-flows-*`：通常是预览或聚焦 E2E/API 结果。
- `backend-go-coverage-*`、`frontend-coverage-*`：通常是后端或前端覆盖率输出。
- `post-cleanup-verification-*`、`unified-verification-*`：通常是清理后或统一验证归档。

## 验证命令

从仓库根目录执行：

```powershell
rg --files verification
Get-ChildItem verification\templates\*.json |
  ForEach-Object { Get-Content -Raw $_ | ConvertFrom-Json > $null }
```

本次目录标准化不运行 broad E2E、后端服务、前端服务或全量覆盖率套件。

## 审查发现

- 历史归档的时效性取决于命令、退出码、目标环境、账号数据和当时提交状态。
- 旧 JSON/HTML/日志不能被改写为新结论，也不能在缺少 manifest 时补造结果。
- 覆盖率快照目录可能很深，属于生成证据，不适合逐层维护 README。

## 重构/清理建议

- 新归档应包含命令、退出码、环境目标、关键产物、阻塞项和是否可公开的说明。
- 若归档缺少上下文，只能标为“历史证据，来源需复核”，不要升级为发布通过。
- 清理大体积覆盖率或 HTML 产物前，先确认是否已有等价、更新、可复现的归档。

## 2026-08-12 synthetic-rdi fresh continuation

当前 fresh 隔离软件证据：[`synthetic-rdi-20260812-fresh`](synthetic-rdi-20260812-fresh/)。manifest 标记为 `synthetic-rdi` / `protocol-emulator`，`claim_scope=isolated-software-path-only`，`real_rdi_status=not-tested`，`production_signoff=not-ready`。fresh fixture 从 `inactive/disabled` 开始，经公开 `POST /api/v1/rdi/devices/activate` 返回 `200` 并记录 `activated-this-run`，随后完成 success/failure ACK、`offline -> online -> offline`、fresh telemetry、SQL 回读、脱敏扫描和 34/34 share/link 软件合同；旧的 `final-6` 仍保留为历史同类证据，不与本轮 fresh 结果拼接。

本轮 fresh PID 为 `SYN260812229`，数据库回读为 `active/enabled`、`is_online=0`、`temperature_1=25.5`；原始激活和回读分别见 [`raw/synthetic-activation.json`](synthetic-rdi-20260812-fresh/raw/synthetic-activation.json) 与 [`raw/db-readback.json`](synthetic-rdi-20260812-fresh/raw/db-readback.json)。share/link 原始报告见 [`share-link-api/reports/02_device-report.json`](synthetic-rdi-20260812-fresh/share-link-api/reports/02_device-report.json)，包含跨租户 tenant ID 不同、首次/重复接受、`shared-with-me`、共享用户只读限制、写入/再次 share 拒绝和无效 token。Node contract `8 passed`、Go emulator test 通过、55/55 文件哈希一致，敏感信息扫描命中均为 `0`。数据库密码只作为运行时参数使用，未写入任何测试文档或证据。

边界不变：模拟 PID、voucher、硬件身份、固件 MQTT session、遥测、在线状态和 ACK 只证明隔离本地软件路径，不是实体 RDI 证明。真实 RDI PID/activation、真实 voucher/硬件身份、真实固件 MQTT session、真实物理遥测/在线状态/ACK、真实设备 RDI share/link 与生产跨租户权限链、ThingsVis/negative-menu、HTTPS、公网 MQTT、Docker/Compose target 和目标环境 backup/restore 仍需单独验收，不能从该归档晋升为部署通过。
