ALTER TABLE public.fleet_saved_filters
    ADD COLUMN IF NOT EXISTS shared bool NOT NULL DEFAULT false;

-- Rows created before this version were only ever readable by their owner, so
-- keeping them private is the behaviour-preserving backfill; sharing stays an
-- explicit opt-in performed by the owner.
UPDATE public.fleet_saved_filters
SET shared = false
WHERE shared IS NULL;

CREATE INDEX IF NOT EXISTS idx_fleet_saved_filters_tenant_shared_updated
    ON public.fleet_saved_filters USING btree (tenant_id, shared, updated_at DESC);

COMMENT ON COLUMN public.fleet_saved_filters.shared IS
    'When true the filter is readable by every member of the same tenant; write access (update/delete) always stays with user_id, and the per-user save quota only counts rows owned by that user.';
