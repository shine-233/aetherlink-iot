-- The alarm summary views expose alarm_history.remark, so PostgreSQL requires
-- them to be removed before widening the underlying column type.
DROP VIEW IF EXISTS public.latest_device_alarms;
DROP VIEW IF EXISTS public.current_device_alarm_streams;

ALTER TABLE public.alarm_history
    ALTER COLUMN remark TYPE text;

COMMENT ON COLUMN public.alarm_history.remark IS
    'JSON audit metadata for acknowledge/reset actions; text avoids truncating cumulative action history.';

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
    jsonb_array_elements_text(ah.alarm_device_list) AS device_id
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
  'Newest state for each tenant, device and alarm-config/group/scene stream.';

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
  'One current alarm summary per device: newest state per alarm stream, then active H/M/L stream first.';
