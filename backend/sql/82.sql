-- 82.sql: Phase D7 AI 2.0——模型中心表 + casbin 受保护路由登记（2026-09-07）。
-- 功能：ai_models 租户级 AI 模型档案（OpenAI 兼容端点/模型名/密钥/用途/启用），
--       供助手（/ai/assistant/chat）与规则链 ai.inference 节点取用；api_key 仅落库不出参（出参脱敏）。
-- casbin：登记 3 条受保护路由（模型 CRUD 两路径 + 助手对话）。
--   授权口径：模型档案属租户级安全配置，仅 SYS_ADMIN + TENANT_ADMIN 可管理；
--   助手对话面向租户用户开放（含 TENANT_USER）。
-- 幂等性：CREATE TABLE/INDEX IF NOT EXISTS；casbin 行 INSERT 前置 NOT EXISTS 守卫。

CREATE TABLE IF NOT EXISTS ai_models (
    id         VARCHAR(36) PRIMARY KEY,
    tenant_id  VARCHAR(36) NOT NULL,
    name       VARCHAR(128) NOT NULL,
    provider   VARCHAR(32) NOT NULL DEFAULT 'openai',
    base_url   VARCHAR(512) NOT NULL,
    model      VARCHAR(128) NOT NULL,
    api_key    TEXT NOT NULL,
    purpose    VARCHAR(32) NOT NULL DEFAULT 'chat',
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_models_tenant ON ai_models (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ai_models_purpose ON ai_models (purpose);

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/ai/models'),
  ('api/v1/ai/models/:id'),
  ('api/v1/ai/assistant/chat')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/ai/models'),
  ('TENANT_ADMIN', 'api/v1/ai/models'),
  ('SYS_ADMIN',    'api/v1/ai/models/:id'),
  ('TENANT_ADMIN', 'api/v1/ai/models/:id'),
  ('SYS_ADMIN',    'api/v1/ai/assistant/chat'),
  ('TENANT_ADMIN', 'api/v1/ai/assistant/chat'),
  ('TENANT_USER',  'api/v1/ai/assistant/chat')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
