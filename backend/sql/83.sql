-- Durable report scheduling and SMTP outbox state.
-- Legacy timestamp values were written from UTC application clocks; reinterpret each
-- column only while it still has the legacy timestamp-without-time-zone type.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'report_schedules'
          AND column_name = 'last_run_at' AND data_type = 'timestamp without time zone'
    ) THEN
        ALTER TABLE public.report_schedules
            ALTER COLUMN last_run_at TYPE timestamptz USING last_run_at AT TIME ZONE 'UTC';
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'report_schedules'
          AND column_name = 'created_at' AND data_type = 'timestamp without time zone'
    ) THEN
        ALTER TABLE public.report_schedules
            ALTER COLUMN created_at TYPE timestamptz USING created_at AT TIME ZONE 'UTC';
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'report_schedules'
          AND column_name = 'updated_at' AND data_type = 'timestamp without time zone'
    ) THEN
        ALTER TABLE public.report_schedules
            ALTER COLUMN updated_at TYPE timestamptz USING updated_at AT TIME ZONE 'UTC';
    END IF;
END $$;

ALTER TABLE public.report_schedules
    ADD COLUMN IF NOT EXISTS timezone varchar(255) NOT NULL DEFAULT 'UTC',
    ADD COLUMN IF NOT EXISTS next_run_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS last_run_id varchar(36) NULL,
    ADD COLUMN IF NOT EXISTS schedule_error_code varchar(128) NULL,
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz NULL;

