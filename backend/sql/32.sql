CREATE TABLE IF NOT EXISTS public.email_templates (
	id varchar(36) NOT NULL,
	tenant_id varchar(36) NOT NULL DEFAULT '',
	name varchar(120) NOT NULL,
	purpose varchar(32) NOT NULL DEFAULT 'ALARM',
	subject_template varchar(500) NOT NULL,
	body_template text NOT NULL,
	enabled boolean NOT NULL DEFAULT true,
	is_default boolean NOT NULL DEFAULT false,
	created_by varchar(36) NOT NULL,
	created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT email_templates_pkey PRIMARY KEY (id),
	CONSTRAINT email_templates_purpose_check CHECK (purpose IN ('ALARM'))
);

CREATE INDEX IF NOT EXISTS idx_email_templates_scope
	ON public.email_templates USING btree (tenant_id, purpose, updated_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_email_templates_default_scope
	ON public.email_templates USING btree (tenant_id, purpose)
	WHERE enabled = true AND is_default = true;

COMMENT ON TABLE public.email_templates IS
	'System and tenant scoped templates used to wrap alarm email subject and body';
