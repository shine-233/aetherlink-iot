ALTER TABLE public.ota_upgrade_tasks
    ADD COLUMN IF NOT EXISTS status varchar(32) NOT NULL DEFAULT 'running',
    ADD COLUMN IF NOT EXISTS status_description text NULL,
    ADD COLUMN IF NOT EXISTS scheduled_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS started_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS completed_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS timeout_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS timeout_seconds int4 NOT NULL DEFAULT 3600,
    ADD COLUMN IF NOT EXISTS rollout_rate_per_minute int4 NOT NULL DEFAULT 60,
    ADD COLUMN IF NOT EXISTS abort_failure_rate_percent numeric(5, 2) NULL,
    ADD COLUMN IF NOT EXISTS next_dispatch_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS rate_window_started_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS rate_window_dispatched int4 NOT NULL DEFAULT 0;

ALTER TABLE public.ota_upgrade_task_details
    ADD COLUMN IF NOT EXISTS dispatch_attempts int4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS dispatch_lease_token varchar(64) NULL,
    ADD COLUMN IF NOT EXISTS dispatch_lease_until timestamptz NULL,
    ADD COLUMN IF NOT EXISTS last_dispatch_started_at timestamptz NULL;

UPDATE public.ota_upgrade_tasks
SET scheduled_at = COALESCE(scheduled_at, created_at),
    next_dispatch_at = COALESCE(next_dispatch_at, now())
WHERE status IN ('scheduled', 'running');

ALTER TABLE public.ota_upgrade_tasks
    DROP CONSTRAINT IF EXISTS ota_upgrade_tasks_status_check,
    ADD CONSTRAINT ota_upgrade_tasks_status_check CHECK (
        status IN (
            'scheduled',
            'running',
            'completed',
            'partially_failed',
            'failed',
            'canceled',
            'aborted',
            'timed_out'
        )
    ),
    DROP CONSTRAINT IF EXISTS ota_upgrade_tasks_timeout_seconds_check,
    ADD CONSTRAINT ota_upgrade_tasks_timeout_seconds_check
        CHECK (timeout_seconds BETWEEN 60 AND 604800),
    DROP CONSTRAINT IF EXISTS ota_upgrade_tasks_rollout_rate_check,
    ADD CONSTRAINT ota_upgrade_tasks_rollout_rate_check
        CHECK (rollout_rate_per_minute BETWEEN 1 AND 300),
    DROP CONSTRAINT IF EXISTS ota_upgrade_tasks_abort_failure_rate_check,
    ADD CONSTRAINT ota_upgrade_tasks_abort_failure_rate_check
        CHECK (abort_failure_rate_percent IS NULL OR (abort_failure_rate_percent > 0 AND abort_failure_rate_percent <= 100)),
    DROP CONSTRAINT IF EXISTS ota_upgrade_tasks_rate_window_dispatched_check,
    ADD CONSTRAINT ota_upgrade_tasks_rate_window_dispatched_check
        CHECK (rate_window_dispatched >= 0);

ALTER TABLE public.ota_upgrade_task_details
    DROP CONSTRAINT IF EXISTS ota_upgrade_task_details_dispatch_attempts_check,
    ADD CONSTRAINT ota_upgrade_task_details_dispatch_attempts_check
        CHECK (dispatch_attempts >= 0);

CREATE INDEX IF NOT EXISTS idx_ota_upgrade_tasks_rollout_due
    ON public.ota_upgrade_tasks USING btree (next_dispatch_at ASC, scheduled_at ASC, created_at ASC)
    WHERE status IN ('scheduled', 'running');

CREATE INDEX IF NOT EXISTS idx_ota_upgrade_tasks_timeout_due
    ON public.ota_upgrade_tasks USING btree (timeout_at ASC)
    WHERE status = 'running' AND timeout_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ota_upgrade_task_details_dispatch_claim
    ON public.ota_upgrade_task_details USING btree (ota_upgrade_task_id, updated_at ASC, id ASC)
    WHERE status = 1;

CREATE INDEX IF NOT EXISTS idx_ota_upgrade_task_details_dispatch_lease
    ON public.ota_upgrade_task_details USING btree (dispatch_lease_until ASC)
    WHERE status = 1 AND dispatch_lease_token IS NOT NULL;

COMMENT ON COLUMN public.ota_upgrade_tasks.status IS
    'Durable rollout governance status; device-level progress remains in ota_upgrade_task_details.';
COMMENT ON COLUMN public.ota_upgrade_tasks.rollout_rate_per_minute IS
    'Maximum device dispatch starts in one UTC minute; dispatch may occur in bounded batches.';
COMMENT ON COLUMN public.ota_upgrade_tasks.abort_failure_rate_percent IS
    'Optional failure-rate threshold evaluated as failed / (succeeded + failed) * 100.';
COMMENT ON COLUMN public.ota_upgrade_tasks.timeout_at IS
    'Absolute rollout deadline set when a scheduled task enters running state.';
COMMENT ON COLUMN public.ota_upgrade_task_details.dispatch_lease_token IS
    'Database claim token preventing concurrent backend instances from publishing the same pending row.';
