CREATE TABLE IF NOT EXISTS public.mqtt_session_revocation_outbox (
	id varchar(36) NOT NULL,
	device_id varchar(36) NOT NULL,
	revoked_at timestamptz NOT NULL,
	status varchar(32) NOT NULL DEFAULT 'pending',
	claim_token varchar(36) NULL,
	attempts int4 NOT NULL DEFAULT 0,
	last_error text NULL,
	next_retry_at timestamptz NULL,
	published_at timestamptz NULL,
	subscriber_count int8 NULL,
	required_broker_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
	acknowledged_at timestamptz NULL,
	acknowledged_broker_count int4 NOT NULL DEFAULT 0,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT mqtt_session_revocation_outbox_pkey PRIMARY KEY (id),
	CONSTRAINT mqtt_session_revocation_outbox_status_check
		CHECK (status IN ('pending', 'processing', 'awaiting_ack', 'acknowledged')),
	CONSTRAINT mqtt_session_revocation_outbox_required_broker_ids_check
		CHECK (jsonb_typeof(required_broker_ids) = 'array'),
	CONSTRAINT mqtt_session_revocation_outbox_acknowledged_broker_count_check
		CHECK (acknowledged_broker_count >= 0)
);

CREATE INDEX IF NOT EXISTS mqtt_session_revocation_outbox_retry_idx
	ON public.mqtt_session_revocation_outbox USING btree (status, next_retry_at, created_at);

CREATE INDEX IF NOT EXISTS mqtt_session_revocation_outbox_device_idx
	ON public.mqtt_session_revocation_outbox USING btree (device_id, revoked_at DESC);

CREATE TABLE IF NOT EXISTS public.mqtt_session_revocation_acks (
	event_id varchar(36) NOT NULL,
	broker_id varchar(128) NOT NULL,
	device_id varchar(36) NOT NULL,
	revoked_at timestamptz NOT NULL,
	processed_at timestamptz NOT NULL,
	terminated_sessions int4 NOT NULL DEFAULT 0,
	created_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT mqtt_session_revocation_acks_pkey PRIMARY KEY (event_id, broker_id),
	CONSTRAINT mqtt_session_revocation_acks_event_fkey
		FOREIGN KEY (event_id) REFERENCES public.mqtt_session_revocation_outbox(id) ON DELETE CASCADE,
	CONSTRAINT mqtt_session_revocation_acks_broker_id_check CHECK (btrim(broker_id) <> ''),
	CONSTRAINT mqtt_session_revocation_acks_device_id_check CHECK (btrim(device_id) <> ''),
	CONSTRAINT mqtt_session_revocation_acks_terminated_sessions_check CHECK (terminated_sessions >= 0)
);

CREATE INDEX IF NOT EXISTS mqtt_session_revocation_acks_device_idx
	ON public.mqtt_session_revocation_acks USING btree (device_id, revoked_at DESC);

COMMENT ON TABLE public.mqtt_session_revocation_outbox IS
	'Durable outbox for SW3-triggered MQTT session revocation publication';
COMMENT ON COLUMN public.mqtt_session_revocation_outbox.revoked_at IS
	'Device state-version cutoff; broker must not terminate sessions authenticated from a later device version';
COMMENT ON COLUMN public.mqtt_session_revocation_outbox.status IS
	'pending/processing/awaiting_ack/acknowledged; only acknowledged proves the configured broker acknowledgement policy';
COMMENT ON COLUMN public.mqtt_session_revocation_outbox.claim_token IS
	'Per-claim fencing token; completion and retry writes must match the current processing owner';
COMMENT ON COLUMN public.mqtt_session_revocation_outbox.subscriber_count IS
	'Redis subscriber count returned at publish time; not a delivery or session-termination acknowledgement';
COMMENT ON COLUMN public.mqtt_session_revocation_outbox.required_broker_ids IS
	'Broker IDs snapshotted when the event is created; an empty array accepts the first valid broker acknowledgement; the reserved migration marker must be backfilled before delivery';
COMMENT ON COLUMN public.mqtt_session_revocation_outbox.acknowledged_broker_count IS
	'Number of distinct persisted broker acknowledgements that contribute to the required-broker policy for this event';
COMMENT ON TABLE public.mqtt_session_revocation_acks IS
	'Idempotent broker processing acknowledgements for durable MQTT session revocation events';
