# 安全策略

AetherLink IoT 会接触本地应用账号、设备凭据、MQTT 运行凭据、API token 和自动化验证产物。所有密钥、口令和本地认证状态都应留在源码仓库之外。

## 漏洞报告

当前阶段请先私下联系项目维护者，再决定是否创建公开 issue。报告中建议包含受影响组件、复现步骤、预期影响和必要日志；提交日志前必须移除 token、密码、证书、私钥、账号口令和可识别的本地环境信息。

## 本地密钥与敏感文件

- 不要提交 `.env`、`.local`、Playwright auth state、运行时证书、私钥、数据库 dump 或自动化生成的账号文件。
- 凭据应通过环境变量、被忽略的本地文件或 CI secrets 注入。
- 发布快照前请复核 `PUBLICATION.md`、`VALIDATION.md` 和 `.gitignore`，确认本地材料没有进入 Git。
- 当前仓库已识别出的高优先级本地敏感项包括 `backend/configs/rsa_key/private_key.pem`、`backend/configs/conf-localdev.yml`、前端 `.env*` 文件和 `backend/cmd/aetherlink-device-autotest/docs/` 下的环境资料；这些都不应进入公开仓库。

## 高敏区域

- 后端认证、授权、租户隔离检查和设备生命周期服务。
- MQTT broker 认证、ACL、topic 映射、root/plugin 凭据和内部 MQTT 转发。
- 前端 token/session 处理，以及 ThingsVis 集成状态。
- 自动化 preflight、种子账号和归档报告。

## 维护与审查建议

- 涉及认证、授权、租户、设备删除、MQTT 凭据或测试账号的改动，应要求更窄但更强的回归验证。
- 审查公开提交前，建议运行一次敏感后缀和敏感关键词扫描，并人工检查新增配置文件。
- 如果发现历史报告中含有真实凭据，不要只删除当前文件；还应评估 Git 历史、归档目录和外部分发包是否需要清理。
- 对已经被 `.gitignore` 明确隔离的本地边界文件，优先保持“忽略 + 私有保留 + 发布前复核”的策略；只有纯运行态或可稳定再生文件才适合直接从工作树清理。
