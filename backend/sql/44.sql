-- Rebuild alarm summary views defensively for legacy rows whose
-- alarm_device_list is a JSON scalar/object instead of an array.  The
-- historical write path accepted those values, and jsonb_array_elements_text
-- raises SQLSTATE 22023 when it is called on them.
DROP VIEW IF EXISTS public.latest_device_alarms;
DROP VIEW IF EXISTS public.current_device_alarm_streams;

CREATE VIEW public.current_device_alarm_streams AS
WITH unnested_devices AS (
  SELECT
    ah.id,
    ah.alarm_config_id,
    ah.group_id,
    ah.scene_automation_id,
    ah.name,
    ah.description,
    ah.content,
    ah.alarm_status,
    ah.tenant_id,
    ah.remark,
    ah.create_at,
    jsonb_array_elements_text(
      CASE
        WHEN ah.alarm_device_list IS NULL THEN '[]'::jsonb
        WHEN jsonb_typeof(ah.alarm_device_list::jsonb) = 'array'
          THEN ah.alarm_device_list::jsonb
        ELSE '[]'::jsonb
      END
    ) AS device_id
  FROM public.alarm_history ah
),
ranked_stream_states AS (
  SELECT
    *,
    ROW_NUMBER() OVER (
      PARTITION BY tenant_id, device_id, alarm_config_id, group_id, scene_automation_id
      ORDER BY create_at DESC NULLS LAST, id DESC
    ) AS stream_rn
  FROM unnested_devices
),
current_stream_states AS (
  SELECT
    id,
    alarm_config_id,
    group_id,
    scene_automation_id,
    name,
    description,
    content,
    alarm_status,
    tenant_id,
    remark,
    create_at,
    device_id
  FROM ranked_stream_states
  WHERE stream_rn = 1
)
SELECT *
FROM current_stream_states;

COMMENT ON VIEW public.current_device_alarm_streams IS
  'Newest state for each tenant, device and alarm-config/group/scene stream; malformed device lists are ignored.';

CREATE VIEW public.latest_device_alarms AS
WITH ranked_device_states AS (
  SELECT
    *,
    ROW_NUMBER() OVER (
      PARTITION BY tenant_id, device_id
      ORDER BY
        CASE WHEN alarm_status IN ('H', 'M', 'L') THEN 0 ELSE 1 END,
        create_at DESC NULLS LAST,
        id DESC
    ) AS device_rn
  FROM public.current_device_alarm_streams
)
SELECT
  id,
  alarm_config_id,
  group_id,
  scene_automation_id,
  name,
  description,
  content,
  alarm_status,
  tenant_id,
  remark,
  create_at,
  device_id
FROM ranked_device_states
WHERE device_rn = 1;

COMMENT ON VIEW public.latest_device_alarms IS
  'One current alarm summary per device; malformed device lists are ignored.';

-- The RDI alarm overview route is part of the tenant alarm workflow.  Older
-- databases predate the route and therefore omit it from the dynamic menu,
-- causing the frontend route guard to render the access boundary instead of
-- mounting the overview component.  Keep this insert idempotent for upgrades.
INSERT INTO public.sys_ui_elements (
  id,
  parent_id,
  element_code,
  element_type,
  orders,
  param1,
  param2,
  param3,
  authority,
  description,
  created_at,
  remark,
  multilingual,
  route_path
)
SELECT
  'b8f6bb70-5a38-4e6d-9d85-6acb3e65e5f2',
  '650bc444-7672-1123-1e41-7e37365b0186',
  'alarm_rdi-overview',
  3,
  998,
  '/alarm/rdi-overview',
  'mdi:chart-bell-curve',
  'self',
  '["TENANT_ADMIN"]'::json,
  'RDI 告警概览',
  CURRENT_TIMESTAMP,
  '',
  'route.alarm_rdi-overview',
  'view.alarm_rdi-overview'
WHERE NOT EXISTS (
  SELECT 1
  FROM public.sys_ui_elements
  WHERE element_code = 'alarm_rdi-overview'
);
