-- 97.sql — P1.5 边缘节点注册表 + P1.5/P1.6 新路由的 Casbin 登记。
--
-- 背景（先把缺口说清楚）：
-- 1) P1.5 此前只有纯决策函数（健康判定/版本兼容/冲突检测/Reconcile 编排），
--    全库没有 edge_nodes 表——注册请求无处落地，健康判定没有数据来源，
--    "节点离线产生告警""重连后按版本同步"两个门禁都无从谈起。
-- 2) P1.6 的 VerifyMarketBundle / CheckMarketBundleDependencies / PreviewMarketBundleImport
--    三个完整性函数此前零调用方——没有导入端点，验签/依赖/预览链路没有入口。
--    本迁移同时登记两条新链路的路由资源，防止 casbin.route-audit-mode（默认
--    fail-fast）在启动期拒绝启动（与 81/90/94/95/96.sql 同源）。

-- ---- P1.5: 边缘节点注册表 ----
-- 设计要点：
--   - id 是节点自报身份（边缘代理生成的稳定标识），不是平台签发 uuid：
--     注册语义是"登记一个已存在的边缘身份"，重注册不得换 ID。
--   - 跨租户抢注由应用层拒绝（先全局读 ID 再判定），表层面用 tenant_id 索引
--     支撑租户内列表；不做 (id) 全局唯一以外的额外约束——id 本就是主键。
--   - capabilities 存 JSON 数组文本，规范化（去空白/去重/排序）在应用层完成。
--   - status 预留 revoked：心跳与注册更新都以 status='active' 为条件，
--     吊销节点的心跳不得续命。
--   - last_seen_at 可空：从未上报过心跳的节点健康判定为 unknown（不得乐观当在线）。
CREATE TABLE IF NOT EXISTS public.edge_nodes (
    id             varchar(64)  PRIMARY KEY,
    tenant_id      varchar(36)  NOT NULL,
    version        varchar(32)  NOT NULL,
    capabilities   text         NOT NULL DEFAULT '[]',
    status         varchar(16)  NOT NULL DEFAULT 'active',
    last_seen_at   timestamptz  NULL,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT edge_nodes_status_check
        CHECK (status IN ('active', 'revoked')),
    CONSTRAINT edge_nodes_tenant_check
        CHECK (tenant_id <> '')
);

-- 租户内列表按最后心跳倒序（运维视角先看最久没响的）。
CREATE INDEX IF NOT EXISTS edge_nodes_tenant_seen_idx
    ON public.edge_nodes (tenant_id, last_seen_at DESC);

-- ---- 新路由的 Casbin 资源与授权 ----
-- P1.5 边缘节点四条 + P1.6 打包导入一条。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/edge/nodes'),
  ('api/v1/edge/nodes/:node_id/heartbeat'),
  ('api/v1/edge/nodes/:node_id/reconcile'),
  ('api/v1/device/template/market/bundle/import')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 授权口径与 81.sql 的 edge 组一致：SYS_ADMIN + TENANT_ADMIN。
-- 节点注册/Reconcile 决定下发给边缘的内容，打包导入决定租户物模型的来源，
-- 都不允许 TENANT_USER 直接触发。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/edge/nodes'),
  ('TENANT_ADMIN', 'api/v1/edge/nodes'),
  ('SYS_ADMIN',    'api/v1/edge/nodes/:node_id/heartbeat'),
  ('TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/heartbeat'),
  ('SYS_ADMIN',    'api/v1/edge/nodes/:node_id/reconcile'),
  ('TENANT_ADMIN', 'api/v1/edge/nodes/:node_id/reconcile'),
  ('SYS_ADMIN',    'api/v1/device/template/market/bundle/import'),
  ('TENANT_ADMIN', 'api/v1/device/template/market/bundle/import')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- P2.1: 插件 manifest 持久化 ----
-- PluginRegistryService.Create 在提供 manifest 时按 pkg/pluginsdk 校验（不过即拒绝），
-- 原始 JSON 落列便于审计与展示。可空：保持 D9 既有注册路径兼容。
ALTER TABLE public.plugin_registries ADD COLUMN IF NOT EXISTS manifest text NULL;

