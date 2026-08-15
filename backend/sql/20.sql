CREATE TABLE IF NOT EXISTS public.command_jobs (
	id varchar(36) NOT NULL,
	tenant_id varchar(36) NOT NULL,
	operator_id varchar(36) NOT NULL,
	job_type varchar(32) NOT NULL,
	scope_type varchar(32) NOT NULL,
	identify varchar(255) NOT NULL,
	command_value text NULL,
	timeout_seconds int4 NOT NULL DEFAULT 60,
	status varchar(32) NOT NULL,
	requested_count int4 NOT NULL DEFAULT 0,
	eligible_count int4 NOT NULL DEFAULT 0,
	blocked_count int4 NOT NULL DEFAULT 0,
	submitted_count int4 NOT NULL DEFAULT 0,
	failed_count int4 NOT NULL DEFAULT 0,
	can_cancel bool NOT NULL DEFAULT false,
	can_retry_failed bool NOT NULL DEFAULT false,
	scope_snapshot jsonb NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	timeout_at timestamptz NULL,
	last_submitted_at timestamptz NULL,
	remark text NULL,
	CONSTRAINT command_jobs_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS public.command_job_details (
	id varchar(36) NOT NULL,
	command_job_id varchar(36) NOT NULL,
	tenant_id varchar(36) NOT NULL,
	device_id varchar(36) NOT NULL,
	device_number varchar(100) NULL,
	name varchar(255) NULL,
	online bool NOT NULL DEFAULT false,
	eligible bool NOT NULL DEFAULT false,
	status varchar(32) NOT NULL,
	recommended_path varchar(32) NULL,
	message_id varchar(64) NULL,
	log_recorded bool NOT NULL DEFAULT false,
	reason text NULL,
	advice text NULL,
	can_retry bool NOT NULL DEFAULT false,
	telemetry_current_count int4 NOT NULL DEFAULT 0,
	latest_telemetry_key varchar(255) NULL,
	latest_telemetry_at timestamptz NULL,
	readiness jsonb NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	submitted_at timestamptz NULL,
	completed_at timestamptz NULL,
	CONSTRAINT command_job_details_pkey PRIMARY KEY (id),
	CONSTRAINT command_job_details_job_fkey FOREIGN KEY (command_job_id) REFERENCES public.command_jobs(id) ON DELETE CASCADE
);

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

CREATE INDEX IF NOT EXISTS idx_command_jobs_tenant_created
	ON public.command_jobs USING btree (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_command_jobs_tenant_status
	ON public.command_jobs USING btree (tenant_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_command_job_details_job_status
	ON public.command_job_details USING btree (tenant_id, command_job_id, status);

CREATE INDEX IF NOT EXISTS idx_command_job_details_device
	ON public.command_job_details USING btree (tenant_id, device_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_command_job_events_job_created
	ON public.command_job_events USING btree (tenant_id, command_job_id, created_at ASC);

COMMENT ON TABLE public.command_jobs IS 'Persisted selected-device and capped device-filter command job records';
COMMENT ON TABLE public.command_job_details IS 'Per-device outcomes for persisted command job records';
COMMENT ON TABLE public.command_job_events IS 'Append-only audit events for command job lifecycle and worker dispatch';
COMMENT ON COLUMN public.command_jobs.scope_type IS 'Current contract accepts selected_devices and device_filter';
COMMENT ON COLUMN public.command_jobs.scope_snapshot IS 'Preview scope, selected devices, counts and preview token at submit time';
COMMENT ON COLUMN public.command_job_details.status IS 'blocked, ready, dispatching, submitted, failed, or canceled';
COMMENT ON COLUMN public.command_job_details.message_id IS 'Tracked command publish message id when platform accepted dispatch';
