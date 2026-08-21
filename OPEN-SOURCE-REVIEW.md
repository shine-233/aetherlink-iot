# Open-Source Review Package

本文件是当前公开源码交给人工开源 review 的入口。它描述公开源码快照的边界和已知限制；源码已经上传到 GitHub，但这不等同于生产部署签字。

## 本轮状态

- review scope: 当前工作树中的源码、迁移、部署合同、测试代码、模拟器源码和状态文档。
- source package boundary: `public-source`。
- GitHub repository: `https://github.com/shine-233/aetherlink-iot`。
- GitHub upload: `executed`；公开 `main` 分支已包含当前源码基线。
- hosted source evidence: Source CI、Minimum quality gate 和 CodeQL 的 GitHub Actions / Go / JavaScript/TypeScript 分析已在当前公开基线上成功；这些结果只证明对应 workflow 覆盖的源码/离线门禁。
- production signoff: `not-ready`。
- real RDI: `not-tested`；synthetic/emulator 证据不升级为真实设备证据。
- target deployment: `pending`；Docker/Compose、HTTPS/TLS、反向代理、防火墙、公网 MQTT 和目标机灾备仍需目标环境验收。

本文件不把本机已有依赖树或历史报告当作当前发布证据。公开仓库的源码门禁由 GitHub Actions 重新执行；真实服务、浏览器、数据库、设备和目标环境验收仍按 `VALIDATION.md` 的证据等级单独记录。

## 保留内容

- `frontend/`、`backend/`、`mqtt-broker/` 的源码、必要生成源码和配置模板。
- `backend/sql/1.sql` 至 `backend/sql/48.sql` 以及迁移说明。
- `automation_tests/` 的测试、契约、API/E2E runner 和 synthetic RDI 协议模拟器源码。
- `deploy/`、Compose、反向代理和备份/恢复脚本合同。
- Native visualization provider，以及作为 optional compatibility provider 保留的 ThingsVis 源码、配置和合同。
- `README.md`、`START-HERE.md`、`PUBLICATION.md`、`VALIDATION.md`、`DEPLOYMENT-PREFLIGHT-METHOD.md`、`COMPATIBILITY.md`、`GENERATED_FILES.md`、`SECURITY.md` 和公开模板。
- `verification/templates/`、`audit_reports/README.md`、`_localrun_instance_b/README.md` 和 `instance-b.env.example` 等公开说明/模板。

生成文件的保留或排除依据见 `GENERATED_FILES.md`；不把生成的二进制、运行报告或本地凭据当作源码输入。

## 排除内容

公开快照不包含以下内容：

- `.env`、真实密码、token、私钥、证书、私有运行配置和本地数据库文件。
- `node_modules/`、Go/npm 缓存、构建目录、`dist/`、coverage、增量编译状态和本地二进制。
- Playwright auth/trace/screenshot、`reports/`、`test-results/`、日志和一次性运行目录。
- `verification/` 中的历史证据归档、压缩包和本地报告；只保留公开模板。
- `audit_reports/` 中的本地审计历史；只保留目录说明。
- `.git/` 及 IDE、Codex、Claude 和其他本地状态。

这些边界由根 `.gitignore` 和各模块的忽略规则共同执行。`_localrun_instance_b/` 只允许公开 README 和环境模板进入快照。

## 人工 review 顺序

1. 先阅读 `README.md`、`START-HERE.md`、`PUBLICATION.md` 和 `VALIDATION.md`，确认“源码可 review”与“生产已验收”是两个不同结论。
2. 检查 `git diff --cached --name-status`，确认新增、修改和删除都属于本轮预期范围；当前已知删除包括旧 route smoke、RSA key 目录说明和 `frontend/src/core/SystemInitializer.ts`。
3. 检查 `.env.example`、Compose、部署脚本和 `SECURITY.md`，确认真实凭据只通过外部环境注入。
4. 阅读 `DEPLOYMENT-PREFLIGHT-METHOD.md` 的证据分类，尤其区分 `local-verifiable`、`synthetic-only` 和 `real-external-only`。
5. 按 `GENERATED_FILES.md` 检查生成源码的来源和再生成风险。
6. 在独立 checkout 中按项目文档重新安装依赖、配置环境并执行构建/测试；不要把本机已有的 node_modules、数据库或历史报告复制进 review 结论。

## 已知审查事项

- 先前干净环境的 frontend Vite build 曾因缺少 `VITE_ICON_PREFIX` / `VITE_ICON_LOCAL_PREFIX` 默认值在插件初始化阶段失败；该问题属于构建环境/源码默认值审查项，本轮打包没有宣称修复。
- 已有 coverage、API/E2E、页面和 synthetic RDI 结果必须按验证文档中的批次、证据等级和 skip 原因阅读；它们不能关闭真实 RDI、真实设备 ACK、ThingsVis 外部服务或目标环境门禁。
- Native visualization 是默认本地实现。ThingsVis 仍是显式 optional compatibility provider，不能因为本地 Native provider 可用就从源码中删除。
- 公开仓库仍是供人工逐文件审查的源码基线，不是经过生产验收或作者签名的 release；未处理的 CodeQL 和 Dependabot 风险不能通过发布文档被默认为已关闭。

## 复核命令

在仓库根目录执行：

```text
git status --short --branch
git diff --cached --stat
git diff --cached --check
git ls-files --cached
git check-ignore -v .env frontend/node_modules frontend/dist automation_tests/reports
```

外部 review package 的文件清单、逐文件 SHA-256、总大小、Git tree id 和 ZIP SHA-256 由本轮生成的同名外部 manifest 保存。manifest 不记录任何密码或 token；如果 package 与工作树再次变化，应废弃旧 manifest 并重新生成。

## 结论

当前项目已完成 GitHub 公开源码上传（见上文 `GitHub upload: executed`），并接入 Source CI、Minimum quality gate 与 CodeQL；但这仍不等于"已部署""生产签字完成"。人工 review 通过后，后续发布仍需由授权者决定 tag、容器发布、许可证/版权确认以及验证证据更新。