-- ---- P2.2: 基础异常检测路由 ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/telemetry/analysis/anomaly')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/telemetry/analysis/anomaly'),
  ('TENANT_ADMIN', 'api/v1/telemetry/analysis/anomaly'),
  ('TENANT_USER',  'api/v1/telemetry/analysis/anomaly')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- P2.3: 遥测降采样冷层 ----
-- 原始数据保留策略（datapolicy，每日 2 点）照常工作；本表是"删掉之前先汇总"的
-- 冷层。降采样作业（每日 3 点，telemetry.downsample.enabled 门控默认关闭）把
-- cutoff 之前的原始行按 1h 桶汇总进来；分析查询对整窗冷数据回落本表。
-- 主键含 bucket_ms：未来引入多粒度（1h/1d）可共存。
CREATE TABLE IF NOT EXISTS public.telemetry_rollups (
    device_id     varchar(36)        NOT NULL,
    key           varchar(255)       NOT NULL,
    bucket_ms     bigint             NOT NULL,
    bucket_start  bigint             NOT NULL,
    min_v         double precision   NULL,
    max_v         double precision   NULL,
    avg_v         double precision   NULL,
    last_v        double precision   NULL,
    count_v       bigint             NOT NULL DEFAULT 0,
    updated_at    timestamptz        NOT NULL DEFAULT now(),
    CONSTRAINT telemetry_rollups_pk
        PRIMARY KEY (device_id, key, bucket_ms, bucket_start)
);

-- 冷读路径按 (key, bucket_start) 范围扫描；主键已覆盖 (device_id, key, ...) 前缀。
CREATE INDEX IF NOT EXISTS telemetry_rollups_window_idx
    ON public.telemetry_rollups (device_id, key, bucket_start);

-- ---- P3: 许可证状态路由 ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/license/status')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 许可证状态属平台管理面，只授予 SYS_ADMIN（与 PluginRegistry 同口径）。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN', 'api/v1/license/status')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- P1.x: 看板项目分组（native-board-provider 项目增删改） ----
-- 设计要点：
--   - 刻意不动 boards 表（gen 生成模型）：分组放独立关联表，迁移独立演进。
--   - 一块看板同时只属于一个项目：member 表 (tenant_id, board_id) 唯一索引收口，
--     并发换项目只有一个赢家，不靠应用层先查。
--   - 内置项目（前端 NATIVE_BOARD_PROJECT_ID）不落库：无归属记录即属于它。
CREATE TABLE IF NOT EXISTS public.board_projects (
    id          varchar(36)  PRIMARY KEY,
    tenant_id   varchar(36)  NOT NULL,
    name        varchar(255) NOT NULL,
    description varchar(500) NULL,
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT board_projects_tenant_name_uk UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS public.board_project_members (
    project_id  varchar(36)  NOT NULL,
    board_id    varchar(36)  NOT NULL,
    tenant_id   varchar(36)  NOT NULL,
    created_at  timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT board_project_members_pk PRIMARY KEY (project_id, board_id)
);

-- 一板一项目：租户内 board_id 唯一。
CREATE UNIQUE INDEX IF NOT EXISTS board_project_members_board_uk
    ON public.board_project_members (tenant_id, board_id);

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/board/projects'),
  ('api/v1/board/projects/:id'),
  ('api/v1/board/projects/:id/boards/:board_id'),
  ('api/v1/board/projects/member-of/:board_id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 读（列表/详情/归属反查）授予三个角色；写（增删改/归属变更）只授予管理员。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/board/projects'),
  ('TENANT_ADMIN', 'api/v1/board/projects'),
  ('TENANT_USER',  'api/v1/board/projects'),
  ('SYS_ADMIN',    'api/v1/board/projects/member-of/:board_id'),
  ('TENANT_ADMIN', 'api/v1/board/projects/member-of/:board_id'),
  ('TENANT_USER',  'api/v1/board/projects/member-of/:board_id'),
  ('SYS_ADMIN',    'api/v1/board/projects/:id'),
  ('TENANT_ADMIN', 'api/v1/board/projects/:id'),
  ('TENANT_USER',  'api/v1/board/projects/:id'),
  ('SYS_ADMIN',    'api/v1/board/projects/:id/boards/:board_id'),
  ('TENANT_ADMIN', 'api/v1/board/projects/:id/boards/:board_id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
