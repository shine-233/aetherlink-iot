ALTER TABLE public.mqtt_session_revocation_outbox
	ADD COLUMN IF NOT EXISTS claim_token varchar(36) NULL;

COMMENT ON COLUMN public.mqtt_session_revocation_outbox.claim_token IS
	'Per-claim fencing token; completion and retry writes must match the current processing owner';
