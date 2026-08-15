CREATE TABLE IF NOT EXISTS public.uplink_storage_receipts (
    id varchar(36) NOT NULL,
    fingerprint char(64) NOT NULL,
    data_type varchar(16) NOT NULL,
    device_id varchar(36) NOT NULL,
    tenant_id varchar(36) NOT NULL,
    ts bigint NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uplink_storage_receipts_pkey PRIMARY KEY (id),
    CONSTRAINT uplink_storage_receipts_data_type_check
        CHECK (data_type = 'attribute')
);

CREATE INDEX IF NOT EXISTS uplink_storage_receipts_device_ts_idx
    ON public.uplink_storage_receipts USING btree (device_id, ts DESC);

CREATE TABLE IF NOT EXISTS public.uplink_storage_dead_letters (
    id varchar(36) NOT NULL,
    data_type varchar(16) NOT NULL,
    device_id varchar(36) NOT NULL,
    tenant_id varchar(36) NOT NULL,
    ts bigint NOT NULL,
    payload jsonb NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'pending',
    attempts int4 NOT NULL DEFAULT 0,
    last_error text NULL,
    next_retry_at timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uplink_storage_dead_letters_pkey PRIMARY KEY (id),
    CONSTRAINT uplink_storage_dead_letters_data_type_check
        CHECK (data_type IN ('attribute', 'event')),
    CONSTRAINT uplink_storage_dead_letters_status_check
        CHECK (status IN ('pending', 'retrying', 'processing', 'resolved', 'dead')),
    CONSTRAINT uplink_storage_dead_letters_attempts_check
        CHECK (attempts >= 0)
);

CREATE INDEX IF NOT EXISTS uplink_storage_dead_letters_replay_idx
    ON public.uplink_storage_dead_letters USING btree (status, next_retry_at, created_at, id);

CREATE INDEX IF NOT EXISTS uplink_storage_dead_letters_device_ts_idx
    ON public.uplink_storage_dead_letters USING btree (device_id, ts DESC);

COMMENT ON TABLE public.uplink_storage_dead_letters IS
    'Replayable attribute/event envelopes retained after their primary storage write fails.';
COMMENT ON COLUMN public.uplink_storage_dead_letters.id IS
    'Stable envelope identity shared by primary write, dead-letter, file spool and replay.';
COMMENT ON COLUMN public.uplink_storage_dead_letters.payload IS
    'Versioned canonical attribute/event envelope with occurrence message_id and full SHA-256 fingerprint; it contains no credentials, raw MQTT message_id or arbitrary protocol metadata.';

COMMENT ON TABLE public.uplink_storage_receipts IS
    'Primary-transaction receipts that make complete attribute envelopes idempotent and collision-verifiable.';
COMMENT ON COLUMN public.uplink_storage_receipts.id IS
    'UUID-shaped occurrence message_id shared with primary, fallback and replay records.';
COMMENT ON COLUMN public.uplink_storage_receipts.fingerprint IS
    'Full SHA-256 fingerprint used to detect a truncated message_id collision.';
