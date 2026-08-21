-- OpenAPI key credential hardening: store only a SHA-256 digest in api_key
-- and keep a short display prefix for list identification.
-- CUTOVER SEMANTICS: verification hashes the presented key before lookup, so
-- rows still holding legacy plaintext STOP AUTHENTICATING after this migration
-- takes effect. Operators must regenerate every key as part of rollout; there
-- is intentionally no dual-read transition window.
ALTER TABLE public.open_api_keys
    ADD COLUMN IF NOT EXISTS key_prefix varchar(20) NOT NULL DEFAULT '';

COMMENT ON COLUMN public.open_api_keys.key_prefix IS '密钥展示前缀（sk_+8个十六进制字符），用于列表辨认，不含完整密钥';
