CREATE TABLE IF NOT EXISTS public.fleet_saved_filters (
	id varchar(36) NOT NULL,
	tenant_id varchar(36) NOT NULL,
	user_id varchar(36) NOT NULL,
	name varchar(80) NOT NULL,
	device_filter jsonb NOT NULL,
	preview_total int8 NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT fleet_saved_filters_pkey PRIMARY KEY (id),
	CONSTRAINT fleet_saved_filters_user_name_unique UNIQUE (tenant_id, user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_fleet_saved_filters_user_updated
	ON public.fleet_saved_filters USING btree (tenant_id, user_id, updated_at DESC);

COMMENT ON TABLE public.fleet_saved_filters IS 'Operator-owned saved fleet device_filter snapshots for Command Center and Fleet workflows';
COMMENT ON COLUMN public.fleet_saved_filters.device_filter IS 'Normalized device_filter JSON contract reused by fleet command jobs';
