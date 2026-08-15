ALTER TABLE public.uplink_storage_dead_letters
    ADD COLUMN IF NOT EXISTS claim_token varchar(36) NULL,
    ADD COLUMN IF NOT EXISTS lease_until timestamptz NULL;

-- Version 37 reserved the processing state but did not assign ownership. Any
-- pre-existing processing row is therefore ownerless and must become retryable
-- before the fenced lease invariant is installed.
UPDATE public.uplink_storage_dead_letters
SET status = 'retrying',
    next_retry_at = COALESCE(next_retry_at, now()),
    claim_token = NULL,
    lease_until = NULL,
    updated_at = now()
WHERE status = 'processing';

ALTER TABLE public.uplink_storage_dead_letters
    DROP CONSTRAINT IF EXISTS uplink_storage_dead_letters_processing_lease_check,
    ADD CONSTRAINT uplink_storage_dead_letters_processing_lease_check CHECK (
        (
            status = 'processing'
            AND claim_token IS NOT NULL
            AND char_length(claim_token) = 36
            AND lease_until IS NOT NULL
        )
        OR
        (
            status <> 'processing'
            AND claim_token IS NULL
            AND lease_until IS NULL
        )
    );

CREATE INDEX IF NOT EXISTS uplink_storage_dead_letters_claim_idx
    ON public.uplink_storage_dead_letters USING btree (
        status,
        next_retry_at,
        lease_until,
        created_at,
        id
    );

COMMENT ON COLUMN public.uplink_storage_dead_letters.claim_token IS
    'Per-replay fencing token; completion and retry writes must match the current processing owner.';
COMMENT ON COLUMN public.uplink_storage_dead_letters.lease_until IS
    'Expired processing leases are eligible for safe claim recovery by another backend instance.';
