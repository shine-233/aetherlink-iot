-- 2025/4/17
ALTER TABLE public.scene_action_info ALTER COLUMN action_param_type TYPE varchar(20) USING action_param_type::varchar(20);

-- 2025/4/21
ALTER TABLE public.devices ALTER COLUMN device_number TYPE varchar(100) USING device_number::varchar(100);

-- 2025/4/25 默认首页
-- 经典首页卡片注册表已退场，默认 Home 布局保持为空，让租户直接进入首台设备工作台 / ThingsVis 流程。
INSERT INTO public.boards (id, "name", config, tenant_id, created_at, updated_at, home_flag, description, remark, menu_flag) VALUES('49c82316-03d0-51eb-2a71-3294d0086599', 'Home', '[]'::json, 'aaaaaa', '2024-08-27 15:15:47.214', '2025-04-25 18:49:51.468', 'Y', '', NULL, '');
INSERT INTO public.boards (id, "name", config, tenant_id, created_at, updated_at, home_flag, description, remark, menu_flag) VALUES('ad40389b-bb15-b10f-dc4e-54d980441778', 'Home', '[]'::json, 'd616bcbb', '2025-04-25 18:22:16.200', '2025-04-25 18:22:16.200', 'Y', NULL, NULL, NULL);
