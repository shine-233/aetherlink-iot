-- P1.3 Widget 与 SCADA 基础层 + P1.4 移动端推送基础。
-- 设计要点（与本项目既有约定一致）：
--   1. 租户隔离一律由 tenant_id 承担，且**包含在唯一约束内**，跨租户同名不会互相覆盖。
--   2. 能由数据库挡住的语义就用 CHECK 挡住，不依赖服务层"记得校验"。
--   3. 凡是"没发生"与"发生了但是零"会被混淆的地方，一律允许 NULL 而不给默认值 0。

-- ---------------------------------------------------------------------------
-- scada_projects：多项目容器（P1.3 门禁"项目 CRUD 不再返回 unsupported"）。
-- 此前看板只有扁平的 boards / vis_dashboard，没有"项目"这一层，
-- 多项目/多看板无从表达，项目 CRUD 只能返回 unsupported。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.scada_projects (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   varchar(36) NOT NULL,
    name        varchar(128) NOT NULL,
    description text NULL,
    created_by  varchar(36) NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scada_projects_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_scada_projects_tenant
    ON public.scada_projects (tenant_id);

COMMENT ON COLUMN public.scada_projects.tenant_id IS
    'Tenant owner; part of the unique key so identical project names in different tenants coexist.';

-- ---------------------------------------------------------------------------
-- scada_documents：画布文档（草稿/发布/归档三态）。
--   - current_version 为草稿版本号，每次保存 +1（乐观并发凭证）。
--   - published_version 为 NULL 表示"从未发布过"；这与"发布了第 0 版"是两种事实，
--     故刻意可空，不默认 0。
--   - json_data 承载画布（widget 布局/数据源绑定/变量），保存即整体覆盖。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.scada_documents (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         varchar(36) NOT NULL,
    project_id        uuid NOT NULL REFERENCES public.scada_projects (id) ON DELETE CASCADE,
    name              varchar(128) NOT NULL,
    status            varchar(16) NOT NULL DEFAULT 'DRAFT',
    current_version   int4 NOT NULL DEFAULT 1,
    published_version int4 NULL,
    json_data         jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by        varchar(36) NULL,
    updated_by        varchar(36) NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scada_documents_status_check
        CHECK (status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED')),
    CONSTRAINT scada_documents_version_positive
        CHECK (current_version > 0),
    -- 已发布版本若非空，不得超前于草稿版本：否则"已发布"的内容比草稿还新，
    -- 回滚/对比会指向一个不存在的版本。
    CONSTRAINT scada_documents_published_not_ahead
        CHECK (published_version IS NULL OR published_version <= current_version),
    CONSTRAINT scada_documents_project_name_unique UNIQUE (tenant_id, project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_scada_documents_project
    ON public.scada_documents (tenant_id, project_id);

COMMENT ON COLUMN public.scada_documents.published_version IS
    'Version currently published; NULL means never published, which is not the same as version 0.';
COMMENT ON COLUMN public.scada_documents.current_version IS
    'Draft version, incremented on every save; used as the optimistic-concurrency token.';

-- ---------------------------------------------------------------------------
-- scada_document_versions：发布版本快照（不可变，供回滚/对比）。
-- 只存"发布过"的版本；草稿保存不产生快照，否则版本表会被保存噪声淹没，
-- 回滚列表里出现一堆没人发布过的版本。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.scada_document_versions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    varchar(36) NOT NULL,
    document_id  uuid NOT NULL REFERENCES public.scada_documents (id) ON DELETE CASCADE,
    version      int4 NOT NULL,
    json_data    jsonb NOT NULL,
    published_by varchar(36) NULL,
    published_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scada_document_versions_unique UNIQUE (tenant_id, document_id, version),
    CONSTRAINT scada_document_versions_version_positive CHECK (version > 0)
);

CREATE INDEX IF NOT EXISTS idx_scada_document_versions_doc
    ON public.scada_document_versions (tenant_id, document_id, version DESC);

-- ---------------------------------------------------------------------------
-- scada_control_audits：实时控制命令审计（P1.3 门禁"控制命令有权限、确认和审计"）。
-- 关键：**被拒绝的命令也要落审计**（outcome='denied'）。只审计成功命令等于
-- 把"谁在反复尝试越权控制"这件事从记录里抹掉，审计就只剩装饰作用。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.scada_control_audits (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          varchar(36) NOT NULL,
    document_id        uuid NOT NULL,
    widget_id          varchar(64) NOT NULL,
    command            varchar(64) NOT NULL,
    params             jsonb NOT NULL DEFAULT '{}'::jsonb,
    actor_user_id      varchar(36) NOT NULL,
    confirmation_token varchar(64) NOT NULL,
    outcome            varchar(16) NOT NULL,
    detail             text NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    -- pending 是"已过闸、准备下发"的中间态：审计必须先于执行落库，
    -- 才能做到"审计写不进去就拒绝执行"。没有它，审计只能事后补记，
    -- 写失败时命令已经下发，等于控制动作可以没有记录地发生。
    CONSTRAINT scada_control_audits_outcome_check
        CHECK (outcome IN ('pending', 'success', 'denied', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_scada_control_audits_doc
    ON public.scada_control_audits (tenant_id, document_id, created_at DESC);

COMMENT ON COLUMN public.scada_control_audits.outcome IS
    'success | denied | failed; denials are recorded too, otherwise rejected attempts leave no trace.';

-- ---------------------------------------------------------------------------
-- push_device_registrations：移动端推送令牌登记（P1.4）。
-- 同一 (租户, 用户, 平台, 令牌) 唯一，重复登记幂等更新而非堆积重复行，
-- 否则同一次推送会被放大成 N 条。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.push_device_registrations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  varchar(36) NOT NULL,
    user_id    varchar(36) NOT NULL,
    platform   varchar(16) NOT NULL,
    token      varchar(512) NOT NULL,
    provider   varchar(32) NOT NULL DEFAULT 'fcm',
    enabled    boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT push_device_registrations_platform_check
        CHECK (platform IN ('ios', 'android', 'h5')),
    CONSTRAINT push_device_registrations_unique
        UNIQUE (tenant_id, user_id, platform, token)
);

CREATE INDEX IF NOT EXISTS idx_push_device_registrations_user
    ON public.push_device_registrations (tenant_id, user_id, enabled);

-- ---------------------------------------------------------------------------
-- push_deliveries：推送投递与重试审计（P1.4 门禁"推送失败可重试并可审计"）。
--   - attempt_count 从 0 起；pending/failed 持有 next_attempt_at，dead 为终态。
--   - 终态 dead 与"还能重试的 failed"必须分开表达，否则重试器会无限捞起
--     早已放弃的投递，把失败伪装成"还在路上"。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.push_deliveries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       varchar(36) NOT NULL,
    registration_id uuid NULL REFERENCES public.push_device_registrations (id) ON DELETE SET NULL,
    user_id         varchar(36) NOT NULL,
    title           varchar(255) NOT NULL,
    body            text NOT NULL,
    data            jsonb NOT NULL DEFAULT '{}'::jsonb,
    status          varchar(16) NOT NULL DEFAULT 'pending',
    attempt_count   int4 NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NULL,
    last_error      text NULL,
    provider        varchar(32) NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT push_deliveries_status_check
        CHECK (status IN ('pending', 'sent', 'failed', 'dead')),
    CONSTRAINT push_deliveries_attempt_count_non_negative
        CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_push_deliveries_retry
    ON public.push_deliveries (status, next_attempt_at)
    WHERE status IN ('pending', 'failed');

COMMENT ON COLUMN public.push_deliveries.status IS
    'pending | sent | failed (retryable) | dead (terminal, no further retry).';
