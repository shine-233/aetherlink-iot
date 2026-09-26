-- 136.sql — TB-23 移动应用中心（第一批：bundle/版本/发布管理后端面）
--
-- 背景：
--   1. 对标 ThingsBoard PE Mobile App Center：平台已有移动端 API 面（api/mobile.go，
--      能力矩阵/推送登记/幂等命令）与推送存储（88.sql push_device_registrations），
--      但移动应用安装包本体（apk/ipa/h5 构建产物）无任何 bundle/版本/发布管理——
--      无处上传、无处看版本、无处控制"哪个版本对终端可见"（ROADMAP TB-23）；
--   2. 本迁移建立 mobile_app_bundles：租户内应用包版本登记行。platform 枚举
--      android/ios/h5（CHECK 兜底，服务层为准）；status 枚举 draft/published/archived，
--      发布状态机由服务层强制（draft→published→archived，非法流转拒绝）；
--      UNIQUE(tenant_id, platform, version) 保证同租户同平台版本唯一（重复上传拒绝，
--      不同租户互不影响——版本号是租户内概念，不做全局唯一）；
--   3. 文件本体复用 ./files 上传存储根（pkg/common.BaseUploadDir），落 files/apps/
--      子目录；checksum 为文件 SHA-256 十六进制（上传时计算，入库后只读）。
--      跨仓边界（ROADMAP §3 第 10 行口径）：uniapp 工程 active/mobile-app-uni 的构建
--      产物对接与商店分发不在本期，本表只负责登记与状态机；下载接口按租户隔离出文件流；
--   4. Casbin 路由登记（g2）与角色授权（p）：仅 SYS_ADMIN / TENANT_ADMIN。
--      应用中心是管理面（上传/发布/归档），TENANT_USER fail-closed；终端拉取已发布包
--      的开放下载策略留给 uniapp 对接阶段单独评审，不在本迁移放行；
--      - POST   /api/v1/mobile/app_bundles/upload          上传并登记（multipart）
--      - GET    /api/v1/mobile/app_bundles                 分页列表（platform/status 过滤）
--      - GET    /api/v1/mobile/app_bundles/:id             详情
--      - PUT    /api/v1/mobile/app_bundles/:id             更新（仅 draft 可改 release_notes）
--      - DELETE /api/v1/mobile/app_bundles/:id             删除（仅 draft/archived；published 先归档）
--      - POST   /api/v1/mobile/app_bundles/:id/publish     发布（draft→published，落 published_at）
--      - POST   /api/v1/mobile/app_bundles/:id/archive     归档（published→archived）
--      - GET    /api/v1/mobile/app_bundles/:id/download    下载文件流（同租户内任意状态）
--   5. 管理菜单（sys_ui_elements）：/mobile-app/app-center 页面行挂新建 mobile-app 顶级目录。
--      前端四件套（下一阶段交付）必须与本种子对齐：路由名 mobile-app、子路由
--      mobile-app_app-center、组件 view.mobile-app_app-center、i18n 键
--      route.mobile-app / route.mobile-app_app-center、页面文案键 page.app_bundle.*。
--
-- 编号沿革：本迁移内容曾随 TB-23 首轮实施落在 133.sql（同内容），后按批次三
--   统一编号分配改挂 136 号位；133.sql 转为号位占位空迁移。建表与 Casbin/菜单
--   登记均带 IF NOT EXISTS / NOT EXISTS 幂等守卫，对已执行过旧 133.sql 的库
--   幂等，两条升级路径结果一致。

