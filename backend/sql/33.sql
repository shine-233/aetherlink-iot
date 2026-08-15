ALTER TABLE public.mqtt_session_revocation_outbox
	ADD COLUMN IF NOT EXISTS required_broker_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
	ADD COLUMN IF NOT EXISTS acknowledged_at timestamptz NULL,
	ADD COLUMN IF NOT EXISTS acknowledged_broker_count int4 NOT NULL DEFAULT 0;

-- Every row present during the 32 -> 33 upgrade predates the broker-policy
-- snapshot. Requeue all legacy states behind a fail-closed marker so neither a
-- failed publish nor a crashed processing lease can fall back to an empty
-- single-broker policy before the worker snapshots the deployment roster.
UPDATE public.mqtt_session_revocation_outbox
SET status = 'pending',
	claim_token = NULL,
	next_retry_at = CURRENT_TIMESTAMP,
	required_broker_ids = '["__migration_policy_backfill_required__"]'::jsonb,
	acknowledged_at = NULL,
	acknowledged_broker_count = 0,
	updated_at = CURRENT_TIMESTAMP
WHERE status IN ('pending', 'processing', 'published', 'superseded');

ALTER TABLE public.mqtt_session_revocation_outbox
	DROP CONSTRAINT IF EXISTS mqtt_session_revocation_outbox_status_check,
	DROP CONSTRAINT IF EXISTS mqtt_session_revocation_outbox_required_broker_ids_check,
	DROP CONSTRAINT IF EXISTS mqtt_session_revocation_outbox_acknowledged_broker_count_check;

ALTER TABLE public.mqtt_session_revocation_outbox
	ADD CONSTRAINT mqtt_session_revocation_outbox_status_check
		CHECK (status IN ('pending', 'processing', 'awaiting_ack', 'acknowledged')),
	ADD CONSTRAINT mqtt_session_revocation_outbox_required_broker_ids_check
		CHECK (jsonb_typeof(required_broker_ids) = 'array'),
	ADD CONSTRAINT mqtt_session_revocation_outbox_acknowledged_broker_count_check
		CHECK (acknowledged_broker_count >= 0);

CREATE TABLE IF NOT EXISTS public.mqtt_session_revocation_acks (
	event_id varchar(36) NOT NULL,
	broker_id varchar(128) NOT NULL,
	device_id varchar(36) NOT NULL,
	revoked_at timestamptz NOT NULL,
	processed_at timestamptz NOT NULL,
	terminated_sessions int4 NOT NULL DEFAULT 0,
	created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT mqtt_session_revocation_acks_pkey PRIMARY KEY (event_id, broker_id),
	CONSTRAINT mqtt_session_revocation_acks_event_fkey
		FOREIGN KEY (event_id) REFERENCES public.mqtt_session_revocation_outbox(id) ON DELETE CASCADE,
	CONSTRAINT mqtt_session_revocation_acks_broker_id_check CHECK (btrim(broker_id) <> ''),
	CONSTRAINT mqtt_session_revocation_acks_device_id_check CHECK (btrim(device_id) <> ''),
	CONSTRAINT mqtt_session_revocation_acks_terminated_sessions_check CHECK (terminated_sessions >= 0)
);

CREATE INDEX IF NOT EXISTS mqtt_session_revocation_acks_device_idx
	ON public.mqtt_session_revocation_acks USING btree (device_id, revoked_at DESC);

COMMENT ON COLUMN public.mqtt_session_revocation_outbox.status IS
	'pending/processing/awaiting_ack/acknowledged; only acknowledged proves the configured broker acknowledgement policy';
COMMENT ON COLUMN public.mqtt_session_revocation_outbox.required_broker_ids IS
	'Broker IDs snapshotted when the event is created; an empty array accepts the first valid broker acknowledgement; migration marker rows are backfilled before worker delivery';
COMMENT ON COLUMN public.mqtt_session_revocation_outbox.acknowledged_broker_count IS
	'Number of distinct persisted broker acknowledgements that contribute to the required-broker policy for this event';
COMMENT ON TABLE public.mqtt_session_revocation_acks IS
	'Idempotent broker processing acknowledgements for durable MQTT session revocation events';
