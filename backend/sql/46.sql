-- Bind an optional payload schema to each device configuration.
-- The service layer enforces tenant equality before writing the reference.
ALTER TABLE public.device_configs
  ADD COLUMN IF NOT EXISTS payload_schema_id varchar(36) NULL;

CREATE INDEX IF NOT EXISTS idx_device_configs_payload_schema_id
  ON public.device_configs (payload_schema_id)
  WHERE payload_schema_id IS NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'fk_device_configs_payload_schema'
      AND conrelid = 'public.device_configs'::regclass
  ) THEN
    ALTER TABLE public.device_configs
      ADD CONSTRAINT fk_device_configs_payload_schema
      FOREIGN KEY (payload_schema_id)
      REFERENCES public.payload_schemas (id)
      ON DELETE SET NULL;
  END IF;
END
$$;
