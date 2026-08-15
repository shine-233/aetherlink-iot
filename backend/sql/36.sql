ALTER TABLE public.command_jobs
    ADD COLUMN IF NOT EXISTS next_dispatch_at timestamptz NULL;

CREATE TABLE IF NOT EXISTS public.command_job_dispatch_quotas (
    scope_type varchar(16) NOT NULL,
    scope_id varchar(64) NOT NULL,
    next_dispatch_at timestamptz NOT NULL DEFAULT now(),
    max_concurrent int4 NOT NULL,
    rate_per_second numeric(12, 3) NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT command_job_dispatch_quotas_pkey PRIMARY KEY (scope_type, scope_id),
    CONSTRAINT command_job_dispatch_quotas_scope_check CHECK (scope_type IN ('global', 'tenant')),
    CONSTRAINT command_job_dispatch_quotas_concurrency_check CHECK (max_concurrent > 0),
    CONSTRAINT command_job_dispatch_quotas_rate_check CHECK (rate_per_second > 0)
);

CREATE INDEX IF NOT EXISTS idx_command_jobs_next_dispatch_due
    ON public.command_jobs USING btree (next_dispatch_at ASC, updated_at ASC)
    WHERE status = 'running' AND next_dispatch_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_command_job_details_global_dispatching_lease
    ON public.command_job_details USING btree (dispatch_lease_until ASC)
    WHERE status = 'dispatching' AND eligible = true;

COMMENT ON COLUMN public.command_jobs.next_dispatch_at IS
    'Durable earliest time when a recovery scan should resume dispatch attempts for this job.';
COMMENT ON TABLE public.command_job_dispatch_quotas IS
    'Database-locked global and tenant dispatch-rate cursors shared by every backend instance.';
COMMENT ON COLUMN public.command_job_dispatch_quotas.next_dispatch_at IS
    'Leaky-bucket cursor advanced atomically whenever a command row is claimed.';
