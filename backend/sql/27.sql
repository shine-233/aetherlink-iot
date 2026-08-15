CREATE TABLE IF NOT EXISTS public.telemetry_dead_letters (
	id varchar(36) NOT NULL,
	device_id varchar(36) NOT NULL,
	tenant_id varchar(36) NOT NULL,
	"key" varchar(255) NOT NULL,
	ts int8 NOT NULL,
	bool_v bool NULL,
	number_v float8 NULL,
	string_v text NULL,
	raw_payload jsonb NOT NULL,
	status varchar(32) NOT NULL DEFAULT 'pending',
	attempts int4 NOT NULL DEFAULT 0,
	last_error text NULL,
	next_retry_at timestamptz NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT telemetry_dead_letters_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS telemetry_dead_letters_status_retry_idx
	ON public.telemetry_dead_letters USING btree (status, next_retry_at);

CREATE INDEX IF NOT EXISTS telemetry_dead_letters_device_ts_idx
	ON public.telemetry_dead_letters USING btree (device_id, ts DESC);

COMMENT ON TABLE public.telemetry_dead_letters IS '遥测写入失败后的持久死信，用于后续重试或人工处理';
COMMENT ON COLUMN public.telemetry_dead_letters.device_id IS '设备ID';
COMMENT ON COLUMN public.telemetry_dead_letters."key" IS '数据标识符';
COMMENT ON COLUMN public.telemetry_dead_letters.ts IS '原始上报时间';
COMMENT ON COLUMN public.telemetry_dead_letters.raw_payload IS '可重放的原始遥测行';
COMMENT ON COLUMN public.telemetry_dead_letters.status IS 'pending/retrying/processing/dead/resolved';
