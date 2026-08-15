-- Derive each alarm stream's current state before deriving a device summary.
-- Alarm recovery appends a newer N row instead of mutating the older H/M/L row,
-- while an operator reset mutates the selected history row in place. Therefore
-- a historical H/M/L row must not outrank a newer N row from the same stream.
CREATE OR REPLACE VIEW public.current_device_alarm_streams AS
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
      ORDER BY
        create_at DESC NULLS LAST,
        id DESC
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

CREATE OR REPLACE VIEW public.latest_device_alarms AS
WITH
ranked_device_states AS (
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
