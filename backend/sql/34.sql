ALTER TABLE public.command_jobs
    ADD COLUMN IF NOT EXISTS scheduled_at timestamptz NULL;

CREATE INDEX IF NOT EXISTS idx_command_jobs_scheduled_due
    ON public.command_jobs USING btree (scheduled_at ASC, updated_at ASC)
    WHERE status = 'scheduled';

COMMENT ON COLUMN public.command_jobs.scheduled_at IS
    'Optional operator-requested start time. Future jobs stay scheduled until a recovery scan atomically activates them.';
