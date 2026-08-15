ALTER TABLE public.command_job_details
	ADD COLUMN IF NOT EXISTS response_status varchar(32) NULL,
	ADD COLUMN IF NOT EXISTS response_payload text NULL,
	ADD COLUMN IF NOT EXISTS response_error text NULL,
	ADD COLUMN IF NOT EXISTS response_at timestamptz NULL;

CREATE INDEX IF NOT EXISTS idx_command_job_details_device_message
	ON public.command_job_details USING btree (device_id, message_id);

COMMENT ON COLUMN public.command_job_details.response_status IS 'Device command response status copied from command_set_logs status';
COMMENT ON COLUMN public.command_job_details.response_payload IS 'Raw device command response payload captured when the response is processed';
COMMENT ON COLUMN public.command_job_details.response_error IS 'Device response error or failure message when available';
COMMENT ON COLUMN public.command_job_details.response_at IS 'Platform time when the device command response was processed';
