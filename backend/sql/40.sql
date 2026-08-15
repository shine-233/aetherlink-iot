ALTER TABLE public.alarm_config
    ADD COLUMN IF NOT EXISTS trigger_duration int4 NULL DEFAULT 0;

-- Rows created before this version fired on the first matching sample. Normalising
-- them to 0 keeps that immediate-trigger behaviour explicit instead of relying on
-- NULL being interpreted as "no delay" by every future reader.
UPDATE public.alarm_config
SET trigger_duration = 0
WHERE trigger_duration IS NULL;

ALTER TABLE public.alarm_config
    DROP CONSTRAINT IF EXISTS alarm_config_trigger_duration_check,
    ADD CONSTRAINT alarm_config_trigger_duration_check CHECK (
        trigger_duration IS NULL
        OR (trigger_duration >= 0 AND trigger_duration <= 86400)
    );

COMMENT ON COLUMN public.alarm_config.trigger_duration IS
    'Seconds the alarm condition must hold continuously before the alarm fires; 0 or NULL fires on the first matching sample.';
