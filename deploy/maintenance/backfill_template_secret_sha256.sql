-- 用途：一次性把 device_configs.template_secret 的存量明文行回填为 SHA-256 摘要，
--       与后端 dal/device_auth.go 的"摘要落库 + 双读兼容 + 登录惰性升级"方案对齐。
-- 前提：
--   1. 目标库为 PostgreSQL 且允许安装 pgcrypto 扩展（多数托管 PG 允许；
--      若不允许，改用应用层遍历脚本，语义一致）。
--   2. 在低峰期或暂停设备配置写入后执行，避免与登录惰性升级并发竞争同一行。
--   3. 回填不可逆：执行后设备端必须仍持有明文 secret 才能通过鉴权（哈希不可逆）。
--      如有设备丢失明文，先补发密钥再回填。
-- 幂等性：WHERE 条件跳过已是 64 位 hex 摘要的行，重复执行无副作用。
-- 验证：执行后用下方 SELECT 抽查，剩余明文行数应为 0。

CREATE EXTENSION IF NOT EXISTS pgcrypto;

UPDATE device_configs
SET template_secret = encode(digest(template_secret, 'sha256'), 'hex'),
    updated_at = updated_at  -- 保持原更新时间不变（可选：去掉该行让 gorm/触发器语义接管）
WHERE template_secret IS NOT NULL
  AND template_secret <> ''
  AND template_secret !~ '^[0-9a-f]{64}$';

-- 回填结果抽查：应返回 0。
SELECT COUNT(*) AS remaining_plaintext_rows
FROM device_configs
WHERE template_secret IS NOT NULL
  AND template_secret <> ''
  AND template_secret !~ '^[0-9a-f]{64}$';
