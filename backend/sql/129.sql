-- 129.sql — TB-41 文件存储与媒体库管理（media_files 媒体登记表）
--
-- 背景：
--   1. 平台此前只有"通用上传"写入链路（api/upload.go 的 UpFile 落盘 ./files/），文件本体
--      落在文件系统、数据库侧无任何登记，vis_files（1.sql:855）是与上传链路无关的孤儿表；
--   2. 本迁移建立租户级媒体登记表 media_files，把 ./files 存储收编为可管理资产：
--      上传成功即登记（file_name 原名 / file_path 对外访问路径 / file_size 字节 / mime 类型）；
--   3. 提供媒体管理 API（列表/详情/删除）与删除时的引用统计（referenced_count 由
--      看板 config、SCADA 文档 json_data、OTA 升级包 package_url 的包含扫描在读取时刷新）；
--   4. 提供 Casbin 路由登记与多租户权限隔离：
--      - GET    /api/v1/media/files     (分页拉取本租户媒体列表，支持 search)
--      - GET    /api/v1/media/files/:id (媒体详情，含实时引用统计)
--      - DELETE /api/v1/media/files/:id (引用计数>0 拒绝删除并返回引用方；否则删文件+删行)
--   5. 前端媒体库页面挂在顶级菜单 /media/library（element_type 1 目录行 + 3 页面行）。
--
-- 关键注意事项：
--   - UNIQUE(tenant_id, file_path) 是"同租户同路径只登记一次"的最终保障；
--     上传文件名由 md5(时间戳+随机串) 生成，正常不会冲突，冲突时走幂等（DO NOTHING）。
--   - file_path 存对外访问路径（如 ./files/board/2026-09-25/xxx.png），与磁盘真实路径
--     的映射（含 OTA 前缀特例）由 service 层统一解析，删除前必须再次做根目录包含性校验。
--   - 本迁移不动 vis_files 表：那是可视化插件的历史孤儿表，收编范围为 ./files 上传链路。

-- ---- 1. 媒体登记表 (media_files) ----
CREATE TABLE IF NOT EXISTS public.media_files (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    file_name VARCHAR(255) NOT NULL,                       -- 上传时的原始文件名
    file_path VARCHAR(500) NOT NULL,                       -- 对外访问路径（./files/... 或 OTA 下载路径）
    file_size BIGINT NOT NULL DEFAULT 0,                   -- 字节数
    mime VARCHAR(100) NOT NULL DEFAULT 'application/octet-stream',
    referenced_count INT NOT NULL DEFAULT 0,               -- 最近一次引用扫描的引用方数量
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_media_files_tenant_path UNIQUE (tenant_id, file_path)
);

CREATE INDEX IF NOT EXISTS idx_media_files_tenant_created ON public.media_files(tenant_id, created_at DESC);

COMMENT ON TABLE public.media_files IS '媒体文件登记表（TB-41 文件存储与媒体库，收编 ./files 上传链路）';
COMMENT ON COLUMN public.media_files.file_path IS '对外访问路径，UNIQUE(tenant_id, file_path) 防重复登记';
COMMENT ON COLUMN public.media_files.referenced_count IS '最近一次引用扫描的引用方数量（看板/SCADA 文档/OTA 升级包）';

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/media/files'),
  ('api/v1/media/files/:id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/media/files'),
  ('SYS_ADMIN',    'api/v1/media/files/:id'),
  ('TENANT_ADMIN', 'api/v1/media/files'),
  ('TENANT_ADMIN', 'api/v1/media/files/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- 4. 媒体库菜单 (sys_ui_elements，对标 TB Files 顶级菜单) ----
-- 授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic），页面路由
-- /media/library 已注册，必须补菜单行否则路由守卫判定 403（见 100.sql 注释）。
-- 顶层目录行 element_type 1 + route_path 'layout.base'，子页面行 element_type 3 + route_path 'view.media_library'。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'b7a1c8d2-5e39-4f60-9ab4-1c6d7e8f9a04',
    '0',
    'media', 1, 118, '/media', 'mdi:image-multiple-outline', 'self',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '媒体库', CURRENT_TIMESTAMP,
    'Media library over ./files uploads (TB-41)', 'route.media',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'media'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'c2d9e0f1-6a48-5b59-8cb3-9e4f5a6b7c83',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'media'),
    'media_library', 3, 1, '/media/library', 'mdi:image-multiple-outline', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '媒体库', CURRENT_TIMESTAMP,
    'Grid preview, upload and reference-aware delete (TB-41)', 'route.media_library',
    'view.media_library'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'media_library'
);
