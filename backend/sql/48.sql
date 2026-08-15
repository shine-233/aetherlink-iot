-- Persist native dashboard publication and public sharing state.
-- Existing boards remain private and receive no share token.
ALTER TABLE public.boards
    ADD COLUMN IF NOT EXISTS published boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS published_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS share_token varchar(64) NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_boards_share_token
    ON public.boards (share_token)
    WHERE share_token IS NOT NULL;

COMMENT ON COLUMN public.boards.published IS '是否允许通过公开链接查看';
COMMENT ON COLUMN public.boards.published_at IS '公开发布时间';
COMMENT ON COLUMN public.boards.share_token IS '公开查看链接令牌';
