-- Register the hidden Command Center handoff route for tenant operators.
-- The page is intentionally hidden from the primary menu (param3 = '1'),
-- but it must remain reachable from device-management and Ready Check links.
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
  'd7dd2c9e-9b64-4c7b-a66a-8a9f0bbf45a8',
  '5373a6a2-1861-af35-eb4c-adfd5ca55ecd',
  'device_command-center',
  3,
  1125,
  '/device/command-center',
  'mdi:send-circle-outline',
  '1',
  '["TENANT_ADMIN","SYS_ADMIN"]'::json,
  'Command Center',
  CURRENT_TIMESTAMP,
  'Hidden handoff route for selected-device command jobs',
  'route.device_command-center',
  'view.device_command-center'
WHERE NOT EXISTS (
  SELECT 1
  FROM public.sys_ui_elements
  WHERE element_code = 'device_command-center'
);
