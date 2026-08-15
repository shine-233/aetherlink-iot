ALTER TABLE public.devices
	ADD COLUMN IF NOT EXISTS owner_user_id varchar(36) NULL;

CREATE INDEX IF NOT EXISTS idx_devices_tenant_owner_active
	ON public.devices USING btree (tenant_id, owner_user_id, activate_flag);

COMMENT ON COLUMN public.devices.owner_user_id IS '设备拥有者用户ID';