ALTER TABLE public.report_schedules
    DROP CONSTRAINT IF EXISTS report_schedules_revision_check,
    DROP CONSTRAINT IF EXISTS report_schedules_error_code_check,
    ADD CONSTRAINT report_schedules_revision_check CHECK (revision >= 1),
    ADD CONSTRAINT report_schedules_error_code_check CHECK (
        schedule_error_code IS NULL OR schedule_error_code IN ('invalid_schedule')
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_schedules_identity
    ON public.report_schedules (id, tenant_id);

CREATE INDEX IF NOT EXISTS idx_report_schedules_due
    ON public.report_schedules (next_run_at, id)
    WHERE enabled AND deleted_at IS NULL AND next_run_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_report_schedules_tenant_history
    ON public.report_schedules (tenant_id, created_at DESC, id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS public.report_schedule_runs (
    id varchar(36) PRIMARY KEY,
    tenant_id varchar(36) NOT NULL,
    schedule_id varchar(36) NOT NULL,
    trigger varchar(16) NOT NULL,
    scheduled_slot timestamptz NULL,
    retry_parent_run_id varchar(36) NULL,
    idempotency_key_hash varchar(64) NULL,
    request_fingerprint varchar(64) NULL,
    window_start_at timestamptz NOT NULL,
    window_end_at timestamptz NOT NULL,
    config_snapshot jsonb NOT NULL,
    misfire_count integer NOT NULL DEFAULT 0,
    misfire_first_slot timestamptz NULL,
    misfire_last_slot timestamptz NULL,
    generation_status varchar(16) NOT NULL DEFAULT 'pending',
    attempt_count integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 3,
    next_attempt_at timestamptz NULL,
    claim_token uuid NULL,
    lease_until timestamptz NULL,
    result jsonb NULL,
    error_code varchar(128) NULL,
    error_message text NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz NULL,
    generated_at timestamptz NULL,
    completed_at timestamptz NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT report_schedule_runs_tenant_identity_unique UNIQUE (id, tenant_id),
    CONSTRAINT report_schedule_runs_identity_unique UNIQUE (id, tenant_id, schedule_id),
    CONSTRAINT report_schedule_runs_schedule_fk FOREIGN KEY (schedule_id, tenant_id)
        REFERENCES public.report_schedules(id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT report_schedule_runs_retry_parent_fk FOREIGN KEY (retry_parent_run_id, tenant_id, schedule_id)
        REFERENCES public.report_schedule_runs(id, tenant_id, schedule_id) ON DELETE RESTRICT,
    CONSTRAINT report_schedule_runs_trigger_check CHECK (trigger IN ('scheduled', 'manual', 'retry')),
    CONSTRAINT report_schedule_runs_trigger_shape_check CHECK (
        (trigger = 'scheduled' AND scheduled_slot IS NOT NULL AND retry_parent_run_id IS NULL)
        OR (trigger = 'manual' AND scheduled_slot IS NULL AND retry_parent_run_id IS NULL)
        OR (trigger = 'retry' AND scheduled_slot IS NULL AND retry_parent_run_id IS NOT NULL)
    ),
    CONSTRAINT report_schedule_runs_window_check CHECK (window_start_at < window_end_at),
    CONSTRAINT report_schedule_runs_config_snapshot_check CHECK (jsonb_typeof(config_snapshot) = 'object'),
    CONSTRAINT report_schedule_runs_misfire_check CHECK (
        misfire_count >= 0 AND (
            (misfire_count = 0 AND misfire_first_slot IS NULL AND misfire_last_slot IS NULL)
            OR (misfire_count > 0 AND misfire_first_slot IS NOT NULL AND misfire_last_slot IS NOT NULL
                AND misfire_first_slot <= misfire_last_slot)
        )
    ),
    CONSTRAINT report_schedule_runs_generation_status_check CHECK (
        generation_status IN ('pending', 'processing', 'retrying', 'succeeded', 'failed')
    ),
    CONSTRAINT report_schedule_runs_attempts_check CHECK (
        attempt_count >= 0 AND max_attempts >= 1 AND attempt_count <= max_attempts
    ),
    CONSTRAINT report_schedule_runs_processing_lease_check CHECK (
        (generation_status = 'processing' AND claim_token IS NOT NULL AND lease_until IS NOT NULL)
        OR (generation_status <> 'processing' AND claim_token IS NULL AND lease_until IS NULL)
    ),
    CONSTRAINT report_schedule_runs_idempotency_shape_check CHECK (
        (idempotency_key_hash IS NULL AND request_fingerprint IS NULL)
        OR (idempotency_key_hash IS NOT NULL AND request_fingerprint IS NOT NULL
            AND idempotency_key_hash ~ '^[0-9a-f]{64}$' AND request_fingerprint ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT report_schedule_runs_terminal_time_check CHECK (
        (generation_status IN ('succeeded', 'failed') AND completed_at IS NOT NULL)
        OR (generation_status NOT IN ('succeeded', 'failed') AND completed_at IS NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_schedule_runs_scheduled_slot
    ON public.report_schedule_runs (schedule_id, scheduled_slot)
    WHERE trigger = 'scheduled';

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_schedule_runs_idempotency
    ON public.report_schedule_runs (tenant_id, schedule_id, trigger, idempotency_key_hash)
    WHERE idempotency_key_hash IS NOT NULL AND trigger IN ('manual', 'retry');

CREATE INDEX IF NOT EXISTS idx_report_schedule_runs_due
    ON public.report_schedule_runs (next_attempt_at, created_at, id)
    WHERE generation_status IN ('pending', 'retrying');

CREATE INDEX IF NOT EXISTS idx_report_schedule_runs_claim_recovery
    ON public.report_schedule_runs (lease_until, id)
    WHERE generation_status = 'processing';

CREATE INDEX IF NOT EXISTS idx_report_schedule_runs_history
    ON public.report_schedule_runs (tenant_id, schedule_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_report_schedule_runs_retry_parent
    ON public.report_schedule_runs (retry_parent_run_id, created_at DESC)
    WHERE retry_parent_run_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS public.report_schedule_deliveries (
    run_id varchar(36) PRIMARY KEY,
    tenant_id varchar(36) NOT NULL,
    envelope_from text NOT NULL,
    envelope_recipients jsonb NOT NULL,
    message_id varchar(255) NOT NULL,
    subject text NOT NULL,
    payload bytea NULL,
    payload_digest varchar(64) NOT NULL,
    payload_size bigint NOT NULL,
    row_count bigint NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'pending',
    attempt_count integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 3,
    next_attempt_at timestamptz NULL,
    claim_token uuid NULL,
    lease_until timestamptz NULL,
    last_error text NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz NULL,
    accepted_at timestamptz NULL,
    failed_at timestamptz NULL,
    ambiguous_at timestamptz NULL,
    completed_at timestamptz NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT report_schedule_deliveries_run_fk FOREIGN KEY (run_id, tenant_id)
        REFERENCES public.report_schedule_runs(id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT report_schedule_deliveries_recipients_check CHECK (
        jsonb_typeof(envelope_recipients) = 'array' AND jsonb_array_length(envelope_recipients) > 0
    ),
    CONSTRAINT report_schedule_deliveries_payload_check CHECK (
        payload_size >= 0 AND row_count >= 0 AND payload_digest ~ '^[0-9a-f]{64}$'
        AND ((status IN ('pending', 'processing', 'retrying') AND payload IS NOT NULL
                AND payload_size = octet_length(payload))
            OR (status IN ('accepted', 'failed', 'ambiguous') AND payload IS NULL))
    ),
    CONSTRAINT report_schedule_deliveries_status_check CHECK (
        status IN ('pending', 'processing', 'retrying', 'accepted', 'failed', 'ambiguous')
    ),
    CONSTRAINT report_schedule_deliveries_attempts_check CHECK (
        attempt_count >= 0 AND max_attempts >= 1 AND attempt_count <= max_attempts
    ),
    CONSTRAINT report_schedule_deliveries_processing_lease_check CHECK (
        (status = 'processing' AND claim_token IS NOT NULL AND lease_until IS NOT NULL)
        OR (status <> 'processing' AND claim_token IS NULL AND lease_until IS NULL)
    ),
    CONSTRAINT report_schedule_deliveries_terminal_time_check CHECK (
        (status = 'accepted' AND accepted_at IS NOT NULL AND failed_at IS NULL AND ambiguous_at IS NULL
            AND completed_at IS NOT NULL)
        OR (status = 'failed' AND accepted_at IS NULL AND failed_at IS NOT NULL AND ambiguous_at IS NULL
            AND completed_at IS NOT NULL)
        OR (status = 'ambiguous' AND accepted_at IS NULL AND failed_at IS NULL AND ambiguous_at IS NOT NULL
            AND completed_at IS NOT NULL)
        OR (status IN ('pending', 'processing', 'retrying') AND accepted_at IS NULL AND failed_at IS NULL
            AND ambiguous_at IS NULL AND completed_at IS NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_report_schedule_deliveries_message_id
    ON public.report_schedule_deliveries (message_id);

CREATE INDEX IF NOT EXISTS idx_report_schedule_deliveries_due
    ON public.report_schedule_deliveries (next_attempt_at, created_at, run_id)
    WHERE status IN ('pending', 'retrying');

CREATE INDEX IF NOT EXISTS idx_report_schedule_deliveries_claim_recovery
    ON public.report_schedule_deliveries (lease_until, run_id)
    WHERE status = 'processing';

CREATE INDEX IF NOT EXISTS idx_report_schedule_deliveries_history
    ON public.report_schedule_deliveries (tenant_id, created_at DESC, run_id);

ALTER TABLE public.report_schedules
    DROP CONSTRAINT IF EXISTS report_schedules_last_run_fk,
    ADD CONSTRAINT report_schedules_last_run_fk FOREIGN KEY (last_run_id, tenant_id, id)
        REFERENCES public.report_schedule_runs(id, tenant_id, schedule_id) ON DELETE RESTRICT;

-- Register every nested durable-run route for fail-closed route auditing.
INSERT INTO public.casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
    ('api/v1/report/schedules/:id/runs'),
    ('api/v1/report/schedules/:id/runs/:run_id'),
    ('api/v1/report/schedules/:id/runs/:run_id/retry')
) AS r(path)
WHERE NOT EXISTS (
    SELECT 1 FROM public.casbin_rule c
    WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

INSERT INTO public.casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
    ('SYS_ADMIN', 'api/v1/report/schedules/:id/runs'),
    ('TENANT_ADMIN', 'api/v1/report/schedules/:id/runs'),
    ('SYS_ADMIN', 'api/v1/report/schedules/:id/runs/:run_id'),
    ('TENANT_ADMIN', 'api/v1/report/schedules/:id/runs/:run_id'),
    ('SYS_ADMIN', 'api/v1/report/schedules/:id/runs/:run_id/retry'),
    ('TENANT_ADMIN', 'api/v1/report/schedules/:id/runs/:run_id/retry')
) AS r(role, path)
WHERE NOT EXISTS (
    SELECT 1 FROM public.casbin_rule c
    WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- Keep the generated Visualization report route visible to its administrator roles.
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'd43c06b8-0c55-4f0f-8a29-f1df239fd083',
    '95e2a961-382b-f4a6-87b3-1898123c95bc',
    'visualization_report', 3, 3, '/visualization/report', 'mdi:file-chart-outline', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '定时报表', CURRENT_TIMESTAMP,
    'Durable report scheduling and delivery history', 'route.visualization-report', 'view.visualization_report'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'visualization_report'
);
