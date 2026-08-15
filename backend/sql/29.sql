CREATE TABLE IF NOT EXISTS public.notification_history_devices (
	notification_history_id varchar(36) NOT NULL,
	device_id varchar(36) NOT NULL,
	tenant_id varchar(36) NOT NULL,
	CONSTRAINT notification_history_devices_pkey PRIMARY KEY (notification_history_id, device_id),
	CONSTRAINT notification_history_devices_history_fk
		FOREIGN KEY (notification_history_id) REFERENCES public.notification_histories(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_notification_history_devices_history_tenant
	ON public.notification_history_devices USING btree (notification_history_id, tenant_id);

CREATE INDEX IF NOT EXISTS idx_notification_history_devices_device_tenant
	ON public.notification_history_devices USING btree (device_id, tenant_id);

CREATE INDEX IF NOT EXISTS idx_notification_histories_tenant_send_time
	ON public.notification_histories USING btree (tenant_id, send_time DESC);

COMMENT ON TABLE public.notification_history_devices IS
	'Devices whose data is included in a notification history entry';

COMMENT ON COLUMN public.notification_history_devices.device_id IS
	'Historical device reference intentionally retained after a device row is deleted';
