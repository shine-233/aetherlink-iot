-- OpenAPI key credential hardening: store only a SHA-256 digest in api_key
-- and keep a short display prefix for list identification.
-- Existing plaintext keys stay readable until rotated; operators should force
-- regeneration so every row ends up holding a digest plus its prefix.
ALTER TABLE public.open_api_keys
    ADD COLUMN IF NOT EXISTS key_prefix varchar(20) NOT NULL DEFAULT '';

COMMENT ON COLUMN public.open_api_keys.key_prefix IS '密钥展示前缀（sk_+8个十六进制字符），用于列表辨认，不含完整密钥';
