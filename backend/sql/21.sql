ALTER TABLE public.ota_upgrade_tasks
	ADD COLUMN IF NOT EXISTS target_mode varchar(32) NOT NULL DEFAULT 'explicit',
	ADD COLUMN IF NOT EXISTS target_filter jsonb NULL,
	ADD COLUMN IF NOT EXISTS preview_total int8 NULL,
	ADD COLUMN IF NOT EXISTS selected_count int4 NULL,
	ADD COLUMN IF NOT EXISTS created_by varchar(36) NULL,
	ADD COLUMN IF NOT EXISTS created_by_authority varchar(64) NULL;

COMMENT ON COLUMN public.ota_upgrade_tasks.target_mode IS 'OTA target mode: explicit or filter';
COMMENT ON COLUMN public.ota_upgrade_tasks.target_filter IS 'OTA target filter snapshot';
COMMENT ON COLUMN public.ota_upgrade_tasks.preview_total IS 'Backend preview total at creation time';
COMMENT ON COLUMN public.ota_upgrade_tasks.selected_count IS 'Selected device count at creation time';
COMMENT ON COLUMN public.ota_upgrade_tasks.created_by IS 'Creator user id';
COMMENT ON COLUMN public.ota_upgrade_tasks.created_by_authority IS 'Creator authority';