-- ---- 1. 移动应用包登记表 (mobile_app_bundles) ----
CREATE TABLE IF NOT EXISTS public.mobile_app_bundles (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    version VARCHAR(50) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    checksum VARCHAR(64) NOT NULL,
    release_notes TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_mobile_app_bundles_tenant_platform_version UNIQUE (tenant_id, platform, version),
    CONSTRAINT ck_mobile_app_bundles_platform CHECK (platform IN ('android', 'ios', 'h5')),
    CONSTRAINT ck_mobile_app_bundles_status CHECK (status IN ('draft', 'published', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_mobile_app_bundles_tenant_id ON public.mobile_app_bundles(tenant_id);
CREATE INDEX IF NOT EXISTS idx_mobile_app_bundles_tenant_status ON public.mobile_app_bundles(tenant_id, status);

COMMENT ON TABLE public.mobile_app_bundles IS '移动应用包版本登记（TB-23；UNIQUE(tenant_id,platform,version)，状态机 draft→published→archived）';
COMMENT ON COLUMN public.mobile_app_bundles.platform IS '目标平台：android/ios/h5（服务层按平台校验扩展名：apk/ipa/zip）';
COMMENT ON COLUMN public.mobile_app_bundles.file_path IS '文件对外访问路径（./files/apps/<platform>/<日期>/<哈希>.<ext>，磁盘位置同 BaseUploadDir 语义）';
COMMENT ON COLUMN public.mobile_app_bundles.checksum IS '文件 SHA-256 十六进制（上传时计算，入库后只读）';
COMMENT ON COLUMN public.mobile_app_bundles.status IS '发布状态机：draft（草稿，可改可删）→ published（已发布，不可改不可删）→ archived（已归档，可删）';
COMMENT ON COLUMN public.mobile_app_bundles.published_at IS '发布时间（draft→published 流转落 UTC 时刻；归档不清除，保留发布履历）';

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/mobile/app_bundles'),
  ('api/v1/mobile/app_bundles/upload'),
  ('api/v1/mobile/app_bundles/:id'),
  ('api/v1/mobile/app_bundles/:id/publish'),
  ('api/v1/mobile/app_bundles/:id/archive'),
  ('api/v1/mobile/app_bundles/:id/download')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p) ----
-- 仅 SYS_ADMIN / TENANT_ADMIN：应用中心是管理面，TENANT_USER fail-closed（口径同 131.sql）。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/mobile/app_bundles'),
  ('SYS_ADMIN',    'api/v1/mobile/app_bundles/upload'),
  ('SYS_ADMIN',    'api/v1/mobile/app_bundles/:id'),
  ('SYS_ADMIN',    'api/v1/mobile/app_bundles/:id/publish'),
  ('SYS_ADMIN',    'api/v1/mobile/app_bundles/:id/archive'),
  ('SYS_ADMIN',    'api/v1/mobile/app_bundles/:id/download'),
  ('TENANT_ADMIN', 'api/v1/mobile/app_bundles'),
  ('TENANT_ADMIN', 'api/v1/mobile/app_bundles/upload'),
  ('TENANT_ADMIN', 'api/v1/mobile/app_bundles/:id'),
  ('TENANT_ADMIN', 'api/v1/mobile/app_bundles/:id/publish'),
  ('TENANT_ADMIN', 'api/v1/mobile/app_bundles/:id/archive'),
  ('TENANT_ADMIN', 'api/v1/mobile/app_bundles/:id/download')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- 4. 管理菜单 (sys_ui_elements，新建 mobile-app 顶级目录，对标 media 目录行) ----
-- 授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic），页面路由
-- /mobile-app/app-center 由下一阶段前端四件套注册，菜单行先行就位（NOT EXISTS 守卫）。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'e5f6a7b8-9c01-4d23-8e45-6f7a8b9c0d11',
    '0',
    'mobile-app', 1, 119, '/mobile-app', 'mdi:cellphone-arrow-down', 'self',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '移动应用中心', CURRENT_TIMESTAMP,
    'Mobile app bundles: version list, upload, publish and archive (TB-23)', 'route.mobile-app',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'mobile-app'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'e5f6a7b8-9c01-4d23-8e45-6f7a8b9c0d12',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'mobile-app'),
    'mobile-app_app-center', 3, 1, '/mobile-app/app-center', 'mdi:cellphone-arrow-down', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '应用中心', CURRENT_TIMESTAMP,
    'Bundle version list with upload/publish/archive state machine (TB-23)', 'route.mobile-app_app-center',
    'view.mobile-app_app-center'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'mobile-app_app-center'
);
