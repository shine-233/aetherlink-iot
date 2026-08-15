CREATE TABLE IF NOT EXISTS public.payload_schemas (
    id varchar(36) NOT NULL,
    tenant_id varchar(36) NOT NULL,
    name varchar(128) NOT NULL,
    description varchar(500) NULL,
    strict bool NOT NULL DEFAULT false,
    fields jsonb NOT NULL,
    created_by varchar(36) NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT payload_schemas_pkey PRIMARY KEY (id),
    CONSTRAINT payload_schemas_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS payload_schemas_tenant_updated_idx
    ON public.payload_schemas USING btree (tenant_id, updated_at DESC, id);

COMMENT ON TABLE public.payload_schemas IS
    'Tenant-scoped payload field-constraint registry reused by the static payload validation engine; broker-side enforcement of these schemas is a separate MQTT-contract change verified at runtime.';
COMMENT ON COLUMN public.payload_schemas.fields IS
    'Declared field constraints (name/type/required/min/max/enum/pattern) as a JSON array; the same shape the stateless validation engine consumes.';
COMMENT ON COLUMN public.payload_schemas.strict IS
    'When true, a payload carrying keys not declared in fields is rejected rather than warned; this is the persisted default a broker enforcement layer would read.';
