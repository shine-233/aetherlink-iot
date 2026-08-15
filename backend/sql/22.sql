CREATE TABLE IF NOT EXISTS public.command_job_events (
	id varchar(36) NOT NULL,
	command_job_id varchar(36) NOT NULL,
	tenant_id varchar(36) NOT NULL,
	detail_id varchar(36) NULL,
	device_id varchar(36) NULL,
	event_type varchar(64) NOT NULL,
	message text NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT command_job_events_pkey PRIMARY KEY (id),
	CONSTRAINT command_job_events_job_fkey FOREIGN KEY (command_job_id) REFERENCES public.command_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_command_job_events_job_created
	ON public.command_job_events USING btree (tenant_id, command_job_id, created_at ASC);

COMMENT ON TABLE public.command_job_events IS 'Append-only audit events for command job lifecycle and worker dispatch';

COMMENT ON TABLE public.command_jobs IS 'Persisted selected-device and capped device-filter command job records';
COMMENT ON TABLE public.command_job_details IS 'Per-device outcomes for persisted command job records';
COMMENT ON COLUMN public.command_jobs.scope_type IS 'Current contract accepts selected_devices and device_filter';
COMMENT ON COLUMN public.command_job_details.status IS 'blocked, ready, dispatching, submitted, failed, or canceled';
