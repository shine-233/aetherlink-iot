ALTER TABLE public.command_job_details
	ADD COLUMN IF NOT EXISTS dispatch_attempts int4 NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS dispatch_lease_token varchar(64) NULL,
	ADD COLUMN IF NOT EXISTS dispatch_lease_until timestamptz NULL,
	ADD COLUMN IF NOT EXISTS last_dispatch_started_at timestamptz NULL,
	ADD COLUMN IF NOT EXISTS next_retry_after timestamptz NULL;

CREATE INDEX IF NOT EXISTS idx_command_job_details_dispatch_lease
	ON public.command_job_details USING btree (tenant_id, status, dispatch_lease_until);

CREATE INDEX IF NOT EXISTS idx_command_job_details_retry_after
	ON public.command_job_details USING btree (tenant_id, command_job_id, status, can_retry, next_retry_after);

COMMENT ON COLUMN public.command_job_details.dispatch_attempts IS 'Number of times this command job row has been claimed for dispatch';
COMMENT ON COLUMN public.command_job_details.dispatch_lease_token IS 'Internal worker lease token for the current dispatch attempt; not exposed to operators';
COMMENT ON COLUMN public.command_job_details.dispatch_lease_until IS 'Lease expiry for the current dispatch attempt';
COMMENT ON COLUMN public.command_job_details.last_dispatch_started_at IS 'Platform time when the latest dispatch attempt was claimed';
COMMENT ON COLUMN public.command_job_details.next_retry_after IS 'Earliest platform time when the failed row may be requeued by the retry action';
