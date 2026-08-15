-- Make the tenant-user navigation contract available after a normal deployment.
--
-- UI visibility is not API authorization: endpoint access remains protected by
-- the existing JWT/Casbin middleware. This migration only adds TENANT_USER to
-- menu elements that are already visible to TENANT_ADMIN, so dynamic route
-- initialization has a usable home/menu tree for tenant users.
--
-- The predicate makes the change idempotent for databases that were repaired by
-- the local automation fixture before this migration is applied.
UPDATE public.sys_ui_elements
SET authority = (authority::jsonb || '"TENANT_USER"'::jsonb)::json
WHERE authority IS NOT NULL
  AND authority::jsonb ? 'TENANT_ADMIN'
  AND NOT (authority::jsonb ? 'TENANT_USER');
