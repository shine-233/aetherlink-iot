# P0.7「日志无明文」活栈取证（AI 凭证静态加密）

> 日期：2026-09-18 深夜
> 对应路线图：§1.2 P0.7（AI 凭证静态加密）未闭环项「生产主密钥注入、'日志无明文'未验证」
> 结论：**"日志无明文"已在真实活栈取证通过**；P0.7 剩余缺口收窄为唯一一项
> ——生产环境的主密钥注入方式（属部署环境验收，与 P0.1 的目标服务器项同类）。

## 一、方法：金丝雀注入 + 三面扫描

主密钥沿用本地配置既有条目（`secrets.active_key_id=k1`，conf-localdev.yml，
gitignore 不入库）。重启后端后，通过真实 API 创建一条带**特征明文**的 AI 凭证：

- 端点：`POST /api/v1/ai/models`（`base_url=https://api.openai.com/v1`，
  需通过 safe-egress 的公网 HTTPS 校验——`127.0.0.1` 端点会被 SSRF 防御拒绝，
  顺带验证了该防线）
- 明文金丝雀：`P07-CANARY-9f3a7c1e-plaintext-must-never-appear`

## 二、三面实测结果

| 门禁面 | 实测 | 结果 |
| --- | --- | --- |
| 数据库 | `SELECT api_key FROM ai_models WHERE id=…` → **`aenv1.k1.Y0RcoYmWskfSBs6Rca7…`**（AES-256-GCM 信封密文，AAD 绑定租户） | ✅ 库内无明文 |
| API 响应 | `GET /ai/models/:id` → `api_key_masked = "P07-****"`（前 4 位 + 掩码，全响应无金丝雀） | ✅ 出参只出不可逆掩码 |
| 后端日志 | 重启前后两份完整 stdout 日志各 grep 金丝雀明文 → **0 命中 / 0 命中** | ✅ 日志无明文 |

补充确认：

- 创建时若 `base_url` 非公网 HTTPS（`http://127.0.0.1:9/v1`）→ `100002 AI provider
  configuration is invalid`，SSRF 防御在真实活栈上成立；
- fail-closed（无主密钥/非法密钥拒绝写入）、自愈式轮换重写、跨租户 AAD 认证失败
  已由既有测试覆盖：`pkg/secrets/envelope_test.go` 8 例、
  `internal/service/ai_model_secret_test.go` 5 例、
  `ai_model_secret_postgres_test.go`（直接 `SELECT api_key` 断言库内无明文）。

## 三、清理

金丝雀模型档案已通过 `DELETE /ai/models/:id` 回收（200）；本地配置中的主密钥
保持原样（09-17 会话所置，仅本地隔离库使用）。

## 四、仍未验证（如实）

- **生产主密钥注入**：生产环境从环境变量/密管系统注入主密钥并完成一次真实的
  创建→读回→轮换，只能在真实部署环境做（与 P0.1 的 HTTPS/公网 MQTT 同类，
  属 P0.1 部署门禁的组成部分）。
